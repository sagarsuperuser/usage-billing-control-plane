#!/usr/bin/env bash
set -euo pipefail

CERT_MANAGER_NAMESPACE="${CERT_MANAGER_NAMESPACE:-cert-manager}"
SECRET_NAME="${CLOUDFLARE_SECRET_NAME:-cloudflare-api-token-secret}"
SECRET_KEY="${CLOUDFLARE_SECRET_KEY:-api-token}"
API_TOKEN="${CLOUDFLARE_API_TOKEN:-}"

if [[ -z "${API_TOKEN}" ]]; then
  echo "CLOUDFLARE_API_TOKEN is required"
  echo "example: CLOUDFLARE_API_TOKEN=... make cloudflare-sync-dns-token"
  exit 1
fi

kubectl create namespace "${CERT_MANAGER_NAMESPACE}" >/dev/null 2>&1 || true

kubectl -n "${CERT_MANAGER_NAMESPACE}" create secret generic "${SECRET_NAME}" \
  --from-literal="${SECRET_KEY}=${API_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "synced secret ${SECRET_NAME} in namespace ${CERT_MANAGER_NAMESPACE}"
