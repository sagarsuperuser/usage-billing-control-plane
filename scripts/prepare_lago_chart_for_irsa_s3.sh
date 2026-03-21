#!/usr/bin/env bash
set -euo pipefail

chart_dir="${1:-}"
backend_image_override="${LAGO_BACKEND_IMAGE_OVERRIDE:-}"

if [[ -z "$chart_dir" ]]; then
  echo "usage: $0 <chart-dir>" >&2
  exit 1
fi

if [[ ! -d "$chart_dir/templates" ]]; then
  echo "chart templates directory not found: $chart_dir/templates" >&2
  exit 1
fi

files=(
  api-deployment.yaml
  billing-worker-deployment.yaml
  clock-worker-deployment.yaml
  clock-deployment.yaml
  events-worker-deployment.yaml
  payment-worker-deployment.yaml
  pdf-worker-deployment.yaml
  webhook-worker-deployment.yaml
  worker-deployment.yaml
)

for file in "${files[@]}"; do
  path="$chart_dir/templates/$file"
  [[ -f "$path" ]] || continue

  perl -0pi -e 's#\{\{ if or \.Values\.global\.s3\.accessKeyId \.Values\.minio\.enabled \.Values\.global\.existingSecret \}\}#{{ if or .Values.global.s3.accessKeyId .Values.minio.enabled (and .Values.global.existingSecret (not .Values.global.s3.useIrsa)) }}#g; s#\{\{\- if or \.Values\.global\.s3\.accessKeyId \.Values\.minio\.enabled \.Values\.global\.existingSecret \}\}#{{- if or .Values.global.s3.accessKeyId .Values.minio.enabled (and .Values.global.existingSecret (not .Values.global.s3.useIrsa)) }}#g; s#\{\{ if or \.Values\.global\.s3\.secretAccessKey \.Values\.minio\.enabled \.Values\.global\.existingSecret \}\}#{{ if or .Values.global.s3.secretAccessKey .Values.minio.enabled (and .Values.global.existingSecret (not .Values.global.s3.useIrsa)) }}#g; s#\{\{\- if or \.Values\.global\.s3\.secretAccessKey \.Values\.minio\.enabled \.Values\.global\.existingSecret \}\}#{{- if or .Values.global.s3.secretAccessKey .Values.minio.enabled (and .Values.global.existingSecret (not .Values.global.s3.useIrsa)) }}#g' "$path"

  if [[ -n "$backend_image_override" ]]; then
    perl -0pi -e 's#image:\s*getlago/api:v\{\{\s*\.Values\.version\s*\}\}#image: '"$backend_image_override"'#g' "$path"
  fi
done

printf '[pass] patched Lago chart for IRSA/S3: %s\n' "$chart_dir"
if [[ -n "$backend_image_override" ]]; then
  printf '[pass] patched Lago chart for backend image override: %s\n' "$backend_image_override"
fi
