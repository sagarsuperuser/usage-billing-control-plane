#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
compose_file="${COMPOSE_FILE:-docker-compose.postgres.yml}"
test_database_url="${TEST_DATABASE_URL:-postgres://postgres:postgres@localhost:5432/lago_alpha_test?sslmode=disable}"
cleanup="${CLEANUP_ON_EXIT:-1}"

cleanup_fn() {
  if [[ "$cleanup" == "1" ]]; then
    docker compose -f "$repo_root/$compose_file" down >/dev/null 2>&1 || true
  fi
}
trap cleanup_fn EXIT

cd "$repo_root"

docker compose -f "$compose_file" up -d
"$repo_root/scripts/wait_for_postgres.sh" "$compose_file" postgres 90

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
TEST_DATABASE_URL="$test_database_url" go test ./internal/api -run TestEndToEndPreviewReplayReconciliation -v
TEST_DATABASE_URL="$test_database_url" go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v
