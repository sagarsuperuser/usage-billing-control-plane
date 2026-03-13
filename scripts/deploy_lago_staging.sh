#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
namespace="${LAGO_NAMESPACE:-lago}"
release_name="${LAGO_RELEASE_NAME:-lago}"
values_file="${LAGO_VALUES_FILE:-$repo_root/deploy/lago/environments/staging-values.yaml}"
chart_version="${LAGO_CHART_VERSION:-}"
helm_repo_name="${LAGO_HELM_REPO_NAME:-lago}"
helm_repo_url="${LAGO_HELM_REPO_URL:-https://charts.getlago.com}"
chart_ref="${LAGO_CHART_REF:-$helm_repo_name/lago}"
sync_secrets="${LAGO_SYNC_SECRETS:-1}"
sync_script="${LAGO_SYNC_SCRIPT:-$repo_root/scripts/sync_lago_staging_secrets.sh}"

echo_info() {
  printf '[info] %s\n' "$*"
}

echo_pass() {
  printf '[pass] %s\n' "$*"
}

require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "missing required command: $cmd" >&2
    exit 1
  }
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || {
    echo "required file not found: $path" >&2
    exit 1
  }
}

require_cmd helm
require_cmd kubectl
require_file "$values_file"

if [[ "$sync_secrets" == "1" ]]; then
  require_cmd aws
  require_file "$sync_script"
fi

echo_info "ensuring namespace $namespace exists"
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

if [[ "$sync_secrets" == "1" ]]; then
  echo_info "ensuring Lago secrets are sourced from AWS Secrets Manager via ExternalSecret in namespace $namespace"
  LAGO_NAMESPACE="$namespace" "$sync_script"
fi

echo_info "configuring Helm repo $helm_repo_name -> $helm_repo_url"
helm repo add "$helm_repo_name" "$helm_repo_url" >/dev/null 2>&1 || helm repo add "$helm_repo_name" "$helm_repo_url" --force-update >/dev/null
helm repo update "$helm_repo_name" >/dev/null

helm_args=(
  upgrade --install "$release_name" "$chart_ref"
  --namespace "$namespace"
  --create-namespace
  --atomic
  --timeout 15m
  -f "$values_file"
)

if [[ -n "$chart_version" ]]; then
  helm_args+=(--version "$chart_version")
fi

echo_info "deploying Lago release=$release_name namespace=$namespace chart=$chart_ref"
helm "${helm_args[@]}"

echo_info "waiting for core Lago rollouts"
for deploy_name in \
  "$release_name-api" \
  "$release_name-front" \
  "$release_name-worker" \
  "$release_name-webhook-worker" \
  "$release_name-clock" \
  "$release_name-payment-worker"; do
  if kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=10m
  fi
done

echo_pass "Lago staging deployment completed"
echo
echo "Next:"
echo "  1. Run: LAGO_API_URL=<https-url> make lago-staging-verify"
echo "  2. Complete the manual items in docs/lago-staging-bootstrap.md"
