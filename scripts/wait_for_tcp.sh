#!/usr/bin/env bash
set -euo pipefail

hostport="${1:-}"
timeout_sec="${2:-60}"

if [[ -z "$hostport" ]]; then
  echo "usage: $0 <host:port> [timeout_sec]" >&2
  exit 1
fi

host="${hostport%:*}"
port="${hostport##*:}"
if [[ -z "$host" || -z "$port" || "$host" == "$hostport" ]]; then
  echo "invalid host:port '$hostport'" >&2
  exit 1
fi

start_ts="$(date +%s)"
while true; do
  if (echo >"/dev/tcp/${host}/${port}") >/dev/null 2>&1; then
    echo "tcp endpoint is ready: ${hostport}"
    exit 0
  fi

  now_ts="$(date +%s)"
  elapsed="$((now_ts - start_ts))"
  if (( elapsed >= timeout_sec )); then
    echo "timeout waiting for tcp endpoint: ${hostport}" >&2
    exit 1
  fi
  sleep 2
done
