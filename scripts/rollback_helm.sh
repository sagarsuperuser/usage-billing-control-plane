#!/usr/bin/env bash
set -euo pipefail

environment="${ENVIRONMENT:-staging}"
release_name="${RELEASE_NAME:-lago-alpha}"
namespace="${NAMESPACE:-lago-alpha}"
revision="${REVISION:-}"

if [[ -z "$revision" ]]; then
  echo "REVISION is required" >&2
  exit 1
fi

case "$environment" in
  staging|prod)
    ;;
  *)
    echo "ENVIRONMENT must be one of: staging, prod" >&2
    exit 1
    ;;
esac

helm rollback "$release_name" "$revision" \
  --namespace "$namespace" \
  --wait \
  --timeout 10m

kubectl -n "$namespace" rollout status deployment/"$release_name"-lago-alpha-api --timeout=5m
