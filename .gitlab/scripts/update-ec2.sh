#!/usr/bin/env bash
# SSH to EC2 and hot-swap changed services without downtime.
# Uses AWS SSM Run Command (no SSH key or port-22 SG rule needed).
# Reads change flags from dotenv artifact + AWS env vars.
set -euxo pipefail

ANY_EC2=false
[ "${BUILD_APPLY_ENGINE:-false}"  = "true" ] && ANY_EC2=true
[ "${BUILD_BROWSER_POOL:-false}"  = "true" ] && ANY_EC2=true
[ "${BUILD_AI_SERVICE:-false}"    = "true" ] && ANY_EC2=true

if [ "$ANY_EC2" != "true" ]; then
  echo "No EC2 services changed — skipping."
  exit 0
fi

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
IMAGE_TAG="${CI_COMMIT_SHA:-${GITHUB_SHA}}"
ECR_PREFIX="${ECR_REPO_PREFIX:-autodream}"
S3_DEPLOY_PREFIX="autodream-resumes-prod/.deploy"

BUILD_APPLY="${BUILD_APPLY_ENGINE:-false}"
BUILD_BROWSER="${BUILD_BROWSER_POOL:-false}"
BUILD_AI="${BUILD_AI_SERVICE:-false}"

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

# ── Upload compose file to S3 ─────────────────────────────────────────────────
echo "Uploading docker-compose.yml to S3..."
aws s3 cp deployments/ec2/docker-compose.yml \
  "s3://${S3_DEPLOY_PREFIX}/docker-compose.yml" \
  --region "${AWS_REGION}"

# ── Write remote update script ────────────────────────────────────────────────
cat > /tmp/ec2-remote-update.sh << SCRIPT
#!/usr/bin/env bash
set -euo pipefail
cd /opt/autodream

ECR_REGISTRY="${ECR_REGISTRY}"
ECR_PREFIX="${ECR_PREFIX}"
IMAGE_TAG="${IMAGE_TAG}"
BUILD_APPLY="${BUILD_APPLY}"
BUILD_BROWSER="${BUILD_BROWSER}"
BUILD_AI="${BUILD_AI}"

# Fetch latest compose file from S3
aws s3 cp "s3://${S3_DEPLOY_PREFIX}/docker-compose.yml" \
  /opt/autodream/docker-compose.yml --region "${AWS_REGION}"

# ECR login (uses EC2 IAM instance profile)
aws ecr get-login-password --region "${AWS_REGION}" | \
  docker login --username AWS --password-stdin "\${ECR_REGISTRY}"

if [ "\${BUILD_APPLY}" = "true" ]; then
  docker pull "\${ECR_REGISTRY}/\${ECR_PREFIX}/apply-engine:\${IMAGE_TAG}"
  ECR_REGISTRY="\${ECR_REGISTRY}" ECR_PREFIX="\${ECR_PREFIX}" IMAGE_TAG="\${IMAGE_TAG}" \
    docker compose up -d --no-deps apply-engine
  echo "apply-engine restarted"
fi

if [ "\${BUILD_BROWSER}" = "true" ]; then
  docker pull "\${ECR_REGISTRY}/\${ECR_PREFIX}/browser-pool:\${IMAGE_TAG}"
  ECR_REGISTRY="\${ECR_REGISTRY}" ECR_PREFIX="\${ECR_PREFIX}" IMAGE_TAG="\${IMAGE_TAG}" \
    docker compose up -d --no-deps browser-pool
  echo "browser-pool restarted"
fi

if [ "\${BUILD_AI}" = "true" ]; then
  docker pull "\${ECR_REGISTRY}/\${ECR_PREFIX}/ai-service:\${IMAGE_TAG}"
  ECR_REGISTRY="\${ECR_REGISTRY}" ECR_PREFIX="\${ECR_PREFIX}" IMAGE_TAG="\${IMAGE_TAG}" \
    docker compose up -d --no-deps ai-service
  echo "ai-service restarted"
fi

aws ssm put-parameter \
  --name /autodream/image_tag \
  --value "\${IMAGE_TAG}" \
  --type String --overwrite --region "${AWS_REGION}"

docker system prune -f --volumes=false
echo "EC2 update complete."
SCRIPT

aws s3 cp /tmp/ec2-remote-update.sh \
  "s3://${S3_DEPLOY_PREFIX}/update-remote.sh" \
  --region "${AWS_REGION}"

# ── Wait for SSM agent ────────────────────────────────────────────────────────
echo "Waiting for SSM agent..."
for i in $(seq 1 20); do
  SSM_STATUS=$(aws ssm describe-instance-information \
    --filters "Key=InstanceIds,Values=${INSTANCE_ID}" \
    --query 'InstanceInformationList[0].PingStatus' \
    --output text --region "${AWS_REGION}" 2>/dev/null || echo "Unknown")
  [ "${SSM_STATUS}" = "Online" ] && echo "SSM agent online." && break
  echo "  SSM status: ${SSM_STATUS} (${i}/20) — waiting 15s..."
  sleep 15
  [ "${i}" = "20" ] && echo "ERROR: SSM agent not online." && exit 1
done

# ── Send update command via SSM ───────────────────────────────────────────────
echo "Deploying to EC2 via SSM (tag=${IMAGE_TAG})"
COMMAND_ID=$(aws ssm send-command \
  --instance-ids "${INSTANCE_ID}" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"aws s3 cp s3://${S3_DEPLOY_PREFIX}/update-remote.sh /tmp/ec2-remote-update.sh --region ${AWS_REGION} && bash /tmp/ec2-remote-update.sh\"]" \
  --timeout-seconds 300 \
  --region "${AWS_REGION}" \
  --query 'Command.CommandId' --output text)

echo "SSM Command ID: ${COMMAND_ID}"

# ── Poll until complete ───────────────────────────────────────────────────────
for i in $(seq 1 30); do
  sleep 10
  CMD_STATUS=$(aws ssm get-command-invocation \
    --command-id "${COMMAND_ID}" \
    --instance-id "${INSTANCE_ID}" \
    --query 'Status' --output text --region "${AWS_REGION}" 2>/dev/null || echo "Pending")
  echo "  [${i}/30] Status: ${CMD_STATUS}"

  if [ "${CMD_STATUS}" = "Success" ]; then
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardOutputContent' --output text --region "${AWS_REGION}"
    echo "EC2 update complete."
    exit 0
  fi

  if [ "${CMD_STATUS}" = "Failed" ] || [ "${CMD_STATUS}" = "TimedOut" ] || [ "${CMD_STATUS}" = "Cancelled" ]; then
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardOutputContent' --output text --region "${AWS_REGION}"
    aws ssm get-command-invocation \
      --command-id "${COMMAND_ID}" --instance-id "${INSTANCE_ID}" \
      --query 'StandardErrorContent' --output text --region "${AWS_REGION}"
    echo "ERROR: EC2 update command ${CMD_STATUS}."
    exit 1
  fi
done

echo "ERROR: Timed out waiting for update command."
exit 1
