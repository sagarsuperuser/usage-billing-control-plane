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
require_cmd curl
require_cmd go
require_cmd jq
require_cmd mktemp
require_env LAGO_API_KEY

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
key_env_file="$(mktemp)"
browser_env_file="$(mktemp)"
fixture_json_file="$(mktemp)"

cleanup() {
  rm -f "$key_env_file" "$browser_env_file" "$fixture_json_file"
}
trap cleanup EXIT

export ALPHA_API_BASE_URL="${ALPHA_API_BASE_URL:-https://api-staging.sagarwaidande.org}"
export LAGO_API_URL="${LAGO_API_URL:-https://lago-api-staging.sagarwaidande.org}"
export PLAYWRIGHT_LIVE_BASE_URL="${PLAYWRIGHT_LIVE_BASE_URL:-https://staging.sagarwaidande.org}"
export PLAYWRIGHT_LIVE_API_BASE_URL="${PLAYWRIGHT_LIVE_API_BASE_URL:-$ALPHA_API_BASE_URL}"
export TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"

PLATFORM_KEY_NAME="${PLATFORM_KEY_NAME:-staging-subchange-platform-admin}"
WRITER_KEY_NAME="${WRITER_KEY_NAME:-staging-subchange-writer}"
READER_KEY_NAME="${READER_KEY_NAME:-staging-subchange-reader}"

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

connection_json="$(curl -fsS \
  -H "X-API-Key: $PLAYWRIGHT_LIVE_PLATFORM_API_KEY" \
  "$ALPHA_API_BASE_URL/internal/billing-provider-connections?limit=100")"
selected_row="$(printf '%s' "$connection_json" | jq -r '.items[] | select((.provider_type // "") == "stripe" and (.status // "") == "connected" and (.workspace_ready // false) == true and (.lago_provider_code // "") != "") | [.id, .lago_provider_code] | @tsv' | head -n 1)"
if [[ -z "$selected_row" ]]; then
  echo "no connected workspace-ready stripe billing provider connection found" >&2
  exit 1
fi
connection_id="${selected_row%%$'\t'*}"
provider_code="${selected_row#*$'\t'}"

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
  cd "$repo_root"
  ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
  ALPHA_WRITER_API_KEY="$PLAYWRIGHT_LIVE_WRITER_API_KEY" \
  ALPHA_READER_API_KEY="$PLAYWRIGHT_LIVE_READER_API_KEY" \
  BILLING_PROVIDER_CODE="$provider_code" \
  LAGO_API_URL="$LAGO_API_URL" \
  LAGO_API_KEY="$LAGO_API_KEY" \
  OUTPUT_FILE="$fixture_json_file" \
  bash ./scripts/prepare_staging_subscription_change_fixture.sh
)

PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID="$(jq -r '.subscription.id // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE="$(jq -r '.subscription.code // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME="$(jq -r '.current_plan.name // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE="$(jq -r '.current_plan.code // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID="$(jq -r '.target_plan.id // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME="$(jq -r '.target_plan.name // empty' "$fixture_json_file")"
PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE="$(jq -r '.target_plan.code // empty' "$fixture_json_file")"

for key in \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE; do
  if [[ -z "${!key:-}" ]]; then
    echo "failed to derive $key from subscription change fixture output" >&2
    exit 1
  fi
done

(
  cd "$repo_root/web"
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE" \
  LAGO_API_URL="$LAGO_API_URL" \
  LAGO_API_KEY="$LAGO_API_KEY" \
  npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME" \
  PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE="$PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE" \
  LAGO_API_URL="$LAGO_API_URL" \
  LAGO_API_KEY="$LAGO_API_KEY" \
  npx -y pnpm@10.30.0 exec playwright test tests/e2e/subscription-change-cancel-live.spec.ts --workers=1
)
