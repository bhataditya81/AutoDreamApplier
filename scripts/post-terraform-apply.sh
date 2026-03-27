#!/bin/bash
# =============================================================================
# post-terraform-apply.sh — Run after `terraform apply` to sync the new EC2 IP
# into every place that hardcodes it (terraform.tfvars, ec2-setup.sh).
#
# Usage:
#   ./scripts/post-terraform-apply.sh                      # auto-detects IP
#   ./scripts/post-terraform-apply.sh 3.14.159.26          # explicit IP
#   SKIP_VERCEL=1 ./scripts/post-terraform-apply.sh        # skip Vercel update
#
# What it does:
#   1. Gets the new EC2 public IP (from terraform output or AWS describe-instances)
#   2. Updates browser_pool_url and ai_service_url in infra/terraform/terraform.tfvars
#   3. Runs terraform apply again so Lambda env vars pick up the new URLs
#   4. Rewrites EC2_HOST in scripts/ec2-setup.sh
#   5. Optionally updates NEXT_PUBLIC_API_URL in Vercel if API Gateway URL changed
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TERRAFORM_DIR="${PROJECT_ROOT}/infra/terraform"
TFVARS="${TERRAFORM_DIR}/terraform.tfvars"

AWS_REGION="${AWS_REGION:-us-east-1}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
info() { echo -e "${CYAN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# Read project_name from terraform.tfvars
PROJECT_NAME=$(grep '^project_name' "$TFVARS" 2>/dev/null | sed 's/.*=\s*"\(.*\)"/\1/' || echo "autodreamApplier")

# ── 1. Determine new EC2 IP ──────────────────────────────────────────────────

get_ip_from_terraform() {
  if ! command -v terraform >/dev/null 2>&1; then return 1; fi
  if [ ! -d "$TERRAFORM_DIR" ]; then return 1; fi
  pushd "$TERRAFORM_DIR" >/dev/null
  IP=$(terraform output -raw ec2_public_ip 2>/dev/null || echo "")
  popd >/dev/null
  [ -n "$IP" ] && echo "$IP" && return 0
  return 1
}

get_ip_from_aws() {
  aws ec2 describe-instances \
    --region "$AWS_REGION" \
    --filters \
      "Name=tag:Project,Values=${PROJECT_NAME}" \
      "Name=instance-state-name,Values=running" \
    --query "Reservations[0].Instances[0].PublicIpAddress" \
    --output text 2>/dev/null | grep -v '^None$' || true
}

if [ -n "${1:-}" ]; then
  NEW_IP="$1"
  info "Using explicitly provided IP: ${NEW_IP}"
else
  log "Detecting new EC2 public IP..."
  NEW_IP=$(get_ip_from_terraform 2>/dev/null || true)
  if [ -z "$NEW_IP" ]; then
    NEW_IP=$(get_ip_from_aws)
  fi
  if [ -z "$NEW_IP" ] || [ "$NEW_IP" = "None" ]; then
    err "Could not detect EC2 IP. Pass it explicitly: $0 <ip>"
  fi
  log "Detected EC2 IP: ${NEW_IP}"
fi

BROWSER_POOL_URL="http://${NEW_IP}:9222"
AI_SERVICE_URL="http://${NEW_IP}:8000"

echo ""
info "New endpoints:"
info "  BROWSER_POOL_URL = ${BROWSER_POOL_URL}"
info "  AI_SERVICE_URL   = ${AI_SERVICE_URL}"
echo ""

# ── 2. Update terraform.tfvars ────────────────────────────────────────────────

log "Patching terraform.tfvars with new IP..."

# Update the comment line, browser_pool_url, and ai_service_url
sed -i.bak \
  -e "s|^# EC2 Elastic IP assigned:.*|# EC2 Elastic IP assigned: ${NEW_IP}|" \
  -e "s|^browser_pool_url = .*|browser_pool_url = \"${BROWSER_POOL_URL}\"|" \
  -e "s|^ai_service_url   = .*|ai_service_url   = \"${AI_SERVICE_URL}\"|" \
  "$TFVARS" && rm -f "${TFVARS}.bak"

log "  ✓ terraform.tfvars patched"

# ── 3. Re-run terraform apply to push new URLs into Lambda env vars ────────────

log "Running terraform apply to update Lambda env vars..."
pushd "$TERRAFORM_DIR" >/dev/null
terraform apply -auto-approve -target=aws_lambda_function.api_gateway \
  -target=aws_lambda_function.job_matcher \
  -target=aws_lambda_function.job_discovery \
  -target=aws_lambda_function.followup_scheduler 2>&1 | grep -E "(Apply|aws_lambda_function|Error|Warning|complete)" || true
popd >/dev/null
log "  ✓ Lambda env vars updated via terraform apply"

# ── 4. Patch EC2_HOST in scripts/ec2-setup.sh ────────────────────────────────

EC2_SETUP="${SCRIPT_DIR}/ec2-setup.sh"
if [ -f "$EC2_SETUP" ]; then
  log "Patching EC2_HOST in ${EC2_SETUP}..."
  sed -i.bak "s/^EC2_HOST=.*/EC2_HOST=\"${NEW_IP}\"/" "$EC2_SETUP" && rm -f "${EC2_SETUP}.bak"
  log "  ✓ scripts/ec2-setup.sh patched (EC2_HOST=${NEW_IP})"
else
  warn "scripts/ec2-setup.sh not found — skipping patch"
fi

# ── 5. Optionally update NEXT_PUBLIC_API_URL in Vercel ───────────────────────

if [ "${SKIP_VERCEL:-0}" != "1" ] && command -v npx >/dev/null 2>&1; then
  API_GW_URL=""
  if command -v terraform >/dev/null 2>&1 && [ -d "$TERRAFORM_DIR" ]; then
    pushd "$TERRAFORM_DIR" >/dev/null
    API_GW_URL=$(terraform output -raw api_endpoint 2>/dev/null || echo "")
    popd >/dev/null
  fi

  if [ -n "$API_GW_URL" ]; then
    log "Updating Vercel NEXT_PUBLIC_API_URL → ${API_GW_URL}"
    API_GW_URL="${API_GW_URL%/}"
    echo "$API_GW_URL" | npx vercel env add NEXT_PUBLIC_API_URL production --force 2>/dev/null \
      && log "  ✓ Vercel env updated" \
      || warn "  Vercel CLI failed — update NEXT_PUBLIC_API_URL manually in Vercel dashboard"
    info "  Triggering Vercel redeploy..."
    npx vercel --prod --yes 2>/dev/null \
      && log "  ✓ Vercel redeployment triggered" \
      || warn "  Vercel redeploy failed — trigger manually"
  else
    warn "Could not read api_endpoint from terraform — skipping Vercel update"
    warn "Set NEXT_PUBLIC_API_URL manually in Vercel: https://vercel.com/dashboard"
  fi
else
  info "Skipping Vercel update (SKIP_VERCEL=1 or npx not found)"
fi

# ── 6. Print next steps ───────────────────────────────────────────────────────

echo ""
log "========================================="
log "  post-terraform-apply complete!"
log "========================================="
echo ""
info "New EC2 IP  : ${NEW_IP}"
info "Browser pool: ${BROWSER_POOL_URL}"
info "AI service  : ${AI_SERVICE_URL}"
echo ""
info "Next steps:"
info "  1. Run EC2 first-time setup (if fresh instance):"
info "     ./scripts/ec2-setup.sh setup"
echo ""
info "  2. Or just restart services (if re-deploy):"
info "     ./scripts/ec2-setup.sh restart"
echo ""
info "  3. Verify Lambda is routing correctly:"
info "     curl \$(cd ${TERRAFORM_DIR} && terraform output -raw api_endpoint)/health"
echo ""
