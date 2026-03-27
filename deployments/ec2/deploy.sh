#!/bin/bash
# Usage: ./deploy.sh <ec2-public-ip> <image-tag>
# Requires SSH key in ~/.ssh/autodream-ec2.pem or SSH_PRIVATE_KEY env var
set -euo pipefail

EC2_HOST="${1:?EC2 public IP required}"
IMAGE_TAG="${2:-latest}"
SSH_KEY="${SSH_KEY_PATH:-~/.ssh/autodream-ec2.pem}"

echo "Deploying tag=$IMAGE_TAG to EC2=$EC2_HOST"

# Update SSM parameter with new image tag
REGION="${AWS_REGION:-us-east-1}"
aws ssm put-parameter \
    --name /autodream/image_tag \
    --value "$IMAGE_TAG" \
    --type String \
    --overwrite \
    --region "$REGION"

# SSH to EC2, pull new images, restart
ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "ec2-user@$EC2_HOST" << SSHEOF
    set -e
    source /opt/autodream/.env
    aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $ECR_REGISTRY
    cd /opt/autodream
    IMAGE_TAG=$IMAGE_TAG docker compose pull
    IMAGE_TAG=$IMAGE_TAG docker compose up -d
    docker system prune -f --volumes=false
    echo "Deploy complete"
SSHEOF

echo "Deployment done!"
