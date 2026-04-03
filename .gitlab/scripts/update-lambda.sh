#!/usr/bin/env bash
# Hot-update a single Lambda function with a new container image.
# Reads: SERVICE_NAME, CHANGE_FLAG (CI vars) + AWS env vars
set -euo pipefail

FLAG_VAL=$(eval echo "${CHANGE_FLAG:-false}")
if [ "$FLAG_VAL" != "true" ]; then
  echo "${SERVICE_NAME} unchanged — skipping Lambda update."
  exit 0
fi

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
IMAGE_URI="${ECR_REGISTRY}/${ECR_REPO_PREFIX}/${SERVICE_NAME}:${CI_COMMIT_SHA}"
FUNCTION_NAME="${PROJECT_NAME}-${SERVICE_NAME}"

echo "Updating Lambda ${FUNCTION_NAME}"
echo "  Image: ${IMAGE_URI}"

aws lambda update-function-code \
  --function-name "${FUNCTION_NAME}" \
  --image-uri "${IMAGE_URI}" \
  --region "${AWS_REGION}"

echo "Waiting for ${FUNCTION_NAME} to become active..."
aws lambda wait function-updated \
  --function-name "${FUNCTION_NAME}" \
  --region "${AWS_REGION}"

echo "Lambda ${FUNCTION_NAME} updated successfully."
