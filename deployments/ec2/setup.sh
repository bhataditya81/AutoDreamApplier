#!/bin/bash
set -euo pipefail

# Update system
dnf update -y

# Install Docker
dnf install -y docker
systemctl enable docker
systemctl start docker

# Install Docker Compose plugin
mkdir -p /usr/local/lib/docker/cli-plugins
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64" \
    -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# Add ec2-user to docker group
usermod -aG docker ec2-user

# Install AWS CLI (for ECR login)
dnf install -y aws-cli

# Create app directory
mkdir -p /opt/autodream
chmod 755 /opt/autodream

# Create env file placeholder (actual values injected by Terraform via SSM)
cat > /opt/autodream/.env << 'ENVEOF'
# Populated by /opt/autodream/update-env.sh on startup
ENVEOF
chmod 600 /opt/autodream/.env

# Pull env from SSM and write to .env file
cat > /opt/autodream/update-env.sh << 'SCRIPTEOF'
#!/bin/bash
set -euo pipefail
REGION=$(curl -s http://169.254.169.254/latest/meta-data/placement/region)
ECR_REGISTRY=$(aws ssm get-parameter --name /autodream/ecr_registry --region "$REGION" --query Parameter.Value --output text)
DATABASE_URL=$(aws ssm get-parameter --name /autodream/database_url --region "$REGION" --with-decryption --query Parameter.Value --output text)
REDIS_URL=$(aws ssm get-parameter --name /autodream/redis_url --region "$REGION" --with-decryption --query Parameter.Value --output text)
AWS_ACCESS_KEY_ID=$(aws ssm get-parameter --name /autodream/aws_access_key_id --region "$REGION" --with-decryption --query Parameter.Value --output text)
AWS_SECRET_ACCESS_KEY=$(aws ssm get-parameter --name /autodream/aws_secret_access_key --region "$REGION" --with-decryption --query Parameter.Value --output text)
S3_BUCKET_RESUMES=$(aws ssm get-parameter --name /autodream/s3_bucket_resumes --region "$REGION" --query Parameter.Value --output text)
S3_BUCKET_SCREENSHOTS=$(aws ssm get-parameter --name /autodream/s3_bucket_screenshots --region "$REGION" --query Parameter.Value --output text)
SES_FROM_EMAIL=$(aws ssm get-parameter --name /autodream/ses_from_email --region "$REGION" --query Parameter.Value --output text)
DASHBOARD_URL=$(aws ssm get-parameter --name /autodream/dashboard_url --region "$REGION" --query Parameter.Value --output text)
AI_SERVICE_URL=$(aws ssm get-parameter --name /autodream/ai_service_url --region "$REGION" --query Parameter.Value --output text)
IMAGE_TAG=$(aws ssm get-parameter --name /autodream/image_tag --region "$REGION" --query Parameter.Value --output text 2>/dev/null || echo "latest")

cat > /opt/autodream/.env << EOF
ECR_REGISTRY=${ECR_REGISTRY}
DATABASE_URL=${DATABASE_URL}
REDIS_URL=${REDIS_URL}
AWS_REGION=${REGION}
AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
S3_BUCKET_RESUMES=${S3_BUCKET_RESUMES}
S3_BUCKET_SCREENSHOTS=${S3_BUCKET_SCREENSHOTS}
SES_FROM_EMAIL=${SES_FROM_EMAIL}
DASHBOARD_URL=${DASHBOARD_URL}
AI_SERVICE_URL=${AI_SERVICE_URL}
IMAGE_TAG=${IMAGE_TAG}
EOF
chmod 600 /opt/autodream/.env
SCRIPTEOF
chmod +x /opt/autodream/update-env.sh

# Create systemd service for autodream
cat > /etc/systemd/system/autodream.service << 'SERVICEEOF'
[Unit]
Description=AutoDreamApplier Services
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/autodream
ExecStartPre=/opt/autodream/update-env.sh
ExecStartPre=/bin/bash -c 'source /opt/autodream/.env && aws ecr get-login-password --region $(curl -s http://169.254.169.254/latest/meta-data/placement/region) | docker login --username AWS --password-stdin $ECR_REGISTRY'
ExecStart=/bin/bash -c 'cd /opt/autodream && source .env && docker compose --env-file .env up -d --pull always'
ExecStop=/bin/bash -c 'cd /opt/autodream && docker compose down'
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
SERVICEEOF

systemctl daemon-reload
systemctl enable autodream

echo "EC2 setup complete. Autodream service will start after docker-compose.yml is deployed."
