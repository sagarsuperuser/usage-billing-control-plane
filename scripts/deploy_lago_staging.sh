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
safe_shared_pvc_rollout="${LAGO_SAFE_SHARED_PVC_ROLLOUT:-1}"
shared_pvc_rollout_timeout="${LAGO_SHARED_PVC_ROLLOUT_TIMEOUT:-10m}"
shared_pvc_probe_timeout="${LAGO_SHARED_PVC_PROBE_TIMEOUT:-45s}"
lago_service_account_name="${LAGO_SERVICE_ACCOUNT_NAME:-lago-serviceaccount}"
lago_service_account_role_arn="${LAGO_SERVICE_ACCOUNT_ROLE_ARN:-}"
ensure_service_account_script="${LAGO_ENSURE_SERVICE_ACCOUNT_SCRIPT:-$repo_root/scripts/ensure_lago_service_account.sh}"
lago_chart_patch_script="${LAGO_CHART_PATCH_SCRIPT:-$repo_root/scripts/prepare_lago_chart_for_irsa_s3.sh}"
use_irsa_s3_chart_patch="${LAGO_USE_IRSA_S3_CHART_PATCH:-1}"
backend_image_override="${LAGO_BACKEND_IMAGE_OVERRIDE:-}"
prepared_chart_root=""

cleanup() {
  if [[ -n "$prepared_chart_root" && -d "$prepared_chart_root" ]]; then
    rm -rf "$prepared_chart_root"
  fi
}
trap cleanup EXIT

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

if [[ -n "$lago_service_account_name" ]]; then
  require_file "$ensure_service_account_script"
fi

if [[ "$use_irsa_s3_chart_patch" == "1" ]]; then
  require_file "$lago_chart_patch_script"
fi

deployment_exists() {
  local deploy_name="$1"
  kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1
}

set_backend_image_override() {
  local deploy_name="$1"

  [[ -n "$backend_image_override" ]] || return 0
  deployment_exists "$deploy_name" || return 0

  echo_info "overriding backend image for $deploy_name -> $backend_image_override"
  kubectl -n "$namespace" set image deployment/"$deploy_name" "$deploy_name=$backend_image_override" >/dev/null
}

deployment_replicas() {
  local deploy_name="$1"
  kubectl -n "$namespace" get deploy "$deploy_name" -o jsonpath='{.spec.replicas}'
}

wait_for_deployment() {
  local deploy_name="$1"
  kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout="$shared_pvc_rollout_timeout" >/dev/null
}

rollout_is_ready() {
  local deploy_name="$1"
  kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout="$shared_pvc_probe_timeout" >/dev/null 2>&1
}

safe_restart_shared_pvc_deployment() {
  local deploy_name="$1"
  local replicas

  if ! deployment_exists "$deploy_name"; then
    return 0
  fi

  replicas="$(deployment_replicas "$deploy_name")"
  if [[ -z "$replicas" || "$replicas" == "0" ]]; then
    echo_info "skipping shared-PVC rollout for $deploy_name (replicas=$replicas)"
    return 0
  fi

  if rollout_is_ready "$deploy_name"; then
    echo_pass "deployment ready without shared-PVC restart: $deploy_name"
    return 0
  fi

  echo_info "deployment $deploy_name is not ready after Helm apply; performing shared-PVC safe restart"
  kubectl -n "$namespace" scale deployment/"$deploy_name" --replicas=0 >/dev/null
  wait_for_deployment "$deploy_name"
  kubectl -n "$namespace" scale deployment/"$deploy_name" --replicas="$replicas" >/dev/null
  wait_for_deployment "$deploy_name"
  echo_pass "deployment rolled out with shared-PVC safe restart: $deploy_name"
}

echo_info "ensuring namespace $namespace exists"
kubectl get namespace "$namespace" >/dev/null 2>&1 || kubectl create namespace "$namespace" >/dev/null

if [[ "$sync_secrets" == "1" ]]; then
  echo_info "ensuring Lago secrets are bootstrapped from AWS Secrets Manager in namespace $namespace"
  LAGO_NAMESPACE="$namespace" "$sync_script"
fi

if [[ -n "$lago_service_account_name" ]]; then
  echo_info "ensuring Lago service account $namespace/$lago_service_account_name exists"
  LAGO_NAMESPACE="$namespace" \
  LAGO_SERVICE_ACCOUNT_NAME="$lago_service_account_name" \
  LAGO_SERVICE_ACCOUNT_ROLE_ARN="$lago_service_account_role_arn" \
  "$ensure_service_account_script"
fi

echo_info "configuring Helm repo $helm_repo_name -> $helm_repo_url"
helm repo add "$helm_repo_name" "$helm_repo_url" >/dev/null 2>&1 || helm repo add "$helm_repo_name" "$helm_repo_url" --force-update >/dev/null
helm repo update "$helm_repo_name" >/dev/null

chart_to_deploy="$chart_ref"
if [[ "$use_irsa_s3_chart_patch" == "1" ]]; then
  local_chart_name="${chart_ref##*/}"
  prepared_chart_root="$(mktemp -d)"

  echo_info "pulling Lago chart locally for IRSA/S3 patch"
  pull_args=(pull "$chart_ref" --untar --untardir "$prepared_chart_root")
  if [[ -n "$chart_version" ]]; then
    pull_args+=(--version "$chart_version")
  fi
  helm "${pull_args[@]}" >/dev/null

  chart_to_deploy="$prepared_chart_root/$local_chart_name"
  "$lago_chart_patch_script" "$chart_to_deploy"
  echo_info "using patched local chart $chart_to_deploy"
fi

helm_args=(
  upgrade --install "$release_name" "$chart_to_deploy"
  --namespace "$namespace"
  --create-namespace
  --timeout 15m
  -f "$values_file"
)

if [[ -n "$chart_version" && "$use_irsa_s3_chart_patch" != "1" ]]; then
  helm_args+=(--version "$chart_version")
fi

echo_info "deploying Lago release=$release_name namespace=$namespace chart=$chart_to_deploy"
helm "${helm_args[@]}"

if [[ -n "$backend_image_override" ]]; then
  echo_info "applying backend image override to Lago backend deployments"
  for deploy_name in \
    "$release_name-api" \
    "$release_name-billing-worker" \
    "$release_name-clock" \
    "$release_name-clock-worker" \
    "$release_name-events-worker" \
    "$release_name-payment-worker" \
    "$release_name-pdf-worker" \
    "$release_name-webhook-worker" \
    "$release_name-worker"; do
    set_backend_image_override "$deploy_name"
  done
fi

echo_info "waiting for non-shared Lago deployments"
for deploy_name in \
  "$release_name-front" \
  "$release_name-webhook-worker" \
  "$release_name-clock" \
  "$release_name-events-worker" \
  "$release_name-pdf"; do
  if kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=10m
  fi
done

if [[ "$safe_shared_pvc_rollout" == "1" ]]; then
  echo_info "reconciling shared-PVC Lago deployments sequentially"
  for deploy_name in \
    "$release_name-api" \
    "$release_name-billing-worker" \
    "$release_name-clock-worker" \
    "$release_name-payment-worker" \
    "$release_name-pdf-worker" \
    "$release_name-worker"; do
    safe_restart_shared_pvc_deployment "$deploy_name"
  done
else
  echo_info "waiting for shared-PVC Lago deployments without safe restart"
  for deploy_name in \
    "$release_name-api" \
    "$release_name-billing-worker" \
    "$release_name-clock-worker" \
    "$release_name-payment-worker" \
    "$release_name-pdf-worker" \
    "$release_name-worker"; do
    if deployment_exists "$deploy_name"; then
      kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=10m
    fi
  done
fi

echo_pass "Lago staging deployment completed"
echo
echo "Next:"
echo "  1. Run: LAGO_API_URL=<https-url> make lago-staging-verify"
echo "  2. Complete the manual items in docs/lago-staging-bootstrap.md"
