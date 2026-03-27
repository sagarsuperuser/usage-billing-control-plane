#!/usr/bin/env bash
set -euo pipefail

namespace="${RELOADER_NAMESPACE:-reloader}"
release_name="${RELOADER_RELEASE_NAME:-reloader}"
chart_version="${RELOADER_CHART_VERSION:-2.2.2}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_cmd helm
require_cmd kubectl

helm repo add stakater https://stakater.github.io/stakater-charts >/dev/null 2>&1 || helm repo add stakater https://stakater.github.io/stakater-charts --force-update >/dev/null
helm repo update stakater >/dev/null
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

helm upgrade --install "$release_name" stakater/reloader \
  --namespace "$namespace" \
  --create-namespace \
  --version "$chart_version" \
  --wait --timeout 10m

kubectl -n "$namespace" rollout status deployment/"$release_name"-reloader --timeout=5m

echo "reloader installed in namespace $namespace"
