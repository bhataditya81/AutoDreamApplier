#!/bin/bash
# =============================================================================
# AutoDreamApplier — EC2 Setup & Deploy Script
# =============================================================================
# Usage:
#   First-time setup:   ./scripts/ec2-setup.sh setup
#   Deploy new images:  ./scripts/ec2-setup.sh deploy
#   Check status:       ./scripts/ec2-setup.sh status
#   View logs:          ./scripts/ec2-setup.sh logs [browser-pool|apply-engine]
#   Restart services:   ./scripts/ec2-setup.sh restart
#   SSH into EC2:       ./scripts/ec2-setup.sh ssh
# =============================================================================
set -euo pipefail

# ── Config (edit these if values change) ─────────────────────────────────────
EC2_HOST="44.216.49.133"
EC2_USER="ec2-user"
# Resolve home dir — on Windows Git Bash $HOME may point to /home/user (WSL-style)
# USERPROFILE always points to C:\Users\<name>; convert to Unix path for ssh
_WIN_HOME="${USERPROFILE:-}"
if [ -n "$_WIN_HOME" ]; then
  _UNIX_HOME=$(cygpath -u "$_WIN_HOME" 2>/dev/null || echo "$_WIN_HOME" | sed 's|\\|/|g' | sed 's|^\([A-Za-z]\):|/\L\1|')
else
  _UNIX_HOME="$HOME"
fi
SSH_KEY="${SSH_KEY_PATH:-${_UNIX_HOME}/.ssh/autodream-ec2.pem}"
ECR_REGISTRY="346992621600.dkr.ecr.us-east-1.amazonaws.com"
AWS_REGION="us-east-1"
IMAGE_TAG="${IMAGE_TAG:-latest}"

APPLY_ENGINE_IMAGE="${ECR_REGISTRY}/autodreamapplier/apply-engine:${IMAGE_TAG}"
BROWSER_POOL_IMAGE="${ECR_REGISTRY}/autodreamapplier/browser-pool:${IMAGE_TAG}"
# ─────────────────────────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; exit 1; }

# ── Helpers ───────────────────────────────────────────────────────────────────
check_prerequisites() {
  command -v aws    >/dev/null 2>&1 || err "aws CLI not found"
  command -v docker >/dev/null 2>&1 || err "docker not found"
  command -v ssh    >/dev/null 2>&1 || err "ssh not found"
  [ -f "$SSH_KEY" ] || err "SSH key not found: $SSH_KEY — set SSH_KEY_PATH env var"
  chmod 400 "$SSH_KEY" 2>/dev/null || true
}

ssh_cmd() {
  ssh -i "$SSH_KEY" \
      -o StrictHostKeyChecking=no \
      -o ConnectTimeout=10 \
      "${EC2_USER}@${EC2_HOST}" "$@"
}

scp_file() {
  scp -i "$SSH_KEY" \
      -o StrictHostKeyChecking=no \
      "$1" "${EC2_USER}@${EC2_HOST}:$2"
}

ecr_login() {
  log "Logging in to ECR..."
  aws ecr get-login-password --region "$AWS_REGION" \
    | docker login --username AWS --password-stdin "$ECR_REGISTRY"
}

# ── Build & Push Images ───────────────────────────────────────────────────────
build_and_push() {
  log "Building apply-engine (linux/arm64)..."
  docker buildx build \
    --platform linux/arm64 \
    --provenance=false \
    --file "${PROJECT_ROOT}/deployments/ec2/apply-engine/Dockerfile" \
    --tag "${APPLY_ENGINE_IMAGE}" \
    --push \
    "${PROJECT_ROOT}"

  log "Building browser-pool (linux/arm64)..."
  docker buildx build \
    --platform linux/arm64 \
    --provenance=false \
    --file "${PROJECT_ROOT}/deployments/ec2/browser-pool/Dockerfile" \
    --tag "${BROWSER_POOL_IMAGE}" \
    --push \
    "${PROJECT_ROOT}"

  log "Images pushed successfully"
}

# ── Upload docker-compose to EC2 ──────────────────────────────────────────────
upload_compose() {
  log "Uploading docker-compose.yml to EC2..."
  ssh_cmd "sudo mkdir -p /opt/autodream && sudo chown ${EC2_USER}:${EC2_USER} /opt/autodream"
  scp_file "${PROJECT_ROOT}/deployments/ec2/docker-compose.yml" "/opt/autodream/docker-compose.yml"
}

# ── Commands ──────────────────────────────────────────────────────────────────
cmd_setup() {
  log "=== First-time EC2 setup ==="
  check_prerequisites

  log "Waiting for EC2 to be reachable..."
  for i in {1..12}; do
    ssh_cmd "echo ok" >/dev/null 2>&1 && break
    warn "Not reachable yet, retrying in 10s... ($i/12)"
    sleep 10
  done

  # Fix ARM64 docker-compose binary if missing
  log "Ensuring Docker Compose is installed on EC2..."
  ssh_cmd "
    if ! docker compose version >/dev/null 2>&1; then
      sudo mkdir -p /usr/local/lib/docker/cli-plugins
      sudo curl -SL https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64 \
        -o /usr/local/lib/docker/cli-plugins/docker-compose
      sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
    fi
    echo 'Docker Compose: '$(docker compose version)
  "

  # Pull env from SSM on EC2
  log "Fetching secrets from SSM Parameter Store..."
  ssh_cmd "
    REGION=\$(curl -s http://169.254.169.254/latest/meta-data/placement/region)
    # Fetch a single SSM parameter; --with-decryption is always passed (harmless for String type)
    get() { aws ssm get-parameter --name \"\$1\" --with-decryption --region \"\$REGION\" --query Parameter.Value --output text 2>/dev/null || echo ''; }

    DB_URL=\$(get /autodream/database_url)
    REDIS_URL=\$(get /autodream/redis_url)
    S3_RES=\$(get /autodream/s3_bucket_resumes)
    S3_SCR=\$(get /autodream/s3_bucket_screenshots)
    SES_EMAIL=\$(get /autodream/ses_from_email)
    DASH_URL=\$(get /autodream/dashboard_url)
    AI_URL=\$(get /autodream/ai_service_url)

    sudo tee /opt/autodream/.env > /dev/null << ENV
ECR_REGISTRY=346992621600.dkr.ecr.us-east-1.amazonaws.com
DATABASE_URL=\${DB_URL}
REDIS_URL=\${REDIS_URL}
AWS_REGION=\${REGION}
S3_BUCKET_RESUMES=\${S3_RES}
S3_BUCKET_SCREENSHOTS=\${S3_SCR}
SES_FROM_EMAIL=\${SES_EMAIL}
DASHBOARD_URL=\${DASH_URL}
AI_SERVICE_URL=\${AI_URL}
IMAGE_TAG=latest
ENV
    sudo chmod 640 /opt/autodream/.env
    sudo chown ec2-user:ec2-user /opt/autodream/.env
    echo 'SSM secrets loaded'
  "

  upload_compose
  ecr_login

  # Build & push images
  log "Building and pushing EC2 images (ARM64)..."
  build_and_push

  # Start services
  log "Starting services on EC2..."
  ssh_cmd "
    ECR_REGISTRY=346992621600.dkr.ecr.us-east-1.amazonaws.com
    aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin \$ECR_REGISTRY
    cd /opt/autodream
    IMAGE_TAG=latest ECR_REGISTRY=\$ECR_REGISTRY docker compose --env-file .env pull
    IMAGE_TAG=latest ECR_REGISTRY=\$ECR_REGISTRY docker compose --env-file .env up -d
    docker ps
  "

  log "=== Setup complete! ==="
  log "Browser pool : http://${EC2_HOST}:9222"
  log "Apply engine : http://${EC2_HOST}:8084"
}

cmd_deploy() {
  log "=== Deploying new images (tag: ${IMAGE_TAG}) ==="
  check_prerequisites
  ecr_login
  build_and_push

  log "Updating IMAGE_TAG in SSM..."
  aws ssm put-parameter \
    --name /autodream/image_tag \
    --value "$IMAGE_TAG" \
    --type String \
    --overwrite \
    --region "$AWS_REGION"

  log "Pulling and restarting on EC2..."
  ssh_cmd "
    ECR_REGISTRY=346992621600.dkr.ecr.us-east-1.amazonaws.com
    aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin \$ECR_REGISTRY
    cd /opt/autodream
    IMAGE_TAG=${IMAGE_TAG} ECR_REGISTRY=\$ECR_REGISTRY docker compose --env-file .env pull
    IMAGE_TAG=${IMAGE_TAG} ECR_REGISTRY=\$ECR_REGISTRY docker compose --env-file .env up -d
    docker system prune -f --volumes=false
    docker ps
  "
  log "=== Deploy complete ==="
}

cmd_status() {
  log "=== EC2 Service Status ==="
  ssh_cmd "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"
}

cmd_logs() {
  SERVICE="${1:-}"
  if [ -n "$SERVICE" ]; then
    ssh_cmd "cd /opt/autodream && docker compose logs --tail=100 -f $SERVICE"
  else
    ssh_cmd "cd /opt/autodream && docker compose logs --tail=50"
  fi
}

cmd_restart() {
  log "Restarting services..."
  ssh_cmd "cd /opt/autodream && docker compose --env-file .env restart"
  cmd_status
}

cmd_ssh() {
  log "SSH into EC2 at ${EC2_HOST}..."
  ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "${EC2_USER}@${EC2_HOST}"
}

# ── Entrypoint ────────────────────────────────────────────────────────────────
COMMAND="${1:-help}"
shift || true

case "$COMMAND" in
  setup)   cmd_setup ;;
  deploy)  cmd_deploy ;;
  status)  cmd_status ;;
  logs)    cmd_logs "${1:-}" ;;
  restart) cmd_restart ;;
  ssh)     cmd_ssh ;;
  *)
    echo "Usage: $0 {setup|deploy|status|logs|restart|ssh}"
    echo ""
    echo "  setup    — first-time: build images, upload compose, start services"
    echo "  deploy   — redeploy with new images (IMAGE_TAG=v1.2 ./ec2-setup.sh deploy)"
    echo "  status   — show running containers"
    echo "  logs     — tail logs (optional: logs browser-pool | logs apply-engine)"
    echo "  restart  — restart all services"
    echo "  ssh      — open SSH shell into EC2"
    ;;
esac
