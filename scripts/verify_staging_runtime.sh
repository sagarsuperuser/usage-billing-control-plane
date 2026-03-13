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
  local response_headers_file
  response_headers_file="$(mktemp)"
  local -a args=(-sS -X "$method" "$url" -D "$response_headers_file" -H "Accept: application/json")
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
  HTTP_HEADERS="$(tr -d '\r' <"$response_headers_file")"
  rm -f "$response_headers_file"
}

require_env ALPHA_API_BASE_URL
require_env ALPHA_READER_API_KEY

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
INVOICE_ID="$(echo "${INVOICE_ID:-}" | xargs)"
BAD_KEY="${BAD_KEY:-invalid_key}"
RATE_LIMIT_PROBE_ATTEMPTS="${RATE_LIMIT_PROBE_ATTEMPTS:-30}"
RATE_LIMIT_PROBE_PATH="${RATE_LIMIT_PROBE_PATH:-/v1/meters}"
REQUIRE_LIFECYCLE="${REQUIRE_LIFECYCLE:-1}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

if ! [[ "$RATE_LIMIT_PROBE_ATTEMPTS" =~ ^[0-9]+$ ]] || [[ "$RATE_LIMIT_PROBE_ATTEMPTS" -le 0 ]]; then
  echo "RATE_LIMIT_PROBE_ATTEMPTS must be a positive integer" >&2
  exit 1
fi
if [[ "$REQUIRE_LIFECYCLE" != "0" && "$REQUIRE_LIFECYCLE" != "1" ]]; then
  echo "REQUIRE_LIFECYCLE must be 0 or 1" >&2
  exit 1
fi

echo "[info] checking health endpoint"
http_call "GET" "$ALPHA_API_BASE_URL/health" "" "X-API-Key: $ALPHA_READER_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] health check failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi
echo "[pass] health endpoint returned 200"
health_status="$HTTP_CODE"

echo "[info] checking invoice payment statuses list"
http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses?limit=1" "" "X-API-Key: $ALPHA_READER_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] invoice status list failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi
if ! jq -e '.items' >/dev/null 2>&1 <<<"$HTTP_BODY"; then
  echo "[fail] invoice status list payload missing items array" >&2
  exit 1
fi
echo "[pass] invoice payment status list reachable"
invoice_list_count="$(jq -r '.items | length' <<<"$HTTP_BODY")"

http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/summary" "" "X-API-Key: $ALPHA_READER_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] invoice summary failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi
invoice_summary_attention_required="$(jq -r '.attention_required_count // 0' <<<"$HTTP_BODY")"
echo "[pass] invoice payment summary reachable"

invoice_payment_status=""
invoice_last_event_at=""
invoice_last_payment_error=""
invoice_events_count=""
lifecycle_recommended_action=""
lifecycle_requires_action=""
lifecycle_failure_event_count=""
lifecycle_events_analyzed=""

if [[ -n "$INVOICE_ID" ]]; then
  echo "[info] checking invoice payment status invoice_id=$INVOICE_ID"
  http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID" "" "X-API-Key: $ALPHA_READER_API_KEY"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[fail] invoice payment status lookup failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
    exit 1
  fi
  if ! jq -e '.invoice_id and .payment_status and .last_event_at' >/dev/null 2>&1 <<<"$HTTP_BODY"; then
    echo "[fail] invoice payment status payload missing required fields (invoice_id/payment_status/last_event_at)" >&2
    exit 1
  fi
  echo "[pass] invoice payment status payload is valid"
  invoice_payment_status="$(jq -r '.payment_status // ""' <<<"$HTTP_BODY")"
  invoice_last_event_at="$(jq -r '.last_event_at // ""' <<<"$HTTP_BODY")"
  invoice_last_payment_error="$(jq -r '.last_payment_error // ""' <<<"$HTTP_BODY")"

  echo "[info] checking invoice timeline events invoice_id=$INVOICE_ID"
  http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID/events?limit=10&order=desc" "" "X-API-Key: $ALPHA_READER_API_KEY"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[fail] invoice timeline fetch failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
    exit 1
  fi
  if ! jq -e '.items' >/dev/null 2>&1 <<<"$HTTP_BODY"; then
    echo "[fail] invoice timeline payload missing items array" >&2
    exit 1
  fi
  echo "[pass] invoice timeline events endpoint is valid"
  invoice_events_count="$(jq -r '.items | length' <<<"$HTTP_BODY")"

  if [[ "$REQUIRE_LIFECYCLE" == "1" ]]; then
    echo "[info] checking lifecycle summary invoice_id=$INVOICE_ID"
    http_call "GET" "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$INVOICE_ID/lifecycle" "" "X-API-Key: $ALPHA_READER_API_KEY"
    if [[ "$HTTP_CODE" != "200" ]]; then
      echo "[fail] lifecycle endpoint failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
      exit 1
    fi
    if ! jq -e '.invoice_id and .recommended_action and (.failure_event_count|type=="number") and (.events_analyzed|type=="number")' >/dev/null 2>&1 <<<"$HTTP_BODY"; then
      echo "[fail] lifecycle payload missing required fields" >&2
      exit 1
    fi
    echo "[pass] lifecycle endpoint payload is valid"
    lifecycle_recommended_action="$(jq -r '.recommended_action // ""' <<<"$HTTP_BODY")"
    lifecycle_requires_action="$(jq -r '.requires_action // false' <<<"$HTTP_BODY")"
    lifecycle_failure_event_count="$(jq -r '.failure_event_count // 0' <<<"$HTTP_BODY")"
    lifecycle_events_analyzed="$(jq -r '.events_analyzed // 0' <<<"$HTTP_BODY")"
  fi
else
  echo "[warn] INVOICE_ID is empty; skipping invoice-specific checks (status/events/lifecycle)"
fi

echo "[info] probing runtime rate limiting on $RATE_LIMIT_PROBE_PATH"
rate_limited="0"
saw_rate_limit_headers="0"
for i in $(seq 1 "$RATE_LIMIT_PROBE_ATTEMPTS"); do
  http_call "GET" "$ALPHA_API_BASE_URL$RATE_LIMIT_PROBE_PATH" "" "X-API-Key: $BAD_KEY"
  if [[ "$HTTP_CODE" == "429" ]]; then
    rate_limited="1"
    if grep -qi "^Retry-After:" <<<"$HTTP_HEADERS" &&
      grep -qi "^X-RateLimit-Limit:" <<<"$HTTP_HEADERS" &&
      grep -qi "^X-RateLimit-Remaining:" <<<"$HTTP_HEADERS" &&
      grep -qi "^X-RateLimit-Reset:" <<<"$HTTP_HEADERS"; then
      saw_rate_limit_headers="1"
      break
    fi
  fi
done

if [[ "$rate_limited" != "1" ]]; then
  echo "[fail] rate limit probe did not observe HTTP 429 after $RATE_LIMIT_PROBE_ATTEMPTS attempts" >&2
  exit 1
fi
if [[ "$saw_rate_limit_headers" != "1" ]]; then
  echo "[fail] observed HTTP 429 but required rate-limit headers were incomplete" >&2
  echo "$HTTP_HEADERS" >&2
  exit 1
fi

echo "[pass] runtime rate limiting is active with Retry-After and X-RateLimit headers"
echo "[pass] staging runtime verification completed"

result_json="$(
jq -n \
  --arg alpha_api_base_url "$ALPHA_API_BASE_URL" \
  --arg invoice_id "$INVOICE_ID" \
  --argjson health_status "$health_status" \
  --argjson invoice_list_count "$invoice_list_count" \
  --argjson invoice_summary_attention_required "$invoice_summary_attention_required" \
  --arg invoice_payment_status "$invoice_payment_status" \
  --arg invoice_last_event_at "$invoice_last_event_at" \
  --arg invoice_last_payment_error "$invoice_last_payment_error" \
  --arg invoice_events_count "$invoice_events_count" \
  --arg require_lifecycle "$REQUIRE_LIFECYCLE" \
  --arg lifecycle_recommended_action "$lifecycle_recommended_action" \
  --arg lifecycle_requires_action "$lifecycle_requires_action" \
  --arg lifecycle_failure_event_count "$lifecycle_failure_event_count" \
  --arg lifecycle_events_analyzed "$lifecycle_events_analyzed" \
  --arg rate_limit_probe_path "$RATE_LIMIT_PROBE_PATH" \
  --argjson rate_limit_probe_attempts "$RATE_LIMIT_PROBE_ATTEMPTS" \
  '{
    alpha_api_base_url: $alpha_api_base_url,
    invoice_id: $invoice_id,
    health_status: $health_status,
    invoice_payment_status_list_count: $invoice_list_count,
    summary_attention_required_count: $invoice_summary_attention_required,
    rate_limit: {
      probe_path: $rate_limit_probe_path,
      probe_attempts: $rate_limit_probe_attempts,
      throttled: true,
      required_headers_present: true
    },
    invoice: if $invoice_id == "" then null else {
      payment_status: $invoice_payment_status,
      last_event_at: $invoice_last_event_at,
      last_payment_error: $invoice_last_payment_error,
      events_count: (if $invoice_events_count == "" then null else ($invoice_events_count | tonumber) end),
      lifecycle: if $require_lifecycle == "1" then {
        recommended_action: $lifecycle_recommended_action,
        requires_action: ($lifecycle_requires_action == "true"),
        failure_event_count: (if $lifecycle_failure_event_count == "" then 0 else ($lifecycle_failure_event_count | tonumber) end),
        events_analyzed: (if $lifecycle_events_analyzed == "" then 0 else ($lifecycle_events_analyzed | tonumber) end)
      } else null end
    } end
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$result_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$result_json"
