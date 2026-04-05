#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
key_env_file="$(mktemp)"
browser_env_file="$(mktemp)"
run_id="$(date +%Y%m%d%H%M%S)-$$"
customer_external_id="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EXTERNAL_ID:-cust_onboard_${run_id}}"
customer_display_name="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_DISPLAY_NAME:-Customer Onboarding ${run_id}}"
customer_email="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EMAIL:-customer-onboarding-${run_id}@alpha.test}"
customer_legal_name="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_LEGAL_NAME:-Customer Onboarding ${run_id} LLC}"
provider_code="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_PROVIDER_CODE:-}"
connection_id="${PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_CONNECTION_ID:-}"

cleanup() {
  rm -f "$key_env_file" "$browser_env_file"
}
trap cleanup EXIT

require_cmd bash
require_cmd curl
require_cmd go
require_cmd jq
require_cmd mktemp

export ALPHA_API_BASE_URL="${ALPHA_API_BASE_URL:-https://api-staging.sagarwaidande.org}"
export PLAYWRIGHT_LIVE_BASE_URL="${PLAYWRIGHT_LIVE_BASE_URL:-https://staging.sagarwaidande.org}"
export PLAYWRIGHT_LIVE_API_BASE_URL="${PLAYWRIGHT_LIVE_API_BASE_URL:-$ALPHA_API_BASE_URL}"
export TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"

PLATFORM_KEY_NAME="${PLATFORM_KEY_NAME:-staging-customer-onboarding-platform-admin}"
WRITER_KEY_NAME="${WRITER_KEY_NAME:-staging-customer-onboarding-writer}"
READER_KEY_NAME="${READER_KEY_NAME:-staging-customer-onboarding-reader}"

(
  cd "$repo_root"
  OUTPUT=shell \
  TARGET_TENANT_ID="$TARGET_TENANT_ID" \
  PLATFORM_KEY_NAME="$PLATFORM_KEY_NAME" \
  WRITER_KEY_NAME="$WRITER_KEY_NAME" \
  READER_KEY_NAME="$READER_KEY_NAME" \
  bash ./scripts/mint_live_e2e_keys_cluster.sh >"$key_env_file"
)
# shellcheck disable=SC1090
source "$key_env_file"

if [[ -z "$connection_id" || -z "$provider_code" ]]; then
  connection_json="$(curl -fsS \
    -H "X-API-Key: $PLAYWRIGHT_LIVE_PLATFORM_API_KEY" \
    "$ALPHA_API_BASE_URL/internal/billing-provider-connections?limit=100")"
  selected_row="$(printf '%s' "$connection_json" | jq -r '.items[] | select((.provider_type // "") == "stripe" and (.status // "") == "connected" and (.workspace_ready // false) == true and (.backend_provider_code // "") != "") | [.id, .backend_provider_code] | @tsv' | head -n 1)"
  if [[ -z "$selected_row" ]]; then
    echo "no connected workspace-ready stripe billing provider connection found" >&2
    exit 1
  fi
  if [[ -z "$connection_id" ]]; then
    connection_id="${selected_row%%$'\t'*}"
  fi
  if [[ -z "$provider_code" ]]; then
    provider_code="${selected_row#*$'\t'}"
  fi
fi

(
  cd "$repo_root"
  go run ./cmd/admin ensure-tenant-workspace-billing \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -platform-api-key "$PLAYWRIGHT_LIVE_PLATFORM_API_KEY" \
    -tenant-id "$TARGET_TENANT_ID" \
    -billing-provider-connection-id "$connection_id" >/dev/null
)

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

(
  cd "$repo_root/web"
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EXTERNAL_ID="$customer_external_id" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_DISPLAY_NAME="$customer_display_name" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EMAIL="$customer_email" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_LEGAL_NAME="$customer_legal_name" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_PROVIDER_CODE="$provider_code" \
  npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EXTERNAL_ID="$customer_external_id" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_DISPLAY_NAME="$customer_display_name" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EMAIL="$customer_email" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_LEGAL_NAME="$customer_legal_name" \
  PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_PROVIDER_CODE="$provider_code" \
  npx -y pnpm@10.30.0 exec playwright test tests/e2e/customer-onboarding-live.spec.ts --workers=1
)
