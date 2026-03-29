#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
baseline_file="$repo_root/config/lago-baseline.env"
if [[ -f "$baseline_file" ]]; then
  # shellcheck disable=SC1090
  source "$baseline_file"
fi
compose_file="${COMPOSE_FILE:-docker-compose.postgres.yml}"
test_database_url="${TEST_DATABASE_URL:-postgres://postgres:postgres@localhost:15432/lago_alpha_test?sslmode=disable}"
test_s3_endpoint="${TEST_S3_ENDPOINT:-http://localhost:19000}"
test_s3_region="${TEST_S3_REGION:-us-east-1}"
test_s3_bucket="${TEST_S3_BUCKET:-lago-alpha-exports}"
test_s3_access_key_id="${TEST_S3_ACCESS_KEY_ID:-minioadmin}"
test_s3_secret_access_key="${TEST_S3_SECRET_ACCESS_KEY:-minioadmin123}"
test_temporal_address="${TEST_TEMPORAL_ADDRESS:-127.0.0.1:17233}"
test_temporal_namespace="${TEST_TEMPORAL_NAMESPACE:-default}"
test_lago_api_url="${TEST_LAGO_API_URL:-}"
test_lago_api_key="${TEST_LAGO_API_KEY:-lago_alpha_test_api_key}"
integration_test_pattern="${INTEGRATION_TEST_PATTERN:-TestEndToEndPreviewReplayReconciliation|TestLargeReplayDatasetDriftThresholds|TestTenantIsolationAcrossAPIKeys|TestRatingRuleGovernanceLifecycle|TestLagoWebhookVisibilityFlow|TestPaymentFailureLifecycleRetryAndOutOfOrderWebhooks|TestAPIKeyLifecycleEndpoints|TestAuditExportToS3}"
run_migrations_integration_test="${INTEGRATION_RUN_MIGRATIONS_TEST:-1}"
bootstrap_lago="${BOOTSTRAP_LAGO_FOR_TESTS:-1}"
lago_repo_path="${LAGO_REPO_PATH:-$repo_root/../lago}"
lago_compose_file="${LAGO_COMPOSE_FILE:-docker-compose.yml}"
cleanup_lago="${CLEANUP_LAGO_ON_EXIT:-0}"
cleanup="${CLEANUP_ON_EXIT:-1}"
verify_lago_backend="${VERIFY_LAGO_BACKEND_FOR_TESTS:-0}"
lago_verify_compose_file="${LAGO_VERIFY_COMPOSE_FILE:-docker-compose.dev.yml}"
debug_on_failure="${DEBUG_ON_FAILURE:-1}"
lago_backend_image_override="${LAGO_BACKEND_IMAGE_OVERRIDE:-${LAGO_STAGING_BACKEND_IMAGE_OVERRIDE:-}}"
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

if [[ -n "$lago_backend_image_override" ]]; then
  export LAGO_BACKEND_IMAGE_OVERRIDE="$lago_backend_image_override"
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

cleanup_fn() {
  if [[ "$cleanup" == "1" ]]; then
    docker compose -f "$repo_root/$compose_file" down >/dev/null 2>&1 || true
  fi
  if [[ "$cleanup_lago" == "1" && "$bootstrap_lago" == "1" ]]; then
    (cd "$lago_stack_root" && docker compose -f "$lago_compose_path" down >/dev/null 2>&1 || true)
  fi
}
trap cleanup_fn EXIT

dump_diagnostics_on_error() {
  local exit_code="$1"
  if [[ "$debug_on_failure" != "1" || "$exit_code" == "0" ]]; then
    return
  fi

  echo "Integration flow failed (exit=${exit_code}). Collecting diagnostics..." >&2
  echo "---- alpha compose ps ----" >&2
  docker compose -f "$repo_root/$compose_file" ps >&2 || true
  echo "---- alpha compose logs (tail=200) ----" >&2
  docker compose -f "$repo_root/$compose_file" logs --tail=200 >&2 || true

  if [[ "$bootstrap_lago" == "1" ]]; then
    echo "---- lago compose ps ----" >&2
    (
      cd "$lago_stack_root" && docker compose -f "$lago_compose_path" ps
    ) >&2 || true
    echo "---- lago compose logs (tail=200) ----" >&2
    (
      cd "$lago_stack_root" && docker compose -f "$lago_compose_path" logs --tail=200
    ) >&2 || true
  fi
}
trap 'dump_diagnostics_on_error "$?"' ERR

cd "$repo_root"

resolve_lago_api_url_from_compose() {
  local port_line
  if ! port_line="$(
    cd "$lago_stack_root"
    docker compose -f "$lago_compose_path" port api 3000 2>/dev/null | head -n1
  )"; then
    return 1
  fi
  if [[ "$port_line" =~ :([0-9]+)$ ]]; then
    echo "http://127.0.0.1:${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
}

if [[ "$bootstrap_lago" == "1" ]]; then
  bootstrap_output="$(
    LAGO_REPO_PATH="$lago_repo_path" \
    LAGO_COMPOSE_FILE="$lago_compose_path" \
    TEST_LAGO_API_URL="$test_lago_api_url" \
    TEST_LAGO_API_KEY="$test_lago_api_key" \
    bash "$repo_root/scripts/bootstrap_lago.sh"
  )"
  printf '%s\n' "$bootstrap_output"

  if parsed_api_url="$(awk -F"'" '/^export TEST_LAGO_API_URL=/{print $2}' <<<"$bootstrap_output" | tail -n1)"; then
    if [[ -n "$parsed_api_url" ]]; then
      test_lago_api_url="$parsed_api_url"
    fi
  fi
  if parsed_api_key="$(awk -F"'" '/^export TEST_LAGO_API_KEY=/{print $2}' <<<"$bootstrap_output" | tail -n1)"; then
    if [[ -n "$parsed_api_key" ]]; then
      test_lago_api_key="$parsed_api_key"
    fi
  fi
fi

if [[ -z "$test_lago_api_url" ]]; then
  test_lago_api_url="$(resolve_lago_api_url_from_compose || true)"
fi
if [[ -z "$test_lago_api_url" || -z "$test_lago_api_key" ]]; then
  echo "TEST_LAGO_API_URL and TEST_LAGO_API_KEY are required for Lago-backed integration tests" >&2
  exit 1
fi

if [[ "$verify_lago_backend" == "1" ]]; then
  verify_script="$lago_repo_path/scripts/verify_e2e.sh"
  if [[ ! -x "$verify_script" ]]; then
    echo "VERIFY_LAGO_BACKEND_FOR_TESTS=1 but verify script not found: $verify_script" >&2
    exit 1
  fi
  (
    cd "$lago_repo_path"
    LAGO_COMPOSE_FILE="$lago_verify_compose_file" ./scripts/verify_e2e.sh
  )
fi

docker compose -f "$compose_file" up -d
"$repo_root/scripts/wait_for_postgres.sh" "$compose_file" postgres 90
bash "$repo_root/scripts/wait_for_http.sh" "${test_s3_endpoint}/minio/health/live" 90
bash "$repo_root/scripts/wait_for_tcp.sh" "$test_temporal_address" 120
go run ./cmd/ensure_temporal_namespace \
  -address "$test_temporal_address" \
  -namespace "$test_temporal_namespace" \
  -timeout 90s

db_name="${test_database_url##*/}"
db_name="${db_name%%\?*}"
if [[ ! "$db_name" =~ ^[a-zA-Z0-9_]+$ ]]; then
  echo "invalid database name derived from TEST_DATABASE_URL: '$db_name'" >&2
  exit 1
fi

if ! docker compose -f "$compose_file" exec -T postgres psql -U postgres -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${db_name}'" | grep -q 1; then
  docker compose -f "$compose_file" exec -T postgres psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${db_name};"
fi

DATABASE_URL="$test_database_url" go run ./cmd/migrate
DATABASE_URL="$test_database_url" go run ./cmd/migrate verify
TEST_DATABASE_URL="$test_database_url" \
TEST_S3_ENDPOINT="$test_s3_endpoint" \
TEST_S3_REGION="$test_s3_region" \
TEST_S3_BUCKET="$test_s3_bucket" \
TEST_S3_ACCESS_KEY_ID="$test_s3_access_key_id" \
TEST_S3_SECRET_ACCESS_KEY="$test_s3_secret_access_key" \
TEST_TEMPORAL_ADDRESS="$test_temporal_address" \
TEST_TEMPORAL_NAMESPACE="$test_temporal_namespace" \
TEST_LAGO_API_URL="$test_lago_api_url" \
TEST_LAGO_API_KEY="$test_lago_api_key" \
go test ./internal/api -run "${integration_test_pattern}" -v

if [[ "$run_migrations_integration_test" == "1" ]]; then
  TEST_DATABASE_URL="$test_database_url" go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v
fi
