#!/usr/bin/env bash
set -euo pipefail

required_cmds=(bash jq mktemp)
for cmd in "${required_cmds[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
done

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
}

run_with_output() {
  local outfile="$1"
  shift
  "$@" | tee "$outfile"
}

REQUIRE_RUNTIME_VERIFY="${REQUIRE_RUNTIME_VERIFY:-1}"
REQUIRE_PAYMENT_E2E="${REQUIRE_PAYMENT_E2E:-1}"
PREPARE_PAYMENT_SMOKE_FIXTURES="${PREPARE_PAYMENT_SMOKE_FIXTURES:-1}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

if [[ "$REQUIRE_RUNTIME_VERIFY" != "0" && "$REQUIRE_RUNTIME_VERIFY" != "1" ]]; then
  echo "REQUIRE_RUNTIME_VERIFY must be 0 or 1" >&2
  exit 1
fi
if [[ "$REQUIRE_PAYMENT_E2E" != "0" && "$REQUIRE_PAYMENT_E2E" != "1" ]]; then
  echo "REQUIRE_PAYMENT_E2E must be 0 or 1" >&2
  exit 1
fi
if [[ "$PREPARE_PAYMENT_SMOKE_FIXTURES" != "0" && "$PREPARE_PAYMENT_SMOKE_FIXTURES" != "1" ]]; then
  echo "PREPARE_PAYMENT_SMOKE_FIXTURES must be 0 or 1" >&2
  exit 1
fi

require_env ALPHA_API_BASE_URL
require_env ALPHA_READER_API_KEY

RUNTIME_INVOICE_ID="${RUNTIME_INVOICE_ID:-${SUCCESS_INVOICE_ID:-${FAILURE_INVOICE_ID:-}}}"
SUCCESS_INVOICE_ID="${SUCCESS_INVOICE_ID:-}"
FAILURE_INVOICE_ID="${FAILURE_INVOICE_ID:-}"

if [[ "$REQUIRE_PAYMENT_E2E" == "1" ]]; then
  require_env ALPHA_WRITER_API_KEY
  require_env LAGO_API_URL
  require_env LAGO_API_KEY
  if [[ "$PREPARE_PAYMENT_SMOKE_FIXTURES" == "0" && ( -z "$SUCCESS_INVOICE_ID" || -z "$FAILURE_INVOICE_ID" ) ]]; then
    echo "SUCCESS_INVOICE_ID and FAILURE_INVOICE_ID are required when REQUIRE_PAYMENT_E2E=1" >&2
    exit 1
  fi
fi

runtime_json_file="$(mktemp)"
success_json_file="$(mktemp)"
failure_json_file="$(mktemp)"
payment_smoke_json_file="$(mktemp)"
trap 'rm -f "$runtime_json_file" "$success_json_file" "$failure_json_file" "$payment_smoke_json_file"' EXIT

runtime_enabled=false
payment_enabled=false

if [[ "$REQUIRE_RUNTIME_VERIFY" == "1" ]]; then
  runtime_enabled=true
  echo "[info] running staging runtime verification"
  INVOICE_ID="$RUNTIME_INVOICE_ID" \
  OUTPUT_FILE="$runtime_json_file" \
  bash ./scripts/verify_staging_runtime.sh
fi

if [[ "$REQUIRE_PAYMENT_E2E" == "1" ]]; then
  payment_enabled=true

  if [[ -n "$SUCCESS_INVOICE_ID" && -n "$FAILURE_INVOICE_ID" ]]; then
    echo "[info] running success-path payment e2e"
    EXPECTED_FINAL_STATUS="succeeded" \
    INVOICE_ID="$SUCCESS_INVOICE_ID" \
    OUTPUT_FILE="$success_json_file" \
    TIMEOUT_SEC="${TIMEOUT_SEC:-600}" \
    POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}" \
    bash ./scripts/test_real_payment_e2e.sh

    echo "[info] running failure-path payment e2e"
    EXPECTED_FINAL_STATUS="failed" \
    INVOICE_ID="$FAILURE_INVOICE_ID" \
    OUTPUT_FILE="$failure_json_file" \
    TIMEOUT_SEC="${TIMEOUT_SEC:-600}" \
    POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}" \
    bash ./scripts/test_real_payment_e2e.sh
  else
    echo "[info] preparing clean payment smoke fixtures and running payment e2e"
    OUTPUT_FILE="$payment_smoke_json_file" \
    TIMEOUT_SEC="${TIMEOUT_SEC:-600}" \
    POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}" \
    bash ./scripts/run_clean_staging_payment_smoke.sh
  fi
fi

summary_json="$(
jq -n \
  --arg alpha_api_base_url "$ALPHA_API_BASE_URL" \
  --arg runtime_invoice_id "$RUNTIME_INVOICE_ID" \
  --arg success_invoice_id "$SUCCESS_INVOICE_ID" \
  --arg failure_invoice_id "$FAILURE_INVOICE_ID" \
  --argjson runtime_enabled "$runtime_enabled" \
  --argjson payment_enabled "$payment_enabled" \
  --slurpfile runtime "$runtime_json_file" \
  --slurpfile success "$success_json_file" \
  --slurpfile failure "$failure_json_file" \
  --slurpfile payment_smoke "$payment_smoke_json_file" \
  '{
    alpha_api_base_url: $alpha_api_base_url,
    runtime_verify_enabled: $runtime_enabled,
    payment_e2e_enabled: $payment_enabled,
    runtime_invoice_id: (if $runtime_invoice_id == "" then null else $runtime_invoice_id end),
    success_invoice_id: (if $success_invoice_id == "" then null else $success_invoice_id end),
    failure_invoice_id: (if $failure_invoice_id == "" then null else $failure_invoice_id end),
    runtime: (if $runtime_enabled then ($runtime[0] // null) else null end),
    payment_e2e: (if $payment_enabled then {
      fixture_source: (
        if ($payment_smoke[0] // null) != null
        then ($payment_smoke[0].fixture_source // null)
        else "explicit_invoice_ids"
        end
      ),
      prepared_fixtures: ($payment_smoke[0] // null),
      success: (
        if ($payment_smoke[0] // null) != null
        then ($payment_smoke[0].payment_e2e.success // null)
        else ($success[0] // null)
        end
      ),
      failure: (
        if ($payment_smoke[0] // null) != null
        then ($payment_smoke[0].payment_e2e.failure // null)
        else ($failure[0] // null)
        end
      )
    } else null end)
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
