#!/usr/bin/env bash
# Build a Docker image and push to ECR.
# Reads: SERVICE_NAME, DOCKERFILE, CONTEXT, PLATFORM, CHANGE_FLAG (all from CI vars)
# Writes: build.env (dotenv artifact with ECR_IMAGE_<SERVICE>)
set -euo pipefail

# Honour change-detection flag — skip if service didn't change
FLAG_VAL=$(eval echo "${CHANGE_FLAG:-false}")
if [ "$FLAG_VAL" != "true" ]; then
  echo "${SERVICE_NAME} unchanged — skipping build."
  # Still write a placeholder so downstream dotenv jobs don't fail
  SAFE=$(echo "${SERVICE_NAME}" | tr '-' '_' | tr '[:lower:]' '[:upper:]')
  echo "ECR_IMAGE_${SAFE}=skipped" >> build.env
  exit 0
fi

ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
ECR_REPO="${ECR_REGISTRY}/${ECR_REPO_PREFIX}/${SERVICE_NAME}"
IMAGE_SHA="${ECR_REPO}:${CI_COMMIT_SHA}"
IMAGE_LATEST="${ECR_REPO}:latest"

echo "Building ${SERVICE_NAME} (${PLATFORM:-linux/amd64})"
echo "  Dockerfile : ${DOCKERFILE}"
echo "  Context    : ${CONTEXT:-.}"
echo "  Tag        : ${CI_COMMIT_SHA}"

docker build \
  --platform "${PLATFORM:-linux/amd64}" \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  --cache-from "${IMAGE_LATEST}" \
  -f "${DOCKERFILE}" \
  -t "${IMAGE_SHA}" \
  -t "${IMAGE_LATEST}" \
  "${CONTEXT:-.}"

docker push "${IMAGE_SHA}"
docker push "${IMAGE_LATEST}"

SAFE=$(echo "${SERVICE_NAME}" | tr '-' '_' | tr '[:lower:]' '[:upper:]')
echo "ECR_IMAGE_${SAFE}=${IMAGE_SHA}" >> build.env
echo "Pushed ${IMAGE_SHA}"
