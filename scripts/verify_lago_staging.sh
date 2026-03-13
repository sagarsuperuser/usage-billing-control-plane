#!/usr/bin/env bash
set -euo pipefail

namespace="${LAGO_NAMESPACE:-lago}"
release_name="${LAGO_RELEASE_NAME:-lago}"
lago_api_url="${LAGO_API_URL:-}"
lago_api_key="${LAGO_API_KEY:-}"

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

require_cmd kubectl
require_cmd curl

echo_info "checking namespace $namespace"
kubectl get namespace "$namespace" >/dev/null

echo_info "checking Lago pods"
kubectl -n "$namespace" get pods

echo_info "checking core services"
for svc_name in "$release_name-api-svc" "$release_name-front-svc"; do
  kubectl -n "$namespace" get svc "$svc_name" >/dev/null
  echo_pass "service present: $svc_name"
done

echo_info "checking deployments ready"
for deploy_name in \
  "$release_name-api" \
  "$release_name-front" \
  "$release_name-worker"; do
  if kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=2m >/dev/null
    echo_pass "deployment ready: $deploy_name"
  fi
done

if [[ -n "$lago_api_url" ]]; then
  echo_info "checking Lago API reachability: $lago_api_url"
  if [[ -n "$lago_api_key" ]]; then
    http_code="$(curl -sS -o /tmp/lago-staging-verify.out -w '%{http_code}' -H "Authorization: Bearer $lago_api_key" "$lago_api_url/api/v1/customers")"
    if [[ "$http_code" == "200" ]]; then
      echo_pass "authenticated Lago API request succeeded"
    else
      echo "authenticated Lago API request failed: status=$http_code body=$(cat /tmp/lago-staging-verify.out)" >&2
      exit 1
    fi
  else
    http_code="$(curl -ksS -o /tmp/lago-staging-verify.out -w '%{http_code}' "$lago_api_url")"
    if [[ "$http_code" =~ ^(200|301|302|401|403)$ ]]; then
      echo_pass "Lago URL is reachable (status=$http_code)"
    else
      echo "Lago URL is not reachable as expected: status=$http_code body=$(cat /tmp/lago-staging-verify.out)" >&2
      exit 1
    fi
  fi
else
  echo "[warn] LAGO_API_URL not set; skipped URL/API reachability check" >&2
  echo "[warn] This is expected if you are only verifying in-cluster readiness or using a restricted admin path." >&2
fi

echo_pass "Lago staging verification completed"
