#!/usr/bin/env bash
set -euo pipefail

namespace="${EXTERNAL_SECRETS_NAMESPACE:-external-secrets}"
release_name="${EXTERNAL_SECRETS_RELEASE_NAME:-external-secrets}"
chart_version="${EXTERNAL_SECRETS_CHART_VERSION:-2.1.0}"
role_arn="${EXTERNAL_SECRETS_IRSA_ROLE_ARN:-arn:aws:iam::139831607173:role/lago-alpha-staging-external-secrets-irsa}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_cmd helm
require_cmd kubectl

helm repo add external-secrets https://charts.external-secrets.io >/dev/null 2>&1 || helm repo add external-secrets https://charts.external-secrets.io --force-update >/dev/null
helm repo update external-secrets >/dev/null
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

helm upgrade --install "$release_name" external-secrets/external-secrets \
  --namespace "$namespace" \
  --create-namespace \
  --version "$chart_version" \
  --set installCRDs=true \
  --set serviceAccount.create=true \
  --set serviceAccount.name=external-secrets \
  --set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="$role_arn" \
  --wait --timeout 10m

kubectl -n "$namespace" rollout status deployment/"$release_name" --timeout=5m

echo "external-secrets installed in namespace $namespace"
