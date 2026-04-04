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
  echo "WARNING: /autodream/api_endpoint not found in SSM. Skipping Vercel deploy."
  exit 0
fi

echo "Deploying frontend — NEXT_PUBLIC_API_URL=${API_URL}"

npm install -g vercel --quiet

# Upsert the env var in the Vercel project (used by future redeployments via dashboard)
(vercel env rm NEXT_PUBLIC_API_URL production --yes \
  --token "${VERCEL_TOKEN}" --scope "${VERCEL_ORG_ID}" 2>/dev/null || true)
echo "${API_URL}" | vercel env add NEXT_PUBLIC_API_URL production \
  --token "${VERCEL_TOKEN}" \
  --scope "${VERCEL_ORG_ID}"

# Deploy with --force (skips Vercel build cache) and inject API URL at build time.
# --build-env ensures NEXT_PUBLIC_API_URL is available during `next build` even
# if the Vercel project env var hasn't propagated yet.
cd frontend
vercel deploy --prod \
  --token "${VERCEL_TOKEN}" \
  --scope "${VERCEL_ORG_ID}" \
  --yes \
  --force \
  --build-env NEXT_PUBLIC_API_URL="${API_URL}" \
  2>&1 | tee /tmp/vercel-out.txt

# Check for errors (vercel CLI exits 0 even on build failure in some versions)
if grep -q "Error:" /tmp/vercel-out.txt; then
  echo "ERROR: Vercel build failed:"
  grep "Error:" /tmp/vercel-out.txt
  exit 1
fi

DEPLOY_URL=$(grep -Eo 'https://[a-z0-9-]+\.vercel\.app' /tmp/vercel-out.txt | tail -1 || echo "")
echo "Frontend deployed: ${DEPLOY_URL:-autodreamapplier.vercel.app}"
