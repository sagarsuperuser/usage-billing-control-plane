#!/usr/bin/env bash
# Integration test runner: starts Postgres + Temporal, runs migrations, executes Go tests.
# Lago is no longer required — the billing engine is fully native.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

compose_file="${COMPOSE_FILE:-docker-compose.postgres.yml}"
test_database_url="${TEST_DATABASE_URL:-postgres://postgres:postgres@localhost:15432/lago_alpha_test?sslmode=disable}"
test_s3_endpoint="${TEST_S3_ENDPOINT:-http://localhost:19000}"
test_s3_region="${TEST_S3_REGION:-us-east-1}"
test_s3_bucket="${TEST_S3_BUCKET:-lago-alpha-exports}"
test_s3_access_key_id="${TEST_S3_ACCESS_KEY_ID:-minioadmin}"
test_s3_secret_access_key="${TEST_S3_SECRET_ACCESS_KEY:-minioadmin123}"
test_temporal_address="${TEST_TEMPORAL_ADDRESS:-127.0.0.1:17233}"
test_temporal_namespace="${TEST_TEMPORAL_NAMESPACE:-default}"
integration_test_pattern="${INTEGRATION_TEST_PATTERN:-TestEndToEndPreviewReplayReconciliation|TestRatingRuleGovernanceLifecycle|TestPaymentFailureLifecycleRetryAndOutOfOrderWebhooks|TestAPIKeyLifecycleEndpoints|TestAuditExportToS3}"
run_migrations_integration_test="${INTEGRATION_RUN_MIGRATIONS_TEST:-1}"
run_large_replay_dataset="${RUN_LARGE_REPLAY_DATASET:-0}"
large_replay_event_count="${LARGE_REPLAY_EVENT_COUNT:-2000}"
large_replay_max_mismatch_rows="${LARGE_REPLAY_MAX_MISMATCH_ROWS:-0}"
large_replay_max_total_abs_delta_cents="${LARGE_REPLAY_MAX_TOTAL_ABS_DELTA_CENTS:-0}"
cleanup="${CLEANUP_ON_EXIT:-1}"
debug_on_failure="${DEBUG_ON_FAILURE:-1}"

cleanup_fn() {
  local exit_code=$?
  if [[ "$cleanup" == "1" ]]; then
    echo "Stopping compose services..."
    docker compose -f "$compose_file" down --volumes --remove-orphans 2>/dev/null || true
  fi
  if [[ $exit_code -ne 0 && "$debug_on_failure" == "1" ]]; then
    echo "---- alpha compose ps ----"
    docker compose -f "$compose_file" ps 2>/dev/null || true
    echo "---- alpha compose logs (tail=200) ----"
    docker compose -f "$compose_file" logs --tail=200 2>/dev/null || true
  fi
  exit $exit_code
}
trap cleanup_fn EXIT

echo "Starting Postgres..."
docker compose -f "$compose_file" up -d

echo "Waiting for Postgres health..."
"$repo_root/scripts/wait_for_postgres.sh" "$compose_file" postgres 90

echo "Creating test database if it does not exist..."
docker compose -f "$compose_file" exec -T postgres \
  psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'lago_alpha_test'" \
  | grep -q 1 || \
  docker compose -f "$compose_file" exec -T postgres \
  psql -U postgres -c "CREATE DATABASE lago_alpha_test"

echo "Running migrations..."
DATABASE_URL="$test_database_url" go run "$repo_root/cmd/migrate"

# Build test flags
test_flags=(-v -count=1 -timeout 20m)
if [[ -n "$integration_test_pattern" ]]; then
  test_flags+=(-run "$integration_test_pattern")
fi

if [[ "$run_migrations_integration_test" == "1" ]]; then
  test_flags+=(-run "TestMigrationsRunCleanly|$integration_test_pattern")
fi

echo "Running integration tests..."
TEST_DATABASE_URL="$test_database_url" \
TEST_S3_ENDPOINT="$test_s3_endpoint" \
TEST_S3_REGION="$test_s3_region" \
TEST_S3_BUCKET="$test_s3_bucket" \
TEST_S3_ACCESS_KEY_ID="$test_s3_access_key_id" \
TEST_S3_SECRET_ACCESS_KEY="$test_s3_secret_access_key" \
TEST_TEMPORAL_ADDRESS="$test_temporal_address" \
TEST_TEMPORAL_NAMESPACE="$test_temporal_namespace" \
RUN_LARGE_REPLAY_DATASET="$run_large_replay_dataset" \
LARGE_REPLAY_EVENT_COUNT="$large_replay_event_count" \
LARGE_REPLAY_MAX_MISMATCH_ROWS="$large_replay_max_mismatch_rows" \
LARGE_REPLAY_MAX_TOTAL_ABS_DELTA_CENTS="$large_replay_max_total_abs_delta_cents" \
go test "${test_flags[@]}" ./internal/api/ ./internal/store/ ./internal/service/ ./migrations/...

echo "Integration tests passed."
