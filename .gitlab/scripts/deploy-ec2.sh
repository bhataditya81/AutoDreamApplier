#!/usr/bin/env bash
# Full EC2 deploy — pull ALL service images and restart stack.
# Used by the DEPLOY pipeline (not the update pipeline).
set -euo pipefail

mkdir -p ~/.ssh && chmod 700 ~/.ssh
aws secretsmanager get-secret-value \
  --secret-id "/autodream/ec2_ssh_private_key" \
  --query 'SecretString' --output text \
  --region "${AWS_REGION}" > ~/.ssh/autodream.pem
chmod 600 ~/.ssh/autodream.pem

EC2_IP="${EC2_PUBLIC_IP:-}"
if [ -z "$EC2_IP" ]; then
  EC2_IP=$(aws ssm get-parameter \
    --name /autodream/ec2_public_ip \
    --query 'Parameter.Value' --output text \
    --region "${AWS_REGION}")
fi

if [ -z "$EC2_IP" ] || [ "$EC2_IP" = "None" ]; then
  echo "ERROR: EC2 IP not available."
  exit 1
fi

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
IMAGE_TAG="${CI_COMMIT_SHA}"

echo "Full EC2 deploy to ${EC2_IP} (tag=${IMAGE_TAG})"
ssh-keyscan -H "${EC2_IP}" >> ~/.ssh/known_hosts 2>/dev/null

ssh -i ~/.ssh/autodream.pem \
    -o StrictHostKeyChecking=no \
    -o ConnectTimeout=30 \
    "ec2-user@${EC2_IP}" \
    ECR_REGISTRY="$ECR_REGISTRY" \
    ECR_PREFIX="$ECR_REPO_PREFIX" \
    IMAGE_TAG="$IMAGE_TAG" \
    AWS_REGION="$AWS_REGION" \
    bash -s << 'REMOTE'
set -euo pipefail
cd /opt/autodream

aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "${ECR_REGISTRY}"

# Pull all images
for svc in apply-engine browser-pool ai-service; do
  docker pull "${ECR_REGISTRY}/${ECR_PREFIX}/${svc}:${IMAGE_TAG}"
done

# Restart full stack
ECR_REGISTRY="${ECR_REGISTRY}" IMAGE_TAG="${IMAGE_TAG}" \
  docker compose up -d --remove-orphans

aws ssm put-parameter \
  --name /autodream/image_tag \
  --value "${IMAGE_TAG}" \
  --type String --overwrite --region "${AWS_REGION}"

docker system prune -f --volumes=false
echo "EC2 full deploy complete."
REMOTE
