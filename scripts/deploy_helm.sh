#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
chart_path="${CHART_PATH:-$repo_root/deploy/helm/lago-alpha}"

environment="${ENVIRONMENT:-staging}"
release_name="${RELEASE_NAME:-lago-alpha}"
namespace="${NAMESPACE:-lago-alpha}"
image_tag="${IMAGE_TAG:-}"

api_image_repository="${API_IMAGE_REPOSITORY:-}"
web_image_repository="${WEB_IMAGE_REPOSITORY:-}"

if [[ -z "$image_tag" ]]; then
  echo "IMAGE_TAG is required" >&2
  exit 1
fi
if [[ -z "$api_image_repository" ]]; then
  echo "API_IMAGE_REPOSITORY is required" >&2
  exit 1
fi
if [[ -z "$web_image_repository" ]]; then
  echo "WEB_IMAGE_REPOSITORY is required" >&2
  exit 1
fi

case "$environment" in
  staging)
    values_file="$chart_path/environments/staging-values.yaml"
    ;;
  prod)
    values_file="$chart_path/environments/prod-values.yaml"
    ;;
  *)
    echo "ENVIRONMENT must be one of: staging, prod" >&2
    exit 1
    ;;
esac

if [[ ! -f "$values_file" ]]; then
  echo "values file not found: $values_file" >&2
  exit 1
fi

helm lint "$chart_path"

helm upgrade --install "$release_name" "$chart_path" \
  --namespace "$namespace" \
  --create-namespace \
  --history-max 20 \
  --atomic \
  --timeout 10m \
  -f "$values_file" \
  --set "api.image.repository=$api_image_repository" \
  --set "api.image.tag=$image_tag" \
  --set "replayWorker.image.repository=$api_image_repository" \
  --set "replayWorker.image.tag=$image_tag" \
  --set "replayDispatcher.image.repository=$api_image_repository" \
  --set "replayDispatcher.image.tag=$image_tag" \
  --set "web.image.repository=$web_image_repository" \
  --set "web.image.tag=$image_tag"

kubectl -n "$namespace" rollout status deployment/"$release_name"-lago-alpha-api --timeout=5m
kubectl -n "$namespace" rollout status deployment/"$release_name"-lago-alpha-replay-worker --timeout=5m || true
kubectl -n "$namespace" rollout status deployment/"$release_name"-lago-alpha-replay-dispatcher --timeout=5m || true
kubectl -n "$namespace" rollout status deployment/"$release_name"-lago-alpha-web --timeout=5m || true
