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
require_env LAGO_API_KEY

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
key_env_file="$(mktemp)"
cleanup() {
  rm -f "$key_env_file"
}
trap cleanup EXIT

export ALPHA_API_BASE_URL="${ALPHA_API_BASE_URL:-https://api-staging.sagarwaidande.org}"
export LAGO_API_URL="${LAGO_API_URL:-https://lago-api-staging.sagarwaidande.org}"
export TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"

PLATFORM_KEY_NAME="${PLATFORM_KEY_NAME:-staging-dunning-journey-platform-admin}"
WRITER_KEY_NAME="${WRITER_KEY_NAME:-staging-dunning-journey-writer}"
READER_KEY_NAME="${READER_KEY_NAME:-staging-dunning-journey-reader}"

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
  PLATFORM_ADMIN_API_KEY="$PLAYWRIGHT_LIVE_PLATFORM_API_KEY" \
  ALPHA_WRITER_API_KEY="$PLAYWRIGHT_LIVE_WRITER_API_KEY" \
  ALPHA_READER_API_KEY="$PLAYWRIGHT_LIVE_READER_API_KEY" \
  ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
  LAGO_API_URL="$LAGO_API_URL" \
  LAGO_API_KEY="$LAGO_API_KEY" \
  TARGET_TENANT_ID="$TARGET_TENANT_ID" \
  bash ./scripts/verify_staging_dunning_journey.sh
)
