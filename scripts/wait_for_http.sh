#!/usr/bin/env bash
set -euo pipefail

url="${1:-}"
timeout_sec="${2:-60}"

if [[ -z "$url" ]]; then
  echo "usage: $0 <url> [timeout_sec]" >&2
  exit 1
fi

start_ts="$(date +%s)"
while true; do
  if curl -fsS "$url" >/dev/null 2>&1; then
    echo "http endpoint is ready: $url"
    exit 0
  fi

  now_ts="$(date +%s)"
  elapsed="$((now_ts - start_ts))"
  if (( elapsed >= timeout_sec )); then
    echo "timeout waiting for http endpoint: $url" >&2
    exit 1
  fi
  sleep 2
done
