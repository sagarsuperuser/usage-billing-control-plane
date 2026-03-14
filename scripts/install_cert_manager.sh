#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${CERT_MANAGER_NAMESPACE:-cert-manager}"
VERSION="${CERT_MANAGER_VERSION:-v1.18.2}"

helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
helm repo update jetstack >/dev/null

helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --version "${VERSION}" \
  --set crds.enabled=true

kubectl -n "${NAMESPACE}" rollout status deployment/cert-manager --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/cert-manager-webhook --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/cert-manager-cainjector --timeout=180s

echo "cert-manager is ready in namespace ${NAMESPACE}"
