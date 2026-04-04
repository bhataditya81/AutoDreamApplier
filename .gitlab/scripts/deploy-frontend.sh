#!/usr/bin/env bash
# Deploy Next.js frontend to Vercel.
# Reads: BUILD_FRONTEND, VERCEL_TOKEN, VERCEL_ORG_ID, VERCEL_PROJECT_ID,
#        AWS env vars (to fetch API URL from SSM).
set -euo pipefail

if [ "${BUILD_FRONTEND:-false}" != "true" ] && [ "${FORCE_DEPLOY:-false}" != "true" ]; then
  echo "Frontend unchanged — skipping Vercel deploy."
  exit 0
fi

# Fetch the live API Gateway URL from SSM
API_URL=$(aws ssm get-parameter \
  --name /autodream/api_endpoint \
  --query 'Parameter.Value' --output text \
  --region "${AWS_REGION}" 2>/dev/null || echo "")

if [ -z "$API_URL" ] || [ "$API_URL" = "None" ]; then
  echo "WARNING: /autodream/api_endpoint not found in SSM."
  echo "  Infrastructure has not been deployed yet."
  echo "  Run the DEPLOY pipeline first. Skipping Vercel deploy."
  exit 0
fi

echo "Deploying frontend → NEXT_PUBLIC_API_URL=${API_URL}"

npm install -g vercel --quiet

# Upsert the env var in the Vercel project
(vercel env rm NEXT_PUBLIC_API_URL production --yes \
  --token "${VERCEL_TOKEN}" --scope "${VERCEL_ORG_ID}" 2>/dev/null || true)
echo "${API_URL}" | vercel env add NEXT_PUBLIC_API_URL production \
  --token "${VERCEL_TOKEN}" \
  --scope "${VERCEL_ORG_ID}"

# Deploy — force fresh build to avoid stale Vercel build cache
cd frontend
vercel deploy --prod \
  --token "${VERCEL_TOKEN}" \
  --scope "${VERCEL_ORG_ID}" \
  --yes \
  --build-env NEXT_PUBLIC_CACHE_BUST="${GITHUB_SHA:-$(date +%s)}" \
  2>&1 | tee /tmp/vercel-out.txt

DEPLOY_URL=$(grep -Eo 'https://[^ ]+\.vercel\.app' /tmp/vercel-out.txt | head -1 || echo "")
echo "Frontend deployed: ${DEPLOY_URL}"
