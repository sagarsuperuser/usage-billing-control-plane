#!/usr/bin/env bash
set -euo pipefail

ISSUER_FILE="${ISSUER_FILE:-}"

if [[ -z "${ISSUER_FILE}" ]]; then
  echo "ISSUER_FILE is required"
  echo "example: ISSUER_FILE=deploy/cert-manager/cluster-issuer-letsencrypt-staging.yaml make cert-manager-apply-issuer"
  exit 1
fi

if [[ ! -f "${ISSUER_FILE}" ]]; then
  echo "issuer file not found: ${ISSUER_FILE}"
  exit 1
fi

kubectl apply -f "${ISSUER_FILE}"
echo "applied ${ISSUER_FILE}"
