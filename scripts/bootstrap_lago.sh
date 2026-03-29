#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
baseline_file="$repo_root/config/lago-baseline.env"
if [[ -f "$baseline_file" ]]; then
  # shellcheck disable=SC1090
  source "$baseline_file"
fi
lago_repo_path="${LAGO_REPO_PATH:-$repo_root/../lago}"
lago_compose_file="${LAGO_COMPOSE_FILE:-docker-compose.yml}"
start_lago_temporal="${START_LAGO_TEMPORAL:-0}"
test_lago_api_url="${TEST_LAGO_API_URL:-}"
test_lago_api_key="${TEST_LAGO_API_KEY:-lago_alpha_test_api_key}"
lago_org_name="${LAGO_ORG_NAME:-Lago Alpha Test Org}"
lago_org_user_email="${LAGO_ORG_USER_EMAIL:-alpha-test@getlago.local}"
lago_org_user_password="${LAGO_ORG_USER_PASSWORD:-AlphaTest123!}"
startup_lago_api_url="${test_lago_api_url:-http://localhost:3000}"
lago_backend_image_override="${LAGO_BACKEND_IMAGE_OVERRIDE:-${LAGO_STAGING_BACKEND_IMAGE_OVERRIDE:-}}"
lago_compose_override_file=""
lago_compose_path=""
lago_stack_root=""
repo_lago_env_file=""
lago_env_file=""
default_lago_env_file=""

if [[ "$lago_compose_file" = /* ]]; then
  lago_compose_path="$lago_compose_file"
elif [[ -f "$lago_repo_path/$lago_compose_file" ]]; then
  lago_compose_path="$lago_repo_path/$lago_compose_file"
elif [[ -f "$repo_root/$lago_compose_file" ]]; then
  lago_compose_path="$repo_root/$lago_compose_file"
fi

if [[ -z "$lago_compose_path" || ! -f "$lago_compose_path" ]]; then
  echo "Lago compose file not found: $lago_compose_file" >&2
  exit 1
fi

lago_stack_root="$(cd "$(dirname "$lago_compose_path")" && pwd)"
repo_lago_env_file="$lago_stack_root/.env"
lago_env_file="${LAGO_ENV_FILE:-$repo_lago_env_file}"
default_lago_env_file="$lago_stack_root/.env.development.default"

if [[ ! -f "$default_lago_env_file" && -f "$lago_repo_path/.env.development.default" ]]; then
  default_lago_env_file="$lago_repo_path/.env.development.default"
fi

if [[ ! -f "$repo_lago_env_file" ]]; then
  if [[ -f "$lago_env_file" && "$lago_env_file" != "$repo_lago_env_file" ]]; then
    cp "$lago_env_file" "$repo_lago_env_file"
  elif [[ -f "$default_lago_env_file" ]]; then
    cp "$default_lago_env_file" "$repo_lago_env_file"
  fi
fi

if [[ ! -f "$repo_lago_env_file" && -f "$default_lago_env_file" ]]; then
  cp "$default_lago_env_file" "$repo_lago_env_file"
fi

if [[ ! -f "$repo_lago_env_file" ]]; then
  : >"$repo_lago_env_file"
fi

set_env_var() {
  local key="$1"
  local value="$2"
  if grep -q "^${key}=" "$repo_lago_env_file"; then
    perl -0pi -e "s/^${key}=.*\$/${key}=${value}/m" "$repo_lago_env_file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$repo_lago_env_file"
  fi
}

ensure_non_empty_env_var() {
  local key="$1"
  local value="$2"
  local current=""
  current="$(awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, "", $0); print $0; exit }' "$repo_lago_env_file")"
  if [[ -z "$current" ]]; then
    set_env_var "$key" "$value"
  fi
}

ensure_non_empty_env_var "SECRET_KEY_BASE" "$(openssl rand -hex 32)"
ensure_non_empty_env_var "LAGO_RSA_PRIVATE_KEY" "$(openssl genrsa 2048 | openssl base64 -A)"
ensure_non_empty_env_var "LAGO_ENCRYPTION_PRIMARY_KEY" "$(openssl rand -hex 32)"
ensure_non_empty_env_var "LAGO_ENCRYPTION_DETERMINISTIC_KEY" "$(openssl rand -hex 32)"
ensure_non_empty_env_var "LAGO_ENCRYPTION_KEY_DERIVATION_SALT" "$(openssl rand -hex 32)"

cleanup_override_file() {
  if [[ -n "$lago_compose_override_file" && -f "$lago_compose_override_file" ]]; then
    rm -f "$lago_compose_override_file"
  fi
}
trap cleanup_override_file EXIT

if [[ -n "$lago_backend_image_override" ]]; then
  export LAGO_BACKEND_IMAGE_OVERRIDE="$lago_backend_image_override"
fi

compose_args=(-f "$lago_compose_path")

echo "Bootstrapping Lago from: $lago_compose_path"
available_services="$(
  cd "$lago_stack_root"
  docker compose "${compose_args[@]}" config --services
)"

if [[ -n "$lago_backend_image_override" ]]; then
  lago_compose_override_file="$(mktemp /tmp/lago-compose-override.XXXXXX.yml)"
  {
    printf 'services:\n'
    printf '  migrate:\n    image: %s\n' "$lago_backend_image_override"
    if grep -qx "api" <<<"$available_services"; then
      printf '  api:\n    image: %s\n' "$lago_backend_image_override"
    fi
    if grep -qx "api-worker" <<<"$available_services"; then
      printf '  api-worker:\n    image: %s\n' "$lago_backend_image_override"
    fi
    if grep -qx "api-clock" <<<"$available_services"; then
      printf '  api-clock:\n    image: %s\n' "$lago_backend_image_override"
    fi
    if grep -qx "clock" <<<"$available_services"; then
      printf '  clock:\n    image: %s\n' "$lago_backend_image_override"
    fi
  } >"$lago_compose_override_file"
  compose_args+=(-f "$lago_compose_override_file")
fi

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
    cd "$lago_stack_root"
    docker compose "${compose_args[@]}" port api 3000 2>/dev/null | head -n1
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
  cd "$lago_stack_root"
  docker compose "${compose_args[@]}" down --remove-orphans >/dev/null 2>&1 || true
)

(
  cd "$lago_stack_root"
  LAGO_API_URL="$startup_lago_api_url" \
  LAGO_CREATE_ORG=true \
  LAGO_ORG_NAME="$lago_org_name" \
  LAGO_ORG_USER_EMAIL="$lago_org_user_email" \
  LAGO_ORG_USER_PASSWORD="$lago_org_user_password" \
  LAGO_ORG_API_KEY="$test_lago_api_key" \
  docker compose "${compose_args[@]}" up -d --force-recreate "${startup_services[@]}"
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
