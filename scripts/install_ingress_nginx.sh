#!/usr/bin/env bash
set -euo pipefail

namespace="${INGRESS_NGINX_NAMESPACE:-ingress-nginx}"
release_name="${INGRESS_NGINX_RELEASE_NAME:-ingress-nginx}"
chart_version="${INGRESS_NGINX_CHART_VERSION:-4.15.0}"
values_file="${INGRESS_NGINX_VALUES_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/deploy/ingress-nginx/staging-values.yaml}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_file() {
  [[ -f "$1" ]] || {
    echo "required file not found: $1" >&2
    exit 1
  }
}

require_cmd helm
require_cmd kubectl
require_file "$values_file"

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx >/dev/null 2>&1 || true
helm repo update ingress-nginx >/dev/null

helm upgrade --install "$release_name" ingress-nginx/ingress-nginx \
  --namespace "$namespace" \
  --create-namespace \
  --version "$chart_version" \
  -f "$values_file" \
  --wait --timeout 15m

kubectl -n "$namespace" rollout status deployment/"$release_name-controller" --timeout=5m
kubectl -n "$namespace" get svc "$release_name-controller" -o wide

echo "ingress-nginx installed in namespace $namespace using $values_file"
