#!/usr/bin/env bash
set -euo pipefail

required_cmds=(helm kubectl jq)
for cmd in "${required_cmds[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
done

trim() {
  echo "$1" | xargs
}

bool_env() {
  local raw
  raw="$(echo "${1:-}" | tr '[:upper:]' '[:lower:]' | xargs)"
  case "$raw" in
    1|true|yes|y) echo "1" ;;
    0|false|no|n|"") echo "0" ;;
    *)
      echo "invalid boolean value: $1" >&2
      exit 1
      ;;
  esac
}

run_cmd() {
  if [[ "$PLAN_ONLY" == "1" ]]; then
    printf '[plan] '
    printf '%q ' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

ENVIRONMENT="$(trim "${ENVIRONMENT:-staging}")"
RELEASE_NAME="$(trim "${RELEASE_NAME:-lago-alpha}")"
NAMESPACE="$(trim "${NAMESPACE:-lago-alpha}")"
IMAGE_TAG="$(trim "${IMAGE_TAG:-}")"
API_IMAGE_REPOSITORY="$(trim "${API_IMAGE_REPOSITORY:-}")"
WEB_IMAGE_REPOSITORY="$(trim "${WEB_IMAGE_REPOSITORY:-}")"
ROLLBACK_TO_REVISION="$(trim "${ROLLBACK_TO_REVISION:-}")"
REDEPLOY_AFTER_ROLLBACK="$(bool_env "${REDEPLOY_AFTER_ROLLBACK:-1}")"
PLAN_ONLY="$(bool_env "${PLAN_ONLY:-0}")"
CONFIRM_RELEASE_REHEARSAL="$(trim "${CONFIRM_RELEASE_REHEARSAL:-}")"
SMOKE_HEALTH_URL="$(trim "${SMOKE_HEALTH_URL:-}")"
SMOKE_API_KEY="$(trim "${SMOKE_API_KEY:-}")"

case "$ENVIRONMENT" in
  staging|prod)
    ;;
  *)
    echo "ENVIRONMENT must be one of: staging, prod" >&2
    exit 1
    ;;
esac

if [[ "$CONFIRM_RELEASE_REHEARSAL" != "YES_I_UNDERSTAND" ]]; then
  echo "set CONFIRM_RELEASE_REHEARSAL=YES_I_UNDERSTAND to execute release rehearsal" >&2
  exit 1
fi
if [[ -z "$IMAGE_TAG" || -z "$API_IMAGE_REPOSITORY" || -z "$WEB_IMAGE_REPOSITORY" ]]; then
  echo "IMAGE_TAG, API_IMAGE_REPOSITORY, and WEB_IMAGE_REPOSITORY are required" >&2
  exit 1
fi

check_smoke() {
  local stage="$1"
  if [[ -z "$SMOKE_HEALTH_URL" ]]; then
    return 0
  fi
  if ! command -v curl >/dev/null 2>&1; then
    echo "missing required command for smoke check: curl" >&2
    exit 1
  fi

  local -a headers=(-H "Accept: application/json")
  if [[ -n "$SMOKE_API_KEY" ]]; then
    headers+=(-H "X-API-Key: ${SMOKE_API_KEY}")
  fi

  echo "[info] smoke check (${stage}): ${SMOKE_HEALTH_URL}"
  if [[ "$PLAN_ONLY" == "1" ]]; then
    printf '[plan] curl -sS -o /dev/null -w %%{http_code} '
    printf '%q ' "${headers[@]}"
    printf '%q\n' "$SMOKE_HEALTH_URL"
    return 0
  fi

  status_code="$(curl -sS -o /dev/null -w '%{http_code}' "${headers[@]}" "$SMOKE_HEALTH_URL")"
  if [[ "$status_code" != "200" ]]; then
    echo "smoke check failed for ${stage}: status=${status_code}" >&2
    exit 1
  fi
  echo "[pass] smoke check (${stage}) returned 200"
}

current_revision=""
if [[ "$PLAN_ONLY" == "1" ]]; then
  current_revision="<plan-only>"
else
  history_json="$(helm history "$RELEASE_NAME" -n "$NAMESPACE" -o json)"
  current_revision="$(jq -r '[.[] | select(.status == "deployed") | .revision] | max // empty' <<<"$history_json")"
  if [[ -z "$current_revision" ]]; then
    echo "failed to resolve current deployed helm revision for ${RELEASE_NAME} in ${NAMESPACE}" >&2
    exit 1
  fi
fi
echo "[info] current deployed revision: $current_revision"

echo "[info] deploying candidate release IMAGE_TAG=$IMAGE_TAG"
run_cmd env \
  ENVIRONMENT="$ENVIRONMENT" \
  RELEASE_NAME="$RELEASE_NAME" \
  NAMESPACE="$NAMESPACE" \
  IMAGE_TAG="$IMAGE_TAG" \
  API_IMAGE_REPOSITORY="$API_IMAGE_REPOSITORY" \
  WEB_IMAGE_REPOSITORY="$WEB_IMAGE_REPOSITORY" \
  ./scripts/deploy_helm.sh
check_smoke "post-deploy"

rollback_revision="$ROLLBACK_TO_REVISION"
if [[ -z "$rollback_revision" ]]; then
  rollback_revision="$current_revision"
fi
echo "[info] rolling back to revision: $rollback_revision"
run_cmd env \
  ENVIRONMENT="$ENVIRONMENT" \
  RELEASE_NAME="$RELEASE_NAME" \
  NAMESPACE="$NAMESPACE" \
  REVISION="$rollback_revision" \
  ./scripts/rollback_helm.sh
check_smoke "post-rollback"

if [[ "$REDEPLOY_AFTER_ROLLBACK" == "1" ]]; then
  echo "[info] redeploying candidate release after rollback"
  run_cmd env \
    ENVIRONMENT="$ENVIRONMENT" \
    RELEASE_NAME="$RELEASE_NAME" \
    NAMESPACE="$NAMESPACE" \
    IMAGE_TAG="$IMAGE_TAG" \
    API_IMAGE_REPOSITORY="$API_IMAGE_REPOSITORY" \
    WEB_IMAGE_REPOSITORY="$WEB_IMAGE_REPOSITORY" \
    ./scripts/deploy_helm.sh
  check_smoke "post-redeploy"
fi

if [[ "$PLAN_ONLY" == "1" ]]; then
  final_revision="<plan-only>"
else
  final_history_json="$(helm history "$RELEASE_NAME" -n "$NAMESPACE" -o json)"
  final_revision="$(jq -r '[.[] | select(.status == "deployed") | .revision] | max // empty' <<<"$final_history_json")"
fi

jq -n \
  --arg environment "$ENVIRONMENT" \
  --arg release_name "$RELEASE_NAME" \
  --arg namespace "$NAMESPACE" \
  --arg image_tag "$IMAGE_TAG" \
  --arg current_revision "$current_revision" \
  --arg rollback_revision "$rollback_revision" \
  --arg final_revision "$final_revision" \
  --argjson redeploy_after_rollback "$REDEPLOY_AFTER_ROLLBACK" \
  --argjson plan_only "$PLAN_ONLY" \
  '{
    environment: $environment,
    release_name: $release_name,
    namespace: $namespace,
    image_tag: $image_tag,
    current_revision: $current_revision,
    rollback_revision: $rollback_revision,
    final_revision: $final_revision,
    redeploy_after_rollback: ($redeploy_after_rollback == 1),
    plan_only: ($plan_only == 1)
  }'
