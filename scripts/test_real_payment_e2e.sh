#!/usr/bin/env bash
set -euo pipefail

required_cmds=(curl jq)
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

trim_trailing_slash() {
  local v="$1"
  while [[ "$v" == */ ]]; do
    v="${v%/}"
  done
  printf "%s" "$v"
}

http_call() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  shift 3
  local -a headers=("$@")
  local -a args=(-sS -X "$method" "$url" -H "Accept: application/json")
  local h
  for h in "${headers[@]}"; do
    args+=(-H "$h")
  done
  if [[ -n "$body" ]]; then
    args+=(-H "Content-Type: application/json" --data "$body")
  fi

  local out
  out="$(curl "${args[@]}" -w $'\n%{http_code}')"
  HTTP_CODE="${out##*$'\n'}"
  HTTP_BODY="${out%$'\n'*}"
}

require_env ALPHA_API_BASE_URL
require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY
require_env LAGO_API_URL
require_env LAGO_API_KEY
require_env INVOICE_ID

EXPECTED_FINAL_STATUS="${EXPECTED_FINAL_STATUS:-succeeded}"
TIMEOUT_SEC="${TIMEOUT_SEC:-600}"
POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}"
REQUIRE_WEBHOOK_TYPES="${REQUIRE_WEBHOOK_TYPES:-invoice.payment_status_updated,invoice.payment_failure,invoice.payment_overdue}"
RETRY_PAYMENT_BODY="${RETRY_PAYMENT_BODY:-{}}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

if [[ "$EXPECTED_FINAL_STATUS" != "succeeded" && "$EXPECTED_FINAL_STATUS" != "failed" ]]; then
  echo "EXPECTED_FINAL_STATUS must be one of: succeeded, failed" >&2
  exit 1
fi
if ! [[ "$TIMEOUT_SEC" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT_SEC" -le 0 ]]; then
  echo "TIMEOUT_SEC must be a positive integer" >&2
  exit 1
fi
if ! [[ "$POLL_INTERVAL_SEC" =~ ^[0-9]+$ ]] || [[ "$POLL_INTERVAL_SEC" -le 0 ]]; then
  echo "POLL_INTERVAL_SEC must be a positive integer" >&2
  exit 1
fi
if ! jq -e . >/dev/null 2>&1 <<<"$RETRY_PAYMENT_BODY"; then
  echo "RETRY_PAYMENT_BODY must be valid JSON" >&2
  exit 1
fi

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"
INVOICE_ID="$(echo "$INVOICE_ID" | xargs)"

if [[ -z "$INVOICE_ID" ]]; then
  echo "INVOICE_ID must not be empty" >&2
  exit 1
fi

echo "[info] validating invoice in Lago"
http_call "GET" "$LAGO_API_URL/api/v1/invoices/$INVOICE_ID" "" "Authorization: Bearer $LAGO_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] lago invoice lookup failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

lago_invoice_status="$(jq -r '.invoice.status // ""' <<<"$HTTP_BODY")"
lago_payment_status_initial="$(jq -r '.invoice.payment_status // ""' <<<"$HTTP_BODY")"
if [[ "$lago_invoice_status" != "finalized" ]]; then
  echo "[fail] invoice must be finalized for payment retry (got status=$lago_invoice_status)" >&2
  exit 1
fi
echo "[pass] lago invoice is finalized with initial payment_status=$lago_payment_status_initial"

echo "[info] triggering payment retry via alpha control plane"
http_call \
  "POST" \
  "$ALPHA_API_BASE_URL/v1/invoices/$INVOICE_ID/retry-payment" \
  "$RETRY_PAYMENT_BODY" \
  "X-API-Key: $ALPHA_WRITER_API_KEY"
if [[ "$HTTP_CODE" != "200" && "$HTTP_CODE" != "202" ]]; then
  echo "[fail] retry-payment failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi
echo "[pass] retry-payment accepted: status=$HTTP_CODE"

deadline_epoch="$(( $(date +%s) + TIMEOUT_SEC ))"
lago_payment_status_final=""

echo "[info] polling Lago invoice payment status (expected=$EXPECTED_FINAL_STATUS)"
while true; do
  http_call "GET" "$LAGO_API_URL/api/v1/invoices/$INVOICE_ID" "" "Authorization: Bearer $LAGO_API_KEY"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[warn] lago poll returned status=$HTTP_CODE body=$HTTP_BODY"
  else
    lago_payment_status_final="$(jq -r '.invoice.payment_status // ""' <<<"$HTTP_BODY")"
    if [[ "$lago_payment_status_final" == "$EXPECTED_FINAL_STATUS" ]]; then
      echo "[pass] Lago payment reached expected terminal status=$lago_payment_status_final"
      break
    fi
    if [[ "$lago_payment_status_final" == "succeeded" || "$lago_payment_status_final" == "failed" ]]; then
      echo "[fail] Lago reached unexpected terminal status=$lago_payment_status_final (expected=$EXPECTED_FINAL_STATUS)" >&2
      exit 1
    fi
  fi

  if [[ "$(date +%s)" -ge "$deadline_epoch" ]]; then
    echo "[fail] timeout waiting for Lago terminal status; last_status=$lago_payment_status_final expected=$EXPECTED_FINAL_STATUS" >&2
    exit 1
  fi
  sleep "$POLL_INTERVAL_SEC"
done

alpha_status=""
alpha_last_event_at=""
alpha_last_payment_error=""
alpha_payment_overdue=""
alpha_last_webhook_key=""
echo "[info] polling alpha payment projection status"
while true; do
  http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID" "" "X-API-Key: $ALPHA_READER_API_KEY"
  if [[ "$HTTP_CODE" == "200" ]]; then
    alpha_status="$(jq -r '.payment_status // ""' <<<"$HTTP_BODY")"
    alpha_last_event_at="$(jq -r '.last_event_at // ""' <<<"$HTTP_BODY")"
    alpha_last_payment_error="$(jq -r '.last_payment_error // ""' <<<"$HTTP_BODY")"
    alpha_payment_overdue="$(jq -r 'if .payment_overdue == null then "" else (.payment_overdue|tostring) end' <<<"$HTTP_BODY")"
    alpha_last_webhook_key="$(jq -r '.last_webhook_key // ""' <<<"$HTTP_BODY")"
    if [[ "$alpha_status" == "$EXPECTED_FINAL_STATUS" && -n "$alpha_last_event_at" ]]; then
      echo "[pass] alpha projection converged: payment_status=$alpha_status last_event_at=$alpha_last_event_at"
      break
    fi
  else
    echo "[warn] alpha projection poll returned status=$HTTP_CODE body=$HTTP_BODY"
  fi

  if [[ "$(date +%s)" -ge "$deadline_epoch" ]]; then
    echo "[fail] timeout waiting for alpha projection convergence; last_status=$alpha_status expected=$EXPECTED_FINAL_STATUS" >&2
    exit 1
  fi
  sleep "$POLL_INTERVAL_SEC"
done

echo "[info] validating alpha webhook timeline"
http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID/events?limit=200" "" "X-API-Key: $ALPHA_READER_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] failed to fetch alpha events: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

event_count="$(jq -r '.items | length' <<<"$HTTP_BODY")"
if ! [[ "$event_count" =~ ^[0-9]+$ ]] || [[ "$event_count" -le 0 ]]; then
  echo "[fail] expected at least one payment webhook event for invoice=$INVOICE_ID" >&2
  exit 1
fi

matched_required_type="false"
IFS=',' read -r -a required_types <<<"$REQUIRE_WEBHOOK_TYPES"
for typ in "${required_types[@]}"; do
  typ="$(echo "$typ" | xargs)"
  [[ -z "$typ" ]] && continue
  if jq -e --arg t "$typ" '.items[]? | select(.webhook_type == $t)' >/dev/null <<<"$HTTP_BODY"; then
    matched_required_type="true"
    break
  fi
done
if [[ "$matched_required_type" != "true" ]]; then
  echo "[fail] expected at least one webhook_type in {$REQUIRE_WEBHOOK_TYPES}; got=$(jq -r '.items[]?.webhook_type' <<<"$HTTP_BODY" | tr '\n' ',' )" >&2
  exit 1
fi

echo "[info] validating alpha lifecycle summary"
http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID/lifecycle" "" "X-API-Key: $ALPHA_READER_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] failed to fetch alpha lifecycle summary: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

lifecycle_payment_status="$(jq -r '.payment_status // ""' <<<"$HTTP_BODY")"
lifecycle_recommended_action="$(jq -r '.recommended_action // ""' <<<"$HTTP_BODY")"
lifecycle_requires_action="$(jq -r '.requires_action // false' <<<"$HTTP_BODY")"
lifecycle_retry_recommended="$(jq -r '.retry_recommended // false' <<<"$HTTP_BODY")"
lifecycle_failure_event_count="$(jq -r '.failure_event_count // 0' <<<"$HTTP_BODY")"
lifecycle_overdue_signal_count="$(jq -r '.overdue_signal_count // 0' <<<"$HTTP_BODY")"
lifecycle_events_analyzed="$(jq -r '.events_analyzed // 0' <<<"$HTTP_BODY")"
lifecycle_last_failure_at="$(jq -r '.last_failure_at // ""' <<<"$HTTP_BODY")"

if [[ "$lifecycle_payment_status" != "$EXPECTED_FINAL_STATUS" ]]; then
  echo "[fail] lifecycle payment_status mismatch: got=$lifecycle_payment_status expected=$EXPECTED_FINAL_STATUS" >&2
  exit 1
fi

case "$EXPECTED_FINAL_STATUS" in
  succeeded)
    if [[ "$lifecycle_recommended_action" != "none" || "$lifecycle_requires_action" != "false" || "$lifecycle_retry_recommended" != "false" ]]; then
      echo "[fail] succeeded lifecycle expectation mismatch: recommended_action=$lifecycle_recommended_action requires_action=$lifecycle_requires_action retry_recommended=$lifecycle_retry_recommended" >&2
      exit 1
    fi
    ;;
  failed)
    if [[ "$lifecycle_recommended_action" != "retry_payment" || "$lifecycle_requires_action" != "true" || "$lifecycle_retry_recommended" != "true" ]]; then
      echo "[fail] failed lifecycle expectation mismatch: recommended_action=$lifecycle_recommended_action requires_action=$lifecycle_requires_action retry_recommended=$lifecycle_retry_recommended" >&2
      exit 1
    fi
    if ! [[ "$lifecycle_failure_event_count" =~ ^[0-9]+$ ]] || [[ "$lifecycle_failure_event_count" -lt 1 ]]; then
      echo "[fail] expected failure lifecycle to include at least one failure event, got=$lifecycle_failure_event_count" >&2
      exit 1
    fi
    ;;
esac

echo "[pass] alpha lifecycle summary matches expected outcome"

echo "[pass] real payment e2e completed"
result_json="$(
jq -n \
  --arg invoice_id "$INVOICE_ID" \
  --arg expected_status "$EXPECTED_FINAL_STATUS" \
  --arg lago_final_status "$lago_payment_status_final" \
  --arg alpha_final_status "$alpha_status" \
  --arg alpha_last_event_at "$alpha_last_event_at" \
  --arg alpha_last_payment_error "$alpha_last_payment_error" \
  --arg alpha_payment_overdue "$alpha_payment_overdue" \
  --arg alpha_last_webhook_key "$alpha_last_webhook_key" \
  --arg lifecycle_recommended_action "$lifecycle_recommended_action" \
  --arg lifecycle_requires_action "$lifecycle_requires_action" \
  --arg lifecycle_retry_recommended "$lifecycle_retry_recommended" \
  --arg lifecycle_last_failure_at "$lifecycle_last_failure_at" \
  --argjson lifecycle_failure_event_count "$lifecycle_failure_event_count" \
  --argjson lifecycle_overdue_signal_count "$lifecycle_overdue_signal_count" \
  --argjson lifecycle_events_analyzed "$lifecycle_events_analyzed" \
  --argjson event_count "$event_count" \
  '{
    invoice_id: $invoice_id,
    expected_status: $expected_status,
    lago_final_status: $lago_final_status,
    alpha_final_status: $alpha_final_status,
    alpha_last_event_at: $alpha_last_event_at,
    alpha_last_payment_error: $alpha_last_payment_error,
    alpha_payment_overdue: $alpha_payment_overdue,
    alpha_last_webhook_key: $alpha_last_webhook_key,
    lifecycle: {
      recommended_action: $lifecycle_recommended_action,
      requires_action: ($lifecycle_requires_action == "true"),
      retry_recommended: ($lifecycle_retry_recommended == "true"),
      failure_event_count: $lifecycle_failure_event_count,
      overdue_signal_count: $lifecycle_overdue_signal_count,
      events_analyzed: $lifecycle_events_analyzed,
      last_failure_at: $lifecycle_last_failure_at
    },
    event_count: $event_count
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$result_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$result_json"
