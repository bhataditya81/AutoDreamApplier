#!/usr/bin/env bash
# SSH to EC2 and hot-swap changed services without downtime.
# Reads change flags from dotenv artifact + AWS env vars.
set -euo pipefail

ANY_EC2=false
[ "${BUILD_APPLY_ENGINE:-false}"  = "true" ] && ANY_EC2=true
[ "${BUILD_BROWSER_POOL:-false}"  = "true" ] && ANY_EC2=true
[ "${BUILD_AI_SERVICE:-false}"    = "true" ] && ANY_EC2=true

if [ "$ANY_EC2" != "true" ]; then
  echo "No EC2 services changed — skipping."
  exit 0
fi

# SSH key from masked CI variable
mkdir -p ~/.ssh && chmod 700 ~/.ssh
printf '%s\n' "${EC2_SSH_PRIVATE_KEY}" > ~/.ssh/autodream.pem
chmod 600 ~/.ssh/autodream.pem

# Fetch EC2 IP from SSM (stored by Terraform)
EC2_IP=$(aws ssm get-parameter \
  --name /autodream/ec2_public_ip \
  --query 'Parameter.Value' --output text \
  --region "${AWS_REGION}")

if [ -z "$EC2_IP" ] || [ "$EC2_IP" = "None" ]; then
  echo "ERROR: /autodream/ec2_public_ip not found in SSM."
  echo "  Run the DEPLOY pipeline first to provision the EC2 instance."
  exit 1
fi

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
IMAGE_TAG="${CI_COMMIT_SHA}"

echo "Deploying to EC2 ${EC2_IP} (tag=${IMAGE_TAG})"
ssh-keyscan -H "${EC2_IP}" >> ~/.ssh/known_hosts 2>/dev/null

# Pass flags as env vars to avoid quoting issues inside the SSH block
BUILD_APPLY="${BUILD_APPLY_ENGINE:-false}"
BUILD_BROWSER="${BUILD_BROWSER_POOL:-false}"
BUILD_AI="${BUILD_AI_SERVICE:-false}"

ssh -i ~/.ssh/autodream.pem \
    -o StrictHostKeyChecking=no \
    -o ConnectTimeout=30 \
    "ec2-user@${EC2_IP}" \
    BUILD_APPLY="$BUILD_APPLY" \
    BUILD_BROWSER="$BUILD_BROWSER" \
    BUILD_AI="$BUILD_AI" \
    ECR_REGISTRY="$ECR_REGISTRY" \
    ECR_PREFIX="$ECR_REPO_PREFIX" \
    IMAGE_TAG="$IMAGE_TAG" \
    AWS_REGION="$AWS_REGION" \
    bash -s << 'REMOTE'
set -euo pipefail
cd /opt/autodream

aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "${ECR_REGISTRY}"

if [ "${BUILD_APPLY}" = "true" ]; then
  docker pull "${ECR_REGISTRY}/${ECR_PREFIX}/apply-engine:${IMAGE_TAG}"
  ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
    docker compose up -d --no-deps apply-engine
  echo "apply-engine restarted"
fi

if [ "${BUILD_BROWSER}" = "true" ]; then
  docker pull "${ECR_REGISTRY}/${ECR_PREFIX}/browser-pool:${IMAGE_TAG}"
  ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
    docker compose up -d --no-deps browser-pool
  echo "browser-pool restarted"
fi

if [ "${BUILD_AI}" = "true" ]; then
  docker pull "${ECR_REGISTRY}/${ECR_PREFIX}/ai-service:${IMAGE_TAG}"
  ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
    docker compose up -d --no-deps ai-service
  echo "ai-service restarted"
fi

# Record the deployed tag in SSM
aws ssm put-parameter \
  --name /autodream/image_tag \
  --value "${IMAGE_TAG}" \
  --type String \
  --overwrite \
  --region "${AWS_REGION}"

docker system prune -f --volumes=false
echo "EC2 update complete."
REMOTE
