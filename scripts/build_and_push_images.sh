#!/usr/bin/env bash
set -euo pipefail

environment="${ENVIRONMENT:-staging}"
image_tag="${IMAGE_TAG:-}"
api_image_repository="${API_IMAGE_REPOSITORY:-}"
web_image_repository="${WEB_IMAGE_REPOSITORY:-}"
aws_region="${AWS_REGION:-us-east-1}"
platforms="${DOCKER_PLATFORMS:-linux/amd64}"

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

repo_registry="$(printf '%s\n%s\n' "$api_image_repository" "$web_image_repository" | sed -E 's#^([^/]+)/.*$#\1#' | sort -u)"
repo_count="$(printf '%s\n' "$repo_registry" | sed '/^$/d' | wc -l | tr -d ' ')"
if [[ "$repo_count" != "1" ]]; then
  echo "API_IMAGE_REPOSITORY and WEB_IMAGE_REPOSITORY must share the same registry" >&2
  exit 1
fi
registry_host="$(printf '%s\n' "$repo_registry" | head -n1)"

echo "[info] logging into ECR registry ${registry_host} (${aws_region})"
aws ecr get-login-password --region "$aws_region" | docker login --username AWS --password-stdin "$registry_host"

if ! docker buildx inspect staging-builder >/dev/null 2>&1; then
  echo "[info] creating docker buildx builder staging-builder"
  docker buildx create --name staging-builder --driver docker-container --use >/dev/null
fi
docker buildx use staging-builder >/dev/null
docker buildx inspect --bootstrap >/dev/null

echo "[info] building API image for ${platforms}"
docker buildx build \
  --platform "$platforms" \
  -t "${api_image_repository}:${image_tag}" \
  --push \
  .

echo "[info] building web image for ${platforms}"
docker buildx build \
  --platform "$platforms" \
  -f web/Dockerfile \
  -t "${web_image_repository}:${image_tag}" \
  --push \
  web

echo "[info] pushed images for environment=${environment} tag=${image_tag}"
