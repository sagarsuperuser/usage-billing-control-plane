#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${TENANT_ID:?TENANT_ID is required}"

KEY_NAME="${KEY_NAME:-bootstrap-admin-${TENANT_ID}}"
OUTPUT="${OUTPUT:-json}"
EXPIRES_AT="${EXPIRES_AT:-}"
ALLOW_EXISTING_ACTIVE_KEYS="${ALLOW_EXISTING_ACTIVE_KEYS:-0}"

cmd=(go run ./cmd/bootstrap_tenant_admin_key -tenant-id "$TENANT_ID" -name "$KEY_NAME" -output "$OUTPUT")

if [[ "$ALLOW_EXISTING_ACTIVE_KEYS" == "1" ]]; then
  cmd+=( -allow-existing-active-keys )
fi

if [[ -n "$EXPIRES_AT" ]]; then
  cmd+=( -expires-at "$EXPIRES_AT" )
fi

exec "${cmd[@]}"
