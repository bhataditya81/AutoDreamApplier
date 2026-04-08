#!/bin/bash
# EC2 bootstrap — runs once via user_data on first boot.
# AL2023 ARM64 (t4g.nano).
#
# Boot-to-SSM-ready target: < 90 seconds
# Strategy: Phase 1 (SSM + scripts + systemd) runs synchronously.
#           Phase 2 (Docker, docker-compose, aws-cli, security patches)
#           runs in the background so the CI pipeline can reach the instance
#           while setup continues.
set -euo pipefail

# ══════════════════════════════════════════════════════════════════════════════
# PHASE 1 — Synchronous: everything SSM needs to register (~30–60 s)
# ══════════════════════════════════════════════════════════════════════════════

# ── SSM agent ────────────────────────────────────────────────────────────────
# AL2023 starts amazon-ssm-agent by default; guarantee it here.
systemctl enable amazon-ssm-agent
systemctl start amazon-ssm-agent 2>/dev/null || true

# ── App directory + scripts ───────────────────────────────────────────────────
mkdir -p /opt/autodream
chmod 755 /opt/autodream

cat > /opt/autodream/.env << 'ENVEOF'
# Populated by /opt/autodream/update-env.sh on startup
ENVEOF
chmod 600 /opt/autodream/.env

# Uses IMDSv2 (http_tokens=required) to fetch region from instance metadata.
cat > /opt/autodream/update-env.sh << 'SCRIPTEOF'
#!/bin/bash
set -euo pipefail

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

# ── systemd service for autodream ────────────────────────────────────────────
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
# Note: autodream.service is NOT enabled here — the CI deploy pipeline
# writes docker-compose.yml and enables/starts the service on first deploy.

echo "[Phase 1 complete] SSM agent running; scripts and systemd unit written."

# ══════════════════════════════════════════════════════════════════════════════
# PHASE 2 — Background: Docker + tooling + security patches (~5–10 min)
# CI pipeline can already use SSM while this runs in the background.
# The deploy pipeline checks for Docker readiness before starting containers.
# ══════════════════════════════════════════════════════════════════════════════
(
  set -euo pipefail
  LOG=/var/log/autodream-setup-phase2.log
  exec > >(tee -a "$LOG") 2>&1
  echo "[$(date -u +%H:%M:%SZ)] Phase 2 starting..."

  # Security patches only (fast — no full package upgrade)
  dnf upgrade -y --security 2>/dev/null || true

  # Docker
  dnf install -y docker
  systemctl enable docker
  systemctl start docker

  # Docker Compose plugin (aarch64 for t4g)
  mkdir -p /usr/local/lib/docker/cli-plugins
  curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64" \
      -o /usr/local/lib/docker/cli-plugins/docker-compose
  chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

  # Add ec2-user to docker group
  usermod -aG docker ec2-user

  # AWS CLI
  dnf install -y aws-cli

  # Nightly security update cron (avoids long boot times on future reboots)
  echo "0 3 * * * root dnf upgrade -y --security >> /var/log/dnf-security-cron.log 2>&1" \
    > /etc/cron.d/autodream-security-updates

  # Signal completion so the CI deploy pipeline can proceed
  touch /var/lib/autodream-setup-complete
  echo "[$(date -u +%H:%M:%SZ)] Phase 2 complete. Docker and tooling ready."
) &

disown
echo "[setup.sh] Phase 2 running in background (PID $!). Boot script exiting."
