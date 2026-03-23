#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
}

require_cmd bash
require_cmd mktemp
require_cmd jq
require_env PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
browser_env_file="$(mktemp)"
bootstrap_json_file="$(mktemp)"
run_id="$(date +%Y%m%d%H%M%S)-$$"
workspace_id="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_ID:-tenant_bconn_${run_id}}"
workspace_name="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_NAME:-Billing Connection Lifecycle ${run_id}}"
primary_connection_name="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_PRIMARY_NAME:-Billing Connection Primary ${run_id}}"
secondary_connection_name="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_SECONDARY_NAME:-Billing Connection Secondary ${run_id}}"
rotated_secret_key="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_ROTATED_STRIPE_SECRET_KEY:-${PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY}}"
lago_organization_id="${PLAYWRIGHT_LIVE_BILLING_CONNECTION_LAGO_ORGANIZATION_ID:-}"

cleanup() {
  rm -f "$browser_env_file"
  rm -f "$bootstrap_json_file"
}
trap cleanup EXIT

export PLAYWRIGHT_LIVE_BASE_URL="${PLAYWRIGHT_LIVE_BASE_URL:-https://staging.sagarwaidande.org}"
export PLAYWRIGHT_LIVE_API_BASE_URL="${PLAYWRIGHT_LIVE_API_BASE_URL:-https://api-staging.sagarwaidande.org}"
export TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"

(
  cd "$repo_root"
  OUTPUT=shell \
  TARGET_TENANT_ID="$TARGET_TENANT_ID" \
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  bash ./scripts/bootstrap_live_e2e_browser_users_cluster.sh >"$browser_env_file"
)
# shellcheck disable=SC1090
source "$browser_env_file"

if [[ -z "$lago_organization_id" ]]; then
  (
    cd "$repo_root"
    RUN_ID="bconn-${run_id}" \
    bash ./scripts/bootstrap_lago_stripe_staging.sh >"$bootstrap_json_file"
  )
  lago_organization_id="$(jq -r '
    select(.organization != null and .stripe_provider != null and .customers != null)
    | .organization.id // empty
  ' "$bootstrap_json_file" | tail -n 1)"
fi

if [[ -z "$lago_organization_id" ]]; then
  echo "failed to resolve staging lago organization id for billing connection lifecycle journey" >&2
  exit 1
fi

(
  cd "$repo_root/web"
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_PLATFORM_EMAIL="$PLAYWRIGHT_LIVE_PLATFORM_EMAIL" \
  PLAYWRIGHT_LIVE_PLATFORM_PASSWORD="$PLAYWRIGHT_LIVE_PLATFORM_PASSWORD" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_ID="$workspace_id" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_NAME="$workspace_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_PRIMARY_NAME="$primary_connection_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_SECONDARY_NAME="$secondary_connection_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_LAGO_ORGANIZATION_ID="$lago_organization_id" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY="$PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_ROTATED_STRIPE_SECRET_KEY="$rotated_secret_key" \
  npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_PLATFORM_EMAIL="$PLAYWRIGHT_LIVE_PLATFORM_EMAIL" \
  PLAYWRIGHT_LIVE_PLATFORM_PASSWORD="$PLAYWRIGHT_LIVE_PLATFORM_PASSWORD" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_ID="$workspace_id" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_NAME="$workspace_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_PRIMARY_NAME="$primary_connection_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_SECONDARY_NAME="$secondary_connection_name" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_LAGO_ORGANIZATION_ID="$lago_organization_id" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY="$PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY" \
  PLAYWRIGHT_LIVE_BILLING_CONNECTION_ROTATED_STRIPE_SECRET_KEY="$rotated_secret_key" \
  npx -y pnpm@10.30.0 exec playwright test tests/e2e/billing-connection-lifecycle-live.spec.ts --workers=1
)
