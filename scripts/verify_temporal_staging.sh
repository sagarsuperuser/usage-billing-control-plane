#!/usr/bin/env bash
set -euo pipefail

namespace="${TEMPORAL_NAMESPACE_K8S:-temporal}"
release_name="${TEMPORAL_RELEASE_NAME:-temporal}"
address="${TEMPORAL_ADDRESS:-${release_name}-frontend.${namespace}.svc.cluster.local:7233}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { echo "missing required command: $1" >&2; exit 1; }
}

require_cmd kubectl

printf '[info] namespace=%s release=%s address=%s\n' "$namespace" "$release_name" "$address"

kubectl get namespace "$namespace" >/dev/null
kubectl -n "$namespace" get deploy,pod,job,svc

for deploy_name in temporal-frontend temporal-history temporal-matching temporal-worker temporal-admintools; do
  if kubectl -n "$namespace" get deploy "$deploy_name" >/dev/null 2>&1; then
    kubectl -n "$namespace" rollout status deployment/"$deploy_name" --timeout=5m >/dev/null
  fi
done

kubectl -n "$namespace" get svc "${release_name}-frontend" >/dev/null
kubectl -n "$namespace" exec deploy/temporal-admintools -- sh -lc 'temporal operator namespace describe --namespace default --address temporal-frontend:7233 >/dev/null 2>&1 || temporal operator namespace create --namespace default --retention 3d --address temporal-frontend:7233 >/dev/null'

printf '[pass] Temporal staging is ready at %s\n' "$address"
