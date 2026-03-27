#!/usr/bin/env bash
# =============================================================================
# AutoDreamApplier — Bootstrap Script
# First-time infrastructure provisioning and setup.
#
# Usage:
#   ./infra/scripts/bootstrap.sh [command]
#
# Commands:
#   init      — terraform init + validate
#   plan      — terraform plan (dry run)
#   apply     — terraform apply (provisions all infrastructure)
#   migrate   — copies migrations to EC2 and runs them via SSM
#   status    — show ECS service health and EC2 status
#   destroy   — terraform destroy (DANGER: destroys all resources)
#   all       — init + apply + migrate (full first-time setup)
#
# Prerequisites:
#   - AWS CLI v2 configured: aws configure
#   - Terraform >= 1.6 installed
#   - A terraform.tfvars file (never commit it) with required secrets
#
# Example terraform.tfvars:
#   db_password         = "change_me_32_chars_minimum!!"
#   jwt_secret          = "change_me_jwt_secret_32_chars_min"
#   anthropic_api_key   = "sk-ant-..."
#   domain_name         = "autodreamapplier.com"
#   alert_email         = "you@example.com"
#   ssh_allowed_cidr    = "YOUR.IP.ADDRESS/32"
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TF_DIR="$PROJECT_ROOT/infra/terraform"
MIGRATIONS_DIR="$PROJECT_ROOT/migrations"

# ── Colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'

log()  { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date +%H:%M:%S)] WARN:${NC} $*"; }
err()  { echo -e "${RED}[$(date +%H:%M:%S)] ERROR:${NC} $*" >&2; exit 1; }
step() { echo; echo -e "${CYAN}══════════════════════════════════════════════════════${NC}"; echo -e "${CYAN}  $*${NC}"; echo -e "${CYAN}══════════════════════════════════════════════════════${NC}"; }

# ── Checks ────────────────────────────────────────────────────────────────────
check_prerequisites() {
    step "Checking prerequisites"
    local missing=0

    command -v terraform > /dev/null 2>&1 || { warn "terraform not found — install from https://terraform.io"; missing=1; }
    command -v aws > /dev/null 2>&1       || { warn "aws CLI not found — install from https://aws.amazon.com/cli/"; missing=1; }
    command -v jq  > /dev/null 2>&1       || { warn "jq not found — install with: brew install jq / apt install jq"; missing=1; }

    [ $missing -eq 0 ] || err "Missing prerequisites. Install them and re-run."

    log "Terraform: $(terraform version -json | jq -r '.terraform_version')"
    log "AWS CLI:   $(aws --version 2>&1 | head -1)"

    # Verify AWS credentials
    local account_id
    account_id="$(aws sts get-caller-identity --query Account --output text 2>/dev/null)" \
        || err "AWS credentials not configured. Run: aws configure"
    log "AWS Account: $account_id"
    log "AWS Region:  ${AWS_DEFAULT_REGION:-$(aws configure get region || echo 'us-east-1')}"
}

# ── Terraform commands ────────────────────────────────────────────────────────
tf_init() {
    step "Terraform: init"
    terraform -chdir="$TF_DIR" init -upgrade
    log "Init complete."
}

tf_validate() {
    step "Terraform: validate"
    terraform -chdir="$TF_DIR" validate
    log "Validation passed."
}

tf_plan() {
    step "Terraform: plan"
    [ -f "$TF_DIR/terraform.tfvars" ] || warn "No terraform.tfvars found — you will be prompted for variables."
    terraform -chdir="$TF_DIR" plan -out="$TF_DIR/tfplan"
    log "Plan saved to $TF_DIR/tfplan"
}

tf_apply() {
    step "Terraform: apply"
    if [ -f "$TF_DIR/tfplan" ]; then
        terraform -chdir="$TF_DIR" apply "$TF_DIR/tfplan"
        rm -f "$TF_DIR/tfplan"
    else
        [ -f "$TF_DIR/terraform.tfvars" ] || warn "No terraform.tfvars found — you will be prompted for variables."
        terraform -chdir="$TF_DIR" apply -auto-approve
    fi
    log "Infrastructure provisioned."
}

# ── Migrations ────────────────────────────────────────────────────────────────
run_migrations() {
    step "Running database migrations on EC2 via SSM"

    local instance_id
    instance_id="$(terraform -chdir="$TF_DIR" output -raw ec2_instance_id 2>/dev/null)" \
        || err "Could not get EC2 instance ID from Terraform outputs. Run terraform apply first."

    local aws_region
    aws_region="$(terraform -chdir="$TF_DIR" output -raw -no-color 2>/dev/null <<< "aws_region" || echo "us-east-1")"
    aws_region="${AWS_DEFAULT_REGION:-us-east-1}"

    local ssm_prefix
    ssm_prefix="$(terraform -chdir="$TF_DIR" output -raw ssm_prefix 2>/dev/null)" \
        || ssm_prefix="/autodreamapplier/production"

    log "Instance: $instance_id"
    log "SSM prefix: $ssm_prefix"

    # Upload migrations to the EC2 instance via S3 then SSM
    local s3_bucket
    s3_bucket="$(terraform -chdir="$TF_DIR" output -raw s3_bucket_name 2>/dev/null)" \
        || err "Could not get S3 bucket name from outputs."

    log "Uploading migrations to s3://$s3_bucket/migrations/"
    aws s3 sync "$MIGRATIONS_DIR/" "s3://$s3_bucket/migrations/" \
        --region "$aws_region" \
        --exclude "*.DS_Store"

    log "Copying migrations to EC2 and running..."
    local command_id
    command_id="$(aws ssm send-command \
        --instance-ids "$instance_id" \
        --document-name "AWS-RunShellScript" \
        --region "$aws_region" \
        --parameters "commands=[
            'aws s3 sync s3://$s3_bucket/migrations/ /opt/autodream/migrations/ --region $aws_region',
            'DB_PASSWORD=\$(aws ssm get-parameter --name \"$ssm_prefix/db_password\" --with-decryption --query \"Parameter.Value\" --output text --region $aws_region)',
            'DB_PASSWORD_SAFE=\$(python3 -c \"import urllib.parse, sys; print(urllib.parse.quote(sys.argv[1]))\" \"\$DB_PASSWORD\")',
            'DATABASE_URL=\"postgres://autodream:\$DB_PASSWORD_SAFE@127.0.0.1:5432/autodreamapplier?sslmode=disable\"',
            'migrate -path /opt/autodream/migrations -database \"\$DATABASE_URL\" up',
            'echo Migration complete'
        ]" \
        --comment "AutoDreamApplier database migration" \
        --query "Command.CommandId" \
        --output text)"

    log "SSM command ID: $command_id"
    log "Waiting for migration to complete (this may take 30-60 seconds)..."

    # Poll until done
    local max_wait=120 elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        local status
        status="$(aws ssm get-command-invocation \
            --command-id "$command_id" \
            --instance-id "$instance_id" \
            --region "$aws_region" \
            --query "Status" \
            --output text 2>/dev/null || echo "Pending")"

        case "$status" in
            Success)
                log "Migrations applied successfully!"
                # Print output
                aws ssm get-command-invocation \
                    --command-id "$command_id" \
                    --instance-id "$instance_id" \
                    --region "$aws_region" \
                    --query "StandardOutputContent" \
                    --output text
                return 0
                ;;
            Failed|Cancelled|TimedOut)
                err "Migration failed with status: $status"
                aws ssm get-command-invocation \
                    --command-id "$command_id" \
                    --instance-id "$instance_id" \
                    --region "$aws_region" \
                    --query "StandardErrorContent" \
                    --output text >&2
                return 1
                ;;
            *)
                echo -n "."
                sleep 5
                elapsed=$((elapsed + 5))
                ;;
        esac
    done

    err "Migration timed out after ${max_wait}s. Check SSM console for command: $command_id"
}

# ── Status ────────────────────────────────────────────────────────────────────
show_status() {
    step "Infrastructure Status"

    local aws_region="${AWS_DEFAULT_REGION:-us-east-1}"

    log "Terraform outputs:"
    terraform -chdir="$TF_DIR" output 2>/dev/null || warn "Run terraform apply first."

    echo ""
    log "ECS services:"
    aws ecs list-services \
        --cluster "autodreamapplier-production-cluster" \
        --region "$aws_region" \
        --output table 2>/dev/null || warn "ECS cluster not found."

    echo ""
    log "EC2 core instance:"
    local instance_id
    instance_id="$(terraform -chdir="$TF_DIR" output -raw ec2_instance_id 2>/dev/null)" || true
    if [ -n "$instance_id" ]; then
        aws ec2 describe-instances \
            --instance-ids "$instance_id" \
            --region "$aws_region" \
            --query "Reservations[0].Instances[0].{State:State.Name,PrivateIP:PrivateIpAddress,Type:InstanceType}" \
            --output table
    fi
}

# ── Destroy ───────────────────────────────────────────────────────────────────
tf_destroy() {
    step "Terraform: DESTROY"
    echo -e "${RED}WARNING: This will destroy ALL infrastructure including the database!${NC}"
    echo -e "${RED}All PostgreSQL data, Redis data, and S3 objects will be permanently deleted.${NC}"
    echo ""
    read -r -p "Type 'yes I want to destroy everything' to confirm: " confirm
    if [ "$confirm" = "yes I want to destroy everything" ]; then
        # S3 buckets have prevent_destroy — must be emptied manually first
        warn "S3 buckets have lifecycle { prevent_destroy = true }."
        warn "Empty them manually if you want to destroy: aws s3 rm s3://BUCKET --recursive"
        terraform -chdir="$TF_DIR" destroy
    else
        log "Destroy cancelled."
    fi
}

# ── Print all URLs ─────────────────────────────────────────────────────────────
print_urls() {
    step "Application URLs"

    local alb_dns
    alb_dns="$(terraform -chdir="$TF_DIR" output -raw alb_dns_name 2>/dev/null)" || alb_dns="(run terraform apply first)"

    echo ""
    echo "  API Gateway:  http://$alb_dns"
    echo "  Health check: http://$alb_dns/health"
    echo "  AI service:   http://$alb_dns/api/v1/health"
    echo ""
    log "Use SSM Session Manager for shell access (no SSH key needed):"
    local instance_id
    instance_id="$(terraform -chdir="$TF_DIR" output -raw ec2_instance_id 2>/dev/null)" || instance_id="INSTANCE_ID"
    echo "  aws ssm start-session --target $instance_id"
}

# ── Main ──────────────────────────────────────────────────────────────────────
COMMAND="${1:-help}"

case "$COMMAND" in
    init)
        check_prerequisites
        tf_init
        tf_validate
        ;;
    plan)
        check_prerequisites
        tf_init
        tf_plan
        ;;
    apply)
        check_prerequisites
        tf_init
        tf_apply
        print_urls
        ;;
    migrate)
        run_migrations
        ;;
    status)
        show_status
        ;;
    destroy)
        tf_destroy
        ;;
    all)
        check_prerequisites
        tf_init
        tf_validate
        tf_plan
        tf_apply
        run_migrations
        print_urls
        log "Bootstrap complete! Your AutoDreamApplier infrastructure is live."
        ;;
    help|--help|-h)
        echo "AutoDreamApplier Bootstrap"
        echo ""
        echo "Usage: $0 <command>"
        echo ""
        echo "Commands:"
        echo "  init      terraform init + validate"
        echo "  plan      terraform plan (dry run, no changes)"
        echo "  apply     terraform apply (provisions infrastructure)"
        echo "  migrate   copy migrations to EC2 and run them"
        echo "  status    show ECS and EC2 status"
        echo "  destroy   destroy all infrastructure (DANGEROUS)"
        echo "  all       full first-time setup: init + apply + migrate"
        echo ""
        echo "First-time setup:"
        echo "  1. Create infra/terraform/terraform.tfvars with your secrets"
        echo "  2. Run: ./infra/scripts/bootstrap.sh all"
        echo "  3. Run: ./infra/scripts/deploy.sh all v1.0.0"
        ;;
    *)
        err "Unknown command: $COMMAND. Run '$0 help' for usage."
        ;;
esac
