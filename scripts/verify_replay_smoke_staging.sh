#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq
require_cmd mktemp
require_cmd date

ALPHA_API_BASE_URL="${ALPHA_API_BASE_URL:-https://api-staging.sagarwaidande.org}"
OUTPUT_FILE="${OUTPUT_FILE:-}"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
RULE_KEY="${RULE_KEY:-replay_smoke_flat_${RUN_ID}}"
RULE_NAME="${RULE_NAME:-Replay Smoke Flat ${RUN_ID}}"
METER_KEY="${METER_KEY:-replay_smoke_meter_${RUN_ID}}"
METER_NAME="${METER_NAME:-Replay Smoke Meter ${RUN_ID}}"
CUSTOMER_ID="${CUSTOMER_ID:-cust_replay_smoke_${RUN_ID}}"
QUANTITY="${QUANTITY:-1}"
COMPUTED_AMOUNT_CENTS="${COMPUTED_AMOUNT_CENTS:-100}"
INITIAL_BILLED_AMOUNT_CENTS="${INITIAL_BILLED_AMOUNT_CENTS:-80}"
EXPECTED_REPLAY_DELTA_CENTS="$((COMPUTED_AMOUNT_CENTS - INITIAL_BILLED_AMOUNT_CENTS))"
EVENT_TIMESTAMP="${EVENT_TIMESTAMP:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
BILLED_TIMESTAMP="${BILLED_TIMESTAMP:-$EVENT_TIMESTAMP}"

USAGE_IDEMPOTENCY_KEY="${USAGE_IDEMPOTENCY_KEY:-replay-smoke-usage-${RUN_ID}}"
BILLED_IDEMPOTENCY_KEY="${BILLED_IDEMPOTENCY_KEY:-replay-smoke-billed-${RUN_ID}}"
REPLAY_IDEMPOTENCY_KEY="${REPLAY_IDEMPOTENCY_KEY:-replay-smoke-job-${RUN_ID}}"

POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-2}"
TIMEOUT_SEC="${TIMEOUT_SEC:-90}"

require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY

STATUS_CODE=""
RESPONSE_BODY=""

log() {
  printf '[info] %s\n' "$*" >&2
}

fail() {
  printf '[fail] %s\n' "$*" >&2
  exit 1
}

api_request() {
  local method="$1"
  local path="$2"
  local api_key="$3"
  local payload="${4:-}"
  local response

  if [[ -n "$payload" ]]; then
    response="$(curl -sS --connect-timeout 10 --max-time 60 -X "$method" \
      -H "X-API-Key: ${api_key}" \
      -H 'Content-Type: application/json' \
      -d "$payload" \
      --write-out $'\n%{http_code}' \
      "${ALPHA_API_BASE_URL}${path}")"
  else
    response="$(curl -sS --connect-timeout 10 --max-time 60 -X "$method" \
      -H "X-API-Key: ${api_key}" \
      --write-out $'\n%{http_code}' \
      "${ALPHA_API_BASE_URL}${path}")"
  fi

  STATUS_CODE="${response##*$'\n'}"
  RESPONSE_BODY="${response%$'\n'*}"
}

expect_status() {
  local expected="$1"
  if [[ "$STATUS_CODE" != "$expected" ]]; then
    printf '%s\n' "$RESPONSE_BODY" >&2
    fail "unexpected status ${STATUS_CODE}; expected ${expected}"
  fi
}

expect_one_of_status() {
  local expected_csv="$1"
  local expected
  IFS=',' read -r -a expected <<<"$expected_csv"
  for code in "${expected[@]}"; do
    if [[ "$STATUS_CODE" == "$code" ]]; then
      return 0
    fi
  done
  printf '%s\n' "$RESPONSE_BODY" >&2
  fail "unexpected status ${STATUS_CODE}; expected one of ${expected_csv}"
}


log "creating replay smoke rating rule ${RULE_KEY}"
api_request POST /v1/rating-rules "$ALPHA_WRITER_API_KEY" "$(jq -nc \
  --arg rule_key "$RULE_KEY" \
  --arg name "$RULE_NAME" \
  --argjson version 1 \
  --arg lifecycle_state active \
  --arg mode flat \
  --arg currency USD \
  --argjson flat_amount_cents "$COMPUTED_AMOUNT_CENTS" \
  '{rule_key:$rule_key,name:$name,version:$version,lifecycle_state:$lifecycle_state,mode:$mode,currency:$currency,flat_amount_cents:$flat_amount_cents}')"
expect_status 201
rule_json="$RESPONSE_BODY"
rule_id="$(jq -r '.id' <<<"$rule_json")"

log "creating replay smoke meter ${METER_KEY}"
api_request POST /v1/meters "$ALPHA_WRITER_API_KEY" "$(jq -nc \
  --arg key "$METER_KEY" \
  --arg name "$METER_NAME" \
  --arg unit call \
  --arg aggregation sum \
  --arg rating_rule_version_id "$rule_id" \
  '{key:$key,name:$name,unit:$unit,aggregation:$aggregation,rating_rule_version_id:$rating_rule_version_id}')"
expect_status 201
meter_json="$RESPONSE_BODY"
meter_id="$(jq -r '.id' <<<"$meter_json")"

log "creating replay smoke usage event for customer ${CUSTOMER_ID}"
api_request POST /v1/usage-events "$ALPHA_WRITER_API_KEY" "$(jq -nc \
  --arg customer_id "$CUSTOMER_ID" \
  --arg meter_id "$meter_id" \
  --arg idempotency_key "$USAGE_IDEMPOTENCY_KEY" \
  --arg timestamp "$EVENT_TIMESTAMP" \
  --argjson quantity "$QUANTITY" \
  '{customer_id:$customer_id,meter_id:$meter_id,quantity:$quantity,idempotency_key:$idempotency_key,timestamp:$timestamp}')"
expect_one_of_status 200,201
usage_json="$RESPONSE_BODY"
usage_id="$(jq -r '.id' <<<"$usage_json")"

log "creating intentionally under-billed entry (${INITIAL_BILLED_AMOUNT_CENTS} cents)"
api_request POST /v1/billed-entries "$ALPHA_WRITER_API_KEY" "$(jq -nc \
  --arg customer_id "$CUSTOMER_ID" \
  --arg meter_id "$meter_id" \
  --arg idempotency_key "$BILLED_IDEMPOTENCY_KEY" \
  --arg timestamp "$BILLED_TIMESTAMP" \
  --argjson amount_cents "$INITIAL_BILLED_AMOUNT_CENTS" \
  '{customer_id:$customer_id,meter_id:$meter_id,amount_cents:$amount_cents,idempotency_key:$idempotency_key,timestamp:$timestamp}')"
expect_one_of_status 200,201
billed_json="$RESPONSE_BODY"
billed_id="$(jq -r '.id' <<<"$billed_json")"

log "capturing reconciliation state before replay"
api_request GET "/v1/reconciliation-report?customer_id=${CUSTOMER_ID}&billed_source=api" "$ALPHA_READER_API_KEY"
expect_status 200
before_recon_json="$RESPONSE_BODY"
before_mismatch="$(jq -r '.mismatch_row_count' <<<"$before_recon_json")"
before_total_billed="$(jq -r '.total_billed_cents' <<<"$before_recon_json")"
before_total_delta="$(jq -r '.total_delta_cents' <<<"$before_recon_json")"
if [[ "$before_mismatch" != "1" ]]; then
  fail "expected one mismatch row before replay, got ${before_mismatch}"
fi
if [[ "$before_total_billed" != "$INITIAL_BILLED_AMOUNT_CENTS" ]]; then
  fail "expected pre-replay billed total ${INITIAL_BILLED_AMOUNT_CENTS}, got ${before_total_billed}"
fi
if [[ "$before_total_delta" != "$EXPECTED_REPLAY_DELTA_CENTS" ]]; then
  fail "expected pre-replay delta ${EXPECTED_REPLAY_DELTA_CENTS}, got ${before_total_delta}"
fi

log "creating replay job"
api_request POST /v1/replay-jobs "$ALPHA_WRITER_API_KEY" "$(jq -nc \
  --arg customer_id "$CUSTOMER_ID" \
  --arg meter_id "$meter_id" \
  --arg idempotency_key "$REPLAY_IDEMPOTENCY_KEY" \
  '{customer_id:$customer_id,meter_id:$meter_id,idempotency_key:$idempotency_key}')"
expect_one_of_status 200,201
replay_create_json="$RESPONSE_BODY"
replay_job_id="$(jq -r '.job.id' <<<"$replay_create_json")"

log "polling replay job ${replay_job_id}"
start_epoch="$(date +%s)"
replay_job_json=''
replay_job_status=''
while :; do
  api_request GET "/v1/replay-jobs/${replay_job_id}" "$ALPHA_READER_API_KEY"
  expect_status 200
  replay_job_json="$RESPONSE_BODY"
  replay_job_status="$(jq -r '.status' <<<"$replay_job_json")"
  if [[ "$replay_job_status" == "done" || "$replay_job_status" == "failed" ]]; then
    break
  fi
  now_epoch="$(date +%s)"
  if (( now_epoch - start_epoch >= TIMEOUT_SEC )); then
    fail "timed out waiting for replay job ${replay_job_id}; last status=${replay_job_status}"
  fi
  sleep "$POLL_INTERVAL_SEC"
done

if [[ "$replay_job_status" != "done" ]]; then
  printf '%s\n' "$replay_job_json" >&2
  fail "expected replay job ${replay_job_id} to finish with status=done, got ${replay_job_status}"
fi

log "capturing replay diagnostics"
api_request GET "/v1/replay-jobs/${replay_job_id}/events" "$ALPHA_READER_API_KEY"
expect_status 200
replay_diag_json="$RESPONSE_BODY"
if [[ "$(jq -r '.usage_events_count' <<<"$replay_diag_json")" != "1" ]]; then
  fail "expected replay diagnostics usage_events_count=1"
fi
if [[ "$(jq -r '.usage_quantity' <<<"$replay_diag_json")" != "$QUANTITY" ]]; then
  fail "expected replay diagnostics usage_quantity=${QUANTITY}"
fi
if [[ "$(jq -r '.billed_entries_count' <<<"$replay_diag_json")" != "2" ]]; then
  fail "expected replay diagnostics billed_entries_count=2"
fi
if [[ "$(jq -r '.billed_amount_cents' <<<"$replay_diag_json")" != "$COMPUTED_AMOUNT_CENTS" ]]; then
  fail "expected replay diagnostics billed_amount_cents=${COMPUTED_AMOUNT_CENTS}"
fi

log "capturing reconciliation state after replay"
api_request GET "/v1/reconciliation-report?customer_id=${CUSTOMER_ID}" "$ALPHA_READER_API_KEY"
expect_status 200
after_recon_json="$RESPONSE_BODY"
after_mismatch="$(jq -r '.mismatch_row_count' <<<"$after_recon_json")"
after_total_delta="$(jq -r '.total_delta_cents' <<<"$after_recon_json")"
after_total_billed="$(jq -r '.total_billed_cents' <<<"$after_recon_json")"
if [[ "$after_mismatch" != "0" ]]; then
  fail "expected mismatch_row_count=0 after replay, got ${after_mismatch}"
fi
if [[ "$after_total_delta" != "0" ]]; then
  fail "expected total_delta_cents=0 after replay, got ${after_total_delta}"
fi
if [[ "$after_total_billed" != "$COMPUTED_AMOUNT_CENTS" ]]; then
  fail "expected total_billed_cents=${COMPUTED_AMOUNT_CENTS} after replay, got ${after_total_billed}"
fi

log "listing replay adjustment billed entry"
api_request GET "/v1/billed-entries?customer_id=${CUSTOMER_ID}&meter_id=${meter_id}&billed_source=replay_adjustment&billed_replay_job_id=${replay_job_id}&limit=10" "$ALPHA_READER_API_KEY"
expect_status 200
replay_adjustment_list_json="$RESPONSE_BODY"
replay_adjustment_count="$(jq -r '.items | length' <<<"$replay_adjustment_list_json")"
if [[ "$replay_adjustment_count" != "1" ]]; then
  fail "expected exactly one replay_adjustment billed entry, got ${replay_adjustment_count}"
fi
replay_adjustment_json="$(jq -c '.items[0]' <<<"$replay_adjustment_list_json")"
replay_adjustment_amount="$(jq -r '.amount_cents' <<<"$replay_adjustment_json")"
if [[ "$replay_adjustment_amount" != "$EXPECTED_REPLAY_DELTA_CENTS" ]]; then
  fail "expected replay adjustment amount ${EXPECTED_REPLAY_DELTA_CENTS}, got ${replay_adjustment_amount}"
fi

summary_json="$(jq -n \
  --arg alpha_api_base_url "$ALPHA_API_BASE_URL" \
  --arg run_id "$RUN_ID" \
  --arg customer_id "$CUSTOMER_ID" \
  --arg expected_rule_key "$RULE_KEY" \
  --arg expected_meter_key "$METER_KEY" \
  --argjson expected_quantity "$QUANTITY" \
  --argjson expected_computed_amount_cents "$COMPUTED_AMOUNT_CENTS" \
  --argjson expected_initial_billed_amount_cents "$INITIAL_BILLED_AMOUNT_CENTS" \
  --argjson expected_replay_delta_cents "$EXPECTED_REPLAY_DELTA_CENTS" \
  --argjson rule "$(jq -c . <<<"$rule_json")" \
  --argjson meter "$(jq -c . <<<"$meter_json")" \
  --argjson usage_event "$(jq -c . <<<"$usage_json")" \
  --argjson initial_billed_entry "$(jq -c . <<<"$billed_json")" \
  --argjson before_reconciliation "$(jq -c . <<<"$before_recon_json")" \
  --argjson replay_job_create "$(jq -c . <<<"$replay_create_json")" \
  --argjson replay_job "$(jq -c . <<<"$replay_job_json")" \
  --argjson replay_diagnostics "$(jq -c . <<<"$replay_diag_json")" \
  --argjson replay_adjustment_entry "$replay_adjustment_json" \
  --argjson after_reconciliation "$(jq -c . <<<"$after_recon_json")" \
  '{
    alpha_api_base_url: $alpha_api_base_url,
    run_id: $run_id,
    fixture: {
      customer_id: $customer_id,
      expected_rule_key: $expected_rule_key,
      expected_meter_key: $expected_meter_key,
      expected_quantity: $expected_quantity,
      expected_computed_amount_cents: $expected_computed_amount_cents,
      expected_initial_billed_amount_cents: $expected_initial_billed_amount_cents,
      expected_replay_delta_cents: $expected_replay_delta_cents,
      rule: $rule,
      meter: $meter,
      usage_event: $usage_event,
      initial_billed_entry: $initial_billed_entry
    },
    before_reconciliation: $before_reconciliation,
    replay_job_create: $replay_job_create,
    replay_job: $replay_job,
    replay_diagnostics: $replay_diagnostics,
    replay_adjustment_entry: $replay_adjustment_entry,
    after_reconciliation: $after_reconciliation,
    live_browser_smoke: {
      playwright_live_replay_job_id: $replay_job.id,
      playwright_live_replay_customer_id: $customer_id,
      playwright_live_replay_meter_id: $meter.id,
      note: "Use these env vars with make web-e2e-live to inspect replay diagnostics or queue a fresh replay job from the UI."
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
