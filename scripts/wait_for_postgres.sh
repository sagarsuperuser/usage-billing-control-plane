#!/usr/bin/env bash
set -euo pipefail

compose_file="${1:-docker-compose.postgres.yml}"
service="${2:-postgres}"
timeout_seconds="${3:-90}"

container_id="$(docker compose -f "$compose_file" ps -q "$service")"
if [[ -z "$container_id" ]]; then
  echo "service '$service' is not running for compose file '$compose_file'" >&2
  exit 1
fi

deadline=$((SECONDS + timeout_seconds))

while (( SECONDS < deadline )); do
  status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id" 2>/dev/null || true)"

  if [[ "$status" == "healthy" || "$status" == "running" ]]; then
    echo "postgres service '$service' is ready (status=$status)"
    exit 0
  fi

  sleep 2
  container_id="$(docker compose -f "$compose_file" ps -q "$service")"
  if [[ -z "$container_id" ]]; then
    echo "service '$service' stopped while waiting for readiness" >&2
    exit 1
  fi

done

echo "timed out waiting for service '$service' readiness after ${timeout_seconds}s" >&2
exit 1
