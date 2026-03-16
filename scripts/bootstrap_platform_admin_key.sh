#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"

PLATFORM_KEY_NAME="${PLATFORM_KEY_NAME:-${KEY_NAME:-bootstrap-platform-admin}}"
OUTPUT="${OUTPUT:-json}"
EXPIRES_AT="${EXPIRES_AT:-}"
ALLOW_EXISTING_ACTIVE_KEYS="${ALLOW_EXISTING_ACTIVE_KEYS:-0}"

cmd=(go run ./cmd/bootstrap_platform_admin_key -name "$PLATFORM_KEY_NAME" -output "$OUTPUT")

if [[ "$ALLOW_EXISTING_ACTIVE_KEYS" == "1" ]]; then
  cmd+=( -allow-existing-active-keys )
fi

if [[ -n "$EXPIRES_AT" ]]; then
  cmd+=( -expires-at "$EXPIRES_AT" )
fi

exec "${cmd[@]}"
