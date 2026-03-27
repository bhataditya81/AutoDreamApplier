#!/usr/bin/env bash
# =============================================================================
# deploy-images.sh — Build ARM64 Docker images and push to ECR
# Run from the project root after Docker Desktop is running.
#
# Usage:
#   bash scripts/deploy-images.sh [lambda|ec2|all]   # default: all
# =============================================================================
set -euo pipefail

REGION="us-east-1"
ACCOUNT="346992621600"
REGISTRY="${ACCOUNT}.dkr.ecr.${REGION}.amazonaws.com"
PROJECT="autodreamapplier"

TARGET="${1:-all}"

# ── Authenticate with ECR ──────────────────────────────────────────────────
echo ">>> Logging into ECR..."
aws ecr get-login-password --region "$REGION" \
  | docker login --username AWS --password-stdin "$REGISTRY"

# ── Enable ARM64 cross-compilation (one-time, Docker Desktop includes QEMU) ─
echo ">>> Enabling ARM64 cross-compilation..."
docker run --privileged --rm tonistiigi/binfmt --install arm64 2>/dev/null || true

# ── Lambda images ──────────────────────────────────────────────────────────
build_lambda() {
  local SERVICE=$1
  local IMAGE="${REGISTRY}/${PROJECT}/${SERVICE}:latest"
  echo ""
  echo ">>> Building Lambda: ${SERVICE}"
  docker buildx build \
    --platform linux/arm64 \
    --file "deployments/lambda/${SERVICE}/Dockerfile" \
    --tag "$IMAGE" \
    --push \
    .
  echo ">>> Pushed: ${IMAGE}"
}

if [[ "$TARGET" == "lambda" || "$TARGET" == "all" ]]; then
  build_lambda "api-gateway"
  build_lambda "job-discovery"
  build_lambda "job-matcher"
  build_lambda "followup-scheduler"
fi

# ── EC2 images ──────────────────────────────────────────────────────────────
build_ec2() {
  local SERVICE=$1
  local IMAGE="${REGISTRY}/${PROJECT}/${SERVICE}:latest"
  echo ""
  echo ">>> Building EC2: ${SERVICE}"
  docker buildx build \
    --platform linux/arm64 \
    --file "deployments/ec2/${SERVICE}/Dockerfile" \
    --tag "$IMAGE" \
    --push \
    .
  echo ">>> Pushed: ${IMAGE}"
}

if [[ "$TARGET" == "ec2" || "$TARGET" == "all" ]]; then
  build_ec2 "apply-engine"
  build_ec2 "browser-pool"

  # ai-service uses its own directory as build context (ai-service/ is in .dockerignore)
  AI_IMAGE="${REGISTRY}/${PROJECT}/ai-service:latest"
  echo ""
  echo ">>> Building EC2: ai-service"
  docker buildx build \
    --platform linux/arm64 \
    --file "deployments/ec2/ai-service/Dockerfile" \
    --tag "$AI_IMAGE" \
    --push \
    ai-service/
  echo ">>> Pushed: ${AI_IMAGE}"
fi

echo ""
echo "=== All images pushed! ==="
echo ""
echo "Next: run terraform apply to create Lambda functions"
echo "  cd infra/terraform"
echo "  terraform apply -auto-approve"
