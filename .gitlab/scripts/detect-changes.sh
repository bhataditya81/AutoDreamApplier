#!/usr/bin/env bash
# Outputs changes.env — dotenv file consumed by downstream jobs.
# Each flag is either "true" or "false".
set -euo pipefail

BEFORE="${CI_COMMIT_BEFORE_SHA:-}"
ZERO="0000000000000000000000000000000000000000"
ENVFILE="changes.env"
: > "$ENVFILE"

# Helper — returns 0 if any given path prefix appears in CHANGED
changed_in() {
  for prefix in "$@"; do
    echo "$CHANGED" | grep -qE "^${prefix}" && return 0
  done
  return 1
}

if [ -z "$BEFORE" ] || [ "$BEFORE" = "$ZERO" ]; then
  echo "First push to main — all services marked changed."
  cat >> "$ENVFILE" << 'EOF'
BUILD_API_GATEWAY=true
BUILD_JOB_DISCOVERY=true
BUILD_JOB_MATCHER=true
BUILD_FOLLOWUP_SCHEDULER=true
BUILD_APPLY_ENGINE=true
BUILD_BROWSER_POOL=true
BUILD_AI_SERVICE=true
BUILD_FRONTEND=true
INFRA_CHANGED=false
EOF
else
  git fetch origin "${BEFORE}" 2>/dev/null || true
  CHANGED=$(git diff --name-only "${BEFORE}" "${CI_COMMIT_SHA}" 2>/dev/null || \
            git diff --name-only HEAD~1 HEAD)

  echo "Changed files:"
  echo "$CHANGED"
  echo ""

  # Shared Go source — touching these rebuilds all Go services
  GO_COMMON="internal/ pkg/ go\.mod go\.sum"

  changed_in $GO_COMMON "cmd/api-gateway/" "deployments/lambda/api-gateway/" \
    && echo "BUILD_API_GATEWAY=true" >> "$ENVFILE" \
    || echo "BUILD_API_GATEWAY=false" >> "$ENVFILE"

  changed_in $GO_COMMON "cmd/job-discovery/" "deployments/lambda/job-discovery/" \
    && echo "BUILD_JOB_DISCOVERY=true" >> "$ENVFILE" \
    || echo "BUILD_JOB_DISCOVERY=false" >> "$ENVFILE"

  changed_in $GO_COMMON "cmd/job-matcher/" "deployments/lambda/job-matcher/" \
    && echo "BUILD_JOB_MATCHER=true" >> "$ENVFILE" \
    || echo "BUILD_JOB_MATCHER=false" >> "$ENVFILE"

  changed_in $GO_COMMON "cmd/followup-scheduler/" "deployments/lambda/followup-scheduler/" \
    && echo "BUILD_FOLLOWUP_SCHEDULER=true" >> "$ENVFILE" \
    || echo "BUILD_FOLLOWUP_SCHEDULER=false" >> "$ENVFILE"

  changed_in $GO_COMMON "cmd/apply-engine/" "deployments/ec2/apply-engine/" \
    && echo "BUILD_APPLY_ENGINE=true" >> "$ENVFILE" \
    || echo "BUILD_APPLY_ENGINE=false" >> "$ENVFILE"

  changed_in "deployments/ec2/browser-pool/" \
    && echo "BUILD_BROWSER_POOL=true" >> "$ENVFILE" \
    || echo "BUILD_BROWSER_POOL=false" >> "$ENVFILE"

  changed_in "ai-service/" \
    && echo "BUILD_AI_SERVICE=true" >> "$ENVFILE" \
    || echo "BUILD_AI_SERVICE=false" >> "$ENVFILE"

  changed_in "frontend/" \
    && echo "BUILD_FRONTEND=true" >> "$ENVFILE" \
    || echo "BUILD_FRONTEND=false" >> "$ENVFILE"

  changed_in "infra/terraform/" \
    && echo "INFRA_CHANGED=true" >> "$ENVFILE" \
    || echo "INFRA_CHANGED=false" >> "$ENVFILE"
fi

echo ""
echo "--- Change detection results ---"
cat "$ENVFILE"

if grep -q "INFRA_CHANGED=true" "$ENVFILE"; then
  echo ""
  echo "WARNING: infra/terraform/ was modified."
  echo "  Terraform changes are NOT applied automatically by the update pipeline."
  echo "  Trigger the DEPLOY pipeline (PIPELINE_TYPE=deploy) to apply them."
fi
