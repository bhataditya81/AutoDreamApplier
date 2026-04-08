#!/bin/bash
# EC2 bootstrap — runs once via user_data on first boot.
# AL2023 ARM64 (t4g.nano). SSM agent is pre-installed; we must NOT let
# "dnf update -y" kill the running agent before it has registered.
set -euo pipefail

# ── SSM agent: ensure running BEFORE any dnf operations ──────────────────────
# AL2023 starts amazon-ssm-agent automatically, but guarantee it here so the
# CI deploy pipeline can reach this instance via SSM Run Command as soon as
# the instance is healthy — even while the rest of setup is still running.
systemctl enable amazon-ssm-agent
systemctl start amazon-ssm-agent 2>/dev/null || true
systemctl is-active amazon-ssm-agent && echo "SSM agent running." || echo "WARN: SSM agent not active yet."

# ── System update (exclude SSM agent to avoid killing the running service) ───
# Updating amazon-ssm-agent via dnf stops the service and the new package
# may take several seconds to auto-restart — during which SSM shows Offline.
# We re-install/upgrade it explicitly after the main update below.
dnf update -y --exclude='amazon-ssm-agent*'

# ── Re-upgrade SSM agent cleanly so it restarts gracefully ───────────────────
# This updates the package then immediately restarts the service.
dnf upgrade -y amazon-ssm-agent 2>/dev/null || true
systemctl restart amazon-ssm-agent 2>/dev/null || true

# ── Install Docker ────────────────────────────────────────────────────────────
dnf install -y docker
systemctl enable docker
systemctl start docker

# ── Install Docker Compose plugin ────────────────────────────────────────────
mkdir -p /usr/local/lib/docker/cli-plugins
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64" \
    -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# Add ec2-user to docker group
usermod -aG docker ec2-user

# ── Install AWS CLI (for ECR login, SSM parameter reads) ─────────────────────
dnf install -y aws-cli

# ── Create app directory ──────────────────────────────────────────────────────
mkdir -p /opt/autodream
chmod 755 /opt/autodream

# Create env file placeholder (populated by update-env.sh on startup)
cat > /opt/autodream/.env << 'ENVEOF'
# Populated by /opt/autodream/update-env.sh on startup
ENVEOF
chmod 600 /opt/autodream/.env

# ── Pull env from SSM and write to .env file ─────────────────────────────────
# AWS credentials come from the EC2 IAM instance profile — no static keys.
# Uses IMDSv2 (http_tokens=required) to fetch region from instance metadata.
cat > /opt/autodream/update-env.sh << 'SCRIPTEOF'
#!/bin/bash
set -euo pipefail

# IMDSv2: fetch a short-lived token, then use it to get the region.
IMDS_TOKEN=$(curl -sf --retry 3 --connect-timeout 5 \
  -X PUT "http://169.254.169.254/latest/api/token" \
  -H "X-aws-ec2-metadata-token-ttl-seconds: 300")
REGION=$(curl -sf --retry 3 --connect-timeout 5 \
  -H "X-aws-ec2-metadata-token: ${IMDS_TOKEN}" \
  "http://169.254.169.254/latest/meta-data/placement/region")

ECR_REGISTRY=$(aws ssm get-parameter --name /autodream/ecr_registry --region "$REGION" --query Parameter.Value --output text)
DATABASE_URL=$(aws ssm get-parameter --name /autodream/database_url --region "$REGION" --with-decryption --query Parameter.Value --output text)
REDIS_URL=$(aws ssm get-parameter --name /autodream/redis_url --region "$REGION" --with-decryption --query Parameter.Value --output text)
S3_BUCKET_RESUMES=$(aws ssm get-parameter --name /autodream/s3_bucket_resumes --region "$REGION" --query Parameter.Value --output text)
S3_BUCKET_SCREENSHOTS=$(aws ssm get-parameter --name /autodream/s3_bucket_screenshots --region "$REGION" --query Parameter.Value --output text)
SES_FROM_EMAIL=$(aws ssm get-parameter --name /autodream/ses_from_email --region "$REGION" --query Parameter.Value --output text)
DASHBOARD_URL=$(aws ssm get-parameter --name /autodream/dashboard_url --region "$REGION" --query Parameter.Value --output text)
GEMINI_API_KEY=$(aws ssm get-parameter --name /autodream/gemini_api_key --region "$REGION" --with-decryption --query Parameter.Value --output text)
IMAGE_TAG=$(aws ssm get-parameter --name /autodream/image_tag --region "$REGION" --query Parameter.Value --output text 2>/dev/null || echo "latest")

cat > /opt/autodream/.env << EOF
ECR_REGISTRY=${ECR_REGISTRY}
ECR_PREFIX=autodream
DATABASE_URL=${DATABASE_URL}
REDIS_URL=${REDIS_URL}
AWS_REGION=${REGION}
S3_BUCKET_RESUMES=${S3_BUCKET_RESUMES}
S3_BUCKET_SCREENSHOTS=${S3_BUCKET_SCREENSHOTS}
SES_FROM_EMAIL=${SES_FROM_EMAIL}
DASHBOARD_URL=${DASHBOARD_URL}
GEMINI_API_KEY=${GEMINI_API_KEY}
IMAGE_TAG=${IMAGE_TAG}
APP_ENV=staging
EOF
chmod 600 /opt/autodream/.env
SCRIPTEOF
chmod +x /opt/autodream/update-env.sh

# ── systemd service for autodream ─────────────────────────────────────────────
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
ExecStartPre=/bin/bash -c 'source /opt/autodream/.env && aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_REGISTRY}'
ExecStart=/bin/bash -c 'cd /opt/autodream && docker compose --env-file .env up -d --pull always'
ExecStop=/bin/bash -c 'cd /opt/autodream && docker compose down'
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
SERVICEEOF

systemctl daemon-reload
# Note: autodream.service is NOT enabled on first boot — the deploy pipeline
# writes the compose file and starts containers. Enable after first deploy.

# ── Final SSM agent status check ─────────────────────────────────────────────
systemctl is-active amazon-ssm-agent \
  && echo "EC2 setup complete. SSM agent is active." \
  || { echo "WARN: SSM agent not active at end of setup — attempting restart."; \
       systemctl restart amazon-ssm-agent || true; }
