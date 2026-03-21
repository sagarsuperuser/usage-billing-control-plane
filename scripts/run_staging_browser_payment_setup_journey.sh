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

PLATFORM_KEY_NAME="${PLATFORM_KEY_NAME:-staging-browser-payment-setup-platform-admin}"
WRITER_KEY_NAME="${WRITER_KEY_NAME:-staging-browser-payment-setup-writer}"
READER_KEY_NAME="${READER_KEY_NAME:-staging-browser-payment-setup-reader}"

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
  PLATFORM_ADMIN_API_KEY="$PLAYWRIGHT_LIVE_PLATFORM_API_KEY" \
  ALPHA_WRITER_API_KEY="$PLAYWRIGHT_LIVE_WRITER_API_KEY" \
  ALPHA_READER_API_KEY="$PLAYWRIGHT_LIVE_READER_API_KEY" \
  ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
  LAGO_API_URL="$LAGO_API_URL" \
  LAGO_API_KEY="$LAGO_API_KEY" \
  TARGET_TENANT_ID="$TARGET_TENANT_ID" \
  OUTPUT_FILE="$fixture_json_file" \
  bash ./scripts/prepare_staging_browser_payment_setup_fixture.sh
)

PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID="$(jq -r '.invoice_id // empty' "$fixture_json_file")"
if [[ -z "$PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID" ]]; then
  echo "failed to derive PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID from fixture output" >&2
  exit 1
fi

(
  cd "$repo_root/web"
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID="$PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID" \
  npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
  PLAYWRIGHT_LIVE_BASE_URL="$PLAYWRIGHT_LIVE_BASE_URL" \
  PLAYWRIGHT_LIVE_API_BASE_URL="$PLAYWRIGHT_LIVE_API_BASE_URL" \
  PLAYWRIGHT_LIVE_WRITER_EMAIL="$PLAYWRIGHT_LIVE_WRITER_EMAIL" \
  PLAYWRIGHT_LIVE_WRITER_PASSWORD="$PLAYWRIGHT_LIVE_WRITER_PASSWORD" \
  PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID="$PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID" \
  npx -y pnpm@10.30.0 exec playwright test tests/e2e/payment-setup-browser-live.spec.ts --workers=1
)
