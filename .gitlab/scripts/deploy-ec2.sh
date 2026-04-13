#!/usr/bin/env bash
# Full EC2 deploy — pull ALL service images and restart stack.
# Uses AWS SSM Run Command (no SSH key or port-22 SG rule needed).
set -euxo pipefail

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
# OVERRIDE_TAG is set to "latest" when skip_image_builds=true (no new image
# was built for this SHA). Otherwise use the commit SHA for immutable tagging.
IMAGE_TAG="${OVERRIDE_TAG:-${CI_COMMIT_SHA:-${GITHUB_SHA}}}"
ECR_PREFIX="${ECR_REPO_PREFIX:-autodream}"
S3_DEPLOY_PREFIX="autodream-resumes-prod/.deploy"

# ── Resolve running instance ID ───────────────────────────────────────────────
echo "Resolving EC2 instance..."
INSTANCE_ID=$(aws ec2 describe-instances \
  --filters \
    "Name=tag:Name,Values=autodream-browser-pool" \
    "Name=instance-state-name,Values=running" \
  --query 'Reservations[0].Instances[0].InstanceId' \
  --output text --region "${AWS_REGION}")

if [ -z "${INSTANCE_ID}" ] || [ "${INSTANCE_ID}" = "None" ]; then
  echo "ERROR: No running autodream-browser-pool instance found."
  echo "  Run the DEPLOY pipeline first to provision the EC2 instance."
  exit 1
fi
echo "Instance: ${INSTANCE_ID} | tag=${IMAGE_TAG}"

# ── Upload compose file to S3 (EC2 IAM role has s3:PutObject on this bucket) ─
echo "Uploading docker-compose.yml to S3..."
aws s3 cp deployments/ec2/docker-compose.yml \
  "s3://${S3_DEPLOY_PREFIX}/docker-compose.yml" \
  --region "${AWS_REGION}"

# ── Write remote deploy script (local vars substituted now, loop vars escaped) ─
cat > /tmp/ec2-remote-deploy.sh << SCRIPT
#!/usr/bin/env bash
set -euo pipefail
mkdir -p /opt/autodream
cd /opt/autodream

ECR_REGISTRY="${ECR_REGISTRY}"
ECR_PREFIX="${ECR_PREFIX}"
IMAGE_TAG="${IMAGE_TAG}"

# Wait for Phase 2 setup to finish (Docker + tooling installed in background).
# setup.sh writes /var/lib/autodream-setup-complete when done.
for i in \$(seq 1 60); do
  [ -f /var/lib/autodream-setup-complete ] && docker --version 2>/dev/null && break
  echo "  Waiting for EC2 setup Phase 2 (\${i}/60 — up to 15 min)..."
  sleep 15
done
if ! docker --version 2>/dev/null; then
  echo "ERROR: Docker not available after 15 min. Phase 2 log:"
  cat /var/log/autodream-setup-phase2.log 2>/dev/null || echo "(no log yet)"
  exit 1
fi

# Fetch latest compose file from S3
aws s3 cp "s3://${S3_DEPLOY_PREFIX}/docker-compose.yml" \
  /opt/autodream/docker-compose.yml --region "${AWS_REGION}"

# ECR login (uses EC2 IAM instance profile — no static credentials needed)
aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "\${ECR_REGISTRY}"

# Pull all service images
for svc in apply-engine browser-pool ai-service; do
  echo "Pulling \${svc}:\${IMAGE_TAG}..."
  docker pull "\${ECR_REGISTRY}/\${ECR_PREFIX}/\${svc}:\${IMAGE_TAG}"
done

# Restart full stack
ECR_REGISTRY="\${ECR_REGISTRY}" ECR_PREFIX="\${ECR_PREFIX}" IMAGE_TAG="\${IMAGE_TAG}" \
  docker compose up -d --remove-orphans

# Record deployed tag for update pipeline and autodream.service restarts
aws ssm put-parameter \
  --name /autodream/image_tag \
  --value "\${IMAGE_TAG}" \
  --type String --overwrite --region "${AWS_REGION}"

docker system prune -f --volumes=false
echo "EC2 full deploy complete."
SCRIPT

aws s3 cp /tmp/ec2-remote-deploy.sh \
  "s3://${S3_DEPLOY_PREFIX}/deploy-remote.sh" \
  --region "${AWS_REGION}"

# ── Wait for SSM agent to be online ──────────────────────────────────────────
# Phase 1 of setup.sh starts SSM before any package installs, so it should
# register within 90 seconds of instance boot. Allow 3 min for safety.
echo "Waiting for SSM agent to be online (up to 3 min)..."
for i in $(seq 1 12); do
  SSM_STATUS=$(aws ssm describe-instance-information \
    --filters "Key=InstanceIds,Values=${INSTANCE_ID}" \
    --query 'InstanceInformationList[0].PingStatus' \
    --output text --region "${AWS_REGION}" 2>/dev/null || echo "Unknown")
  [ "${SSM_STATUS}" = "Online" ] && echo "SSM agent online." && break
  echo "  SSM status: ${SSM_STATUS} (${i}/12) — waiting 15s..."
  sleep 15
  if [ "${i}" = "12" ]; then
    echo "ERROR: SSM agent did not come online within 3 minutes."
    echo "  Check: IAM role has AmazonSSMManagedInstanceCore, instance has internet access."
    exit 1
  fi
done

# ── Send deploy command via SSM ───────────────────────────────────────────────
echo "Sending deploy command via SSM Run Command..."
COMMAND_ID=$(aws ssm send-command \
  --instance-ids "${INSTANCE_ID}" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"aws s3 cp s3://${S3_DEPLOY_PREFIX}/deploy-remote.sh /tmp/ec2-remote-deploy.sh --region ${AWS_REGION} && bash /tmp/ec2-remote-deploy.sh\"]" \
  --timeout-seconds 1800 \
  --region "${AWS_REGION}" \
  --query 'Command.CommandId' --output text)

echo "SSM Command ID: ${COMMAND_ID}"

# ── Poll until complete ───────────────────────────────────────────────────────
for i in $(seq 1 120); do
  sleep 15
  CMD_STATUS=$(aws ssm get-command-invocation \
    --command-id "${COMMAND_ID}" \
    --instance-id "${INSTANCE_ID}" \
    --query 'Status' --output text --region "${AWS_REGION}" 2>/dev/null || echo "Pending")
  echo "  [${i}/120] Status: ${CMD_STATUS}"

  if [ "${CMD_STATUS}" = "Success" ]; then
    echo "=== Remote output ==="
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardOutputContent' --output text --region "${AWS_REGION}"
    echo "EC2 deploy complete."
    exit 0
  fi

  if [ "${CMD_STATUS}" = "Failed" ] || [ "${CMD_STATUS}" = "TimedOut" ] || [ "${CMD_STATUS}" = "Cancelled" ]; then
    echo "=== Remote stdout ==="
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardOutputContent' --output text --region "${AWS_REGION}"
    echo "=== Remote stderr ==="
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardErrorContent' --output text --region "${AWS_REGION}"
    echo "ERROR: EC2 deploy command ${CMD_STATUS}."
    exit 1
  fi
done

echo "ERROR: Timed out waiting for deploy command after 30 minutes."
exit 1
