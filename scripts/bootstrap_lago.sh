#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
lago_repo_path="${LAGO_REPO_PATH:-$repo_root/../lago}"
lago_compose_file="${LAGO_COMPOSE_FILE:-docker-compose.yml}"
start_lago_temporal="${START_LAGO_TEMPORAL:-0}"
test_lago_api_url="${TEST_LAGO_API_URL:-}"
test_lago_api_key="${TEST_LAGO_API_KEY:-lago_alpha_test_api_key}"
lago_org_name="${LAGO_ORG_NAME:-Lago Alpha Test Org}"
lago_org_user_email="${LAGO_ORG_USER_EMAIL:-alpha-test@getlago.local}"
lago_org_user_password="${LAGO_ORG_USER_PASSWORD:-AlphaTest123!}"
startup_lago_api_url="${test_lago_api_url:-http://localhost:3000}"

if [[ ! -f "$lago_repo_path/$lago_compose_file" ]]; then
  echo "Lago compose file not found: $lago_repo_path/$lago_compose_file" >&2
  exit 1
fi

echo "Bootstrapping Lago from: $lago_repo_path/$lago_compose_file"
available_services="$(
  cd "$lago_repo_path"
  docker compose -f "$lago_compose_file" config --services
)"

has_service() {
  local name="$1"
  grep -qx "$name" <<<"$available_services"
}

startup_services=(db redis migrate api)
if has_service "api-worker"; then
  startup_services+=(api-worker)
fi
if has_service "api-clock"; then
  startup_services+=(api-clock)
elif has_service "clock"; then
  startup_services+=(clock)
fi
if [[ "$start_lago_temporal" == "1" ]]; then
  if has_service "temporal"; then
    startup_services+=(temporal)
  fi
  if has_service "temporal-ui"; then
    startup_services+=(temporal-ui)
  fi
  if has_service "api-temporal-worker"; then
    startup_services+=(api-temporal-worker)
  fi
fi

discover_lago_api_url() {
  local port_line
  if ! port_line="$(
    cd "$lago_repo_path"
    docker compose -f "$lago_compose_file" port api 3000 2>/dev/null | head -n1
  )"; then
    return 1
  fi
  if [[ "$port_line" =~ :([0-9]+)$ ]]; then
    echo "http://127.0.0.1:${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

(
  cd "$lago_repo_path"
  LAGO_API_URL="$startup_lago_api_url" \
  LAGO_CREATE_ORG=true \
  LAGO_ORG_NAME="$lago_org_name" \
  LAGO_ORG_USER_EMAIL="$lago_org_user_email" \
  LAGO_ORG_USER_PASSWORD="$lago_org_user_password" \
  LAGO_ORG_API_KEY="$test_lago_api_key" \
  docker compose -f "$lago_compose_file" up -d "${startup_services[@]}"
)

resolved_lago_api_url="$test_lago_api_url"
if [[ -z "$resolved_lago_api_url" ]]; then
  resolved_lago_api_url="$(discover_lago_api_url || true)"
fi
if [[ -z "$resolved_lago_api_url" ]]; then
  resolved_lago_api_url="$startup_lago_api_url"
fi

bash "$repo_root/scripts/wait_for_http.sh" "${resolved_lago_api_url}/health" 180

if ! probe_response="$(curl -sS -H "Authorization: Bearer ${test_lago_api_key}" "${resolved_lago_api_url}/api/v1/billable_metrics" -w $'\n%{http_code}')"; then
  echo "failed to reach Lago API with configured key" >&2
  exit 1
fi
probe_status="${probe_response##*$'\n'}"
probe_body="${probe_response%$'\n'*}"
if [[ "$probe_status" != "200" ]]; then
  echo "Lago API key probe failed: status=$probe_status body=$probe_body" >&2
  exit 1
fi

echo "Lago is ready for integration tests."
echo "export TEST_LAGO_API_URL='${resolved_lago_api_url}'"
echo "export TEST_LAGO_API_KEY='${test_lago_api_key}'"
