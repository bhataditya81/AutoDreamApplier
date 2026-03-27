#!/usr/bin/env bash
# =============================================================================
# AutoDreamApplier — Deploy Script
# Usage: ./infra/scripts/deploy.sh <service> <image_tag>
#
# Examples:
#   ./infra/scripts/deploy.sh api-gateway v1.2.3
#   ./infra/scripts/deploy.sh ai-service v1.2.3
#   ./infra/scripts/deploy.sh apply-engine latest
#   ./infra/scripts/deploy.sh all v1.2.3
#
# Supported services:
#   api-gateway, ai-service, apply-engine, job-discovery, job-matcher, browser-pool, all
#
# What it does:
#   1. Builds the Docker image locally
#   2. Pushes to ECR
#   3. For ECS services: triggers a new deployment (rolling update)
#   4. For api-gateway (EC2): SSM Run Command to pull and restart Docker Compose
# =============================================================================

set -euo pipefail

# ── Config ────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TF_DIR="$PROJECT_ROOT/infra/terraform"

# Read AWS region and app name from Terraform outputs (requires prior apply)
AWS_REGION="${AWS_REGION:-$(terraform -chdir="$TF_DIR" output -raw -no-color 2>/dev/null <<< aws_region || echo "us-east-1")}"
AWS_REGION="${AWS_REGION:-us-east-1}"
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
ECR_BASE="$ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"
APP_NAME="autodreamapplier"
ENVIRONMENT="${ENVIRONMENT:-production}"
ECS_CLUSTER="${APP_NAME}-${ENVIRONMENT}-cluster"

# ── Helpers ───────────────────────────────────────────────────────────────────
log()  { echo "[$(date +%H:%M:%S)] $*"; }
err()  { echo "[$(date +%H:%M:%S)] ERROR: $*" >&2; exit 1; }
step() { echo; echo "══════════════════════════════════════════════════════"; echo "  $*"; echo "══════════════════════════════════════════════════════"; }

ecr_login() {
    log "Authenticating with ECR ($ECR_BASE)..."
    aws ecr get-login-password --region "$AWS_REGION" \
        | docker login --username AWS --password-stdin "$ECR_BASE"
}

get_ecr_url() {
    local service="$1"
    terraform -chdir="$TF_DIR" output -json ecr_urls 2>/dev/null \
        | jq -r ".\"$service\"" 2>/dev/null \
        || echo "$ECR_BASE/$APP_NAME-$ENVIRONMENT/$service"
}

get_ec2_instance_id() {
    terraform -chdir="$TF_DIR" output -raw ec2_instance_id 2>/dev/null \
        || aws ec2 describe-instances \
            --filters "Name=tag:Name,Values=$APP_NAME-$ENVIRONMENT-core" \
                      "Name=instance-state-name,Values=running" \
            --query "Reservations[0].Instances[0].InstanceId" \
            --output text \
            --region "$AWS_REGION"
}

# ── Build and push ────────────────────────────────────────────────────────────
build_and_push() {
    local service="$1"
    local tag="$2"
    local ecr_url
    ecr_url="$(get_ecr_url "$service")"
    local full_image="$ecr_url:$tag"
    local latest_image="$ecr_url:latest"

    step "Building $service → $full_image"

    case "$service" in
        api-gateway)
            docker build \
                -f "$PROJECT_ROOT/deploy/docker/api-gateway.Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT"
            ;;
        ai-service)
            docker build \
                -f "$PROJECT_ROOT/ai-service/Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT/ai-service"
            ;;
        apply-engine)
            docker build \
                -f "$PROJECT_ROOT/deploy/docker/apply-engine.Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT"
            ;;
        job-discovery)
            docker build \
                -f "$PROJECT_ROOT/deploy/docker/job-discovery.Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT"
            ;;
        job-matcher)
            docker build \
                -f "$PROJECT_ROOT/deploy/docker/job-matcher.Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT"
            ;;
        browser-pool)
            docker build \
                -f "$PROJECT_ROOT/deploy/docker/browser-node.Dockerfile" \
                -t "$full_image" \
                -t "$latest_image" \
                "$PROJECT_ROOT"
            ;;
        *)
            err "Unknown service: $service"
            ;;
    esac

    log "Pushing $full_image..."
    docker push "$full_image"
    docker push "$latest_image"
    log "Pushed: $full_image"
}

# ── Deploy ────────────────────────────────────────────────────────────────────
deploy_ecs() {
    local service="$1"
    local ecs_service_name="$APP_NAME-$ENVIRONMENT-$service"

    step "Deploying ECS service: $ecs_service_name"

    aws ecs update-service \
        --cluster "$ECS_CLUSTER" \
        --service "$ecs_service_name" \
        --force-new-deployment \
        --region "$AWS_REGION" \
        --output text \
        --query "service.serviceName" > /dev/null

    log "Waiting for $ecs_service_name to stabilise..."
    aws ecs wait services-stable \
        --cluster "$ECS_CLUSTER" \
        --services "$ecs_service_name" \
        --region "$AWS_REGION"

    log "$ecs_service_name is stable."
}

deploy_api_gateway_ec2() {
    local tag="$1"
    local instance_id
    instance_id="$(get_ec2_instance_id)"

    if [ -z "$instance_id" ] || [ "$instance_id" = "None" ]; then
        err "Could not determine core EC2 instance ID. Is it running?"
    fi

    step "Deploying api-gateway on EC2 instance $instance_id"

    local ecr_url
    ecr_url="$(get_ecr_url api-gateway)"

    # Use SSM Run Command — no SSH needed
    local command_id
    command_id="$(aws ssm send-command \
        --instance-ids "$instance_id" \
        --document-name "AWS-RunShellScript" \
        --region "$AWS_REGION" \
        --parameters "commands=[
            'aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ECR_BASE',
            'docker pull $ecr_url:$tag',
            'sed -i \"s|image: .*api-gateway.*|image: $ecr_url:$tag|g\" /opt/autodream/docker-compose.core.yml',
            'docker-compose -f /opt/autodream/docker-compose.core.yml up -d api-gateway',
            'echo Deploy complete'
        ]" \
        --query "Command.CommandId" \
        --output text)"

    log "SSM command sent: $command_id"
    log "Waiting for command to complete..."

    aws ssm wait command-executed \
        --command-id "$command_id" \
        --instance-id "$instance_id" \
        --region "$AWS_REGION" || true

    local status
    status="$(aws ssm get-command-invocation \
        --command-id "$command_id" \
        --instance-id "$instance_id" \
        --region "$AWS_REGION" \
        --query "Status" \
        --output text)"

    if [ "$status" = "Success" ]; then
        log "api-gateway deployed successfully on EC2."
    else
        err "SSM command failed with status: $status. Check SSM console for details."
    fi
}

# ── Main ──────────────────────────────────────────────────────────────────────
SERVICE="${1:-}"
TAG="${2:-latest}"

if [ -z "$SERVICE" ]; then
    echo "Usage: $0 <service> [image_tag]"
    echo ""
    echo "Services: api-gateway ai-service apply-engine job-discovery job-matcher browser-pool all"
    echo "Example:  $0 api-gateway v1.2.3"
    exit 1
fi

ecr_login

deploy_service() {
    local svc="$1"
    build_and_push "$svc" "$TAG"
    case "$svc" in
        api-gateway)
            deploy_api_gateway_ec2 "$TAG"
            ;;
        ai-service|apply-engine|browser-pool)
            deploy_ecs "$svc"
            ;;
        job-discovery|job-matcher)
            # Scheduled tasks — just push the image. The next EventBridge trigger
            # will use the new task definition revision automatically.
            log "$svc image pushed. The next scheduled run will use the new image."
            log "To trigger immediately: aws ecs run-task --cluster $ECS_CLUSTER --task-definition $APP_NAME-$ENVIRONMENT-$svc --launch-type FARGATE ..."
            ;;
    esac
}

if [ "$SERVICE" = "all" ]; then
    for svc in api-gateway ai-service apply-engine job-discovery job-matcher browser-pool; do
        deploy_service "$svc"
    done
else
    deploy_service "$SERVICE"
fi

step "Deploy complete!"
log "ALB DNS: $(terraform -chdir="$TF_DIR" output -raw alb_dns_name 2>/dev/null || echo 'run terraform output alb_dns_name')"
