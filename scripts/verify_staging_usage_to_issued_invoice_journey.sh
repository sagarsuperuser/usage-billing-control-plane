#!/usr/bin/env bash
set -euo pipefail

required_cmds=(bash curl jq mktemp)
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
  local value="$1"
  while [[ "$value" == */ ]]; do
    value="${value%/}"
  done
  printf '%s' "$value"
}

urlencode() {
  jq -nr --arg v "$1" '$v|@uri'
}

date_offset_utc() {
  local days="$1"
  if date -u -v-"${days}"d '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
    date -u -v-"${days}"d '+%Y-%m-%dT%H:%M:%SZ'
  else
    date -u -d "${days} days ago" '+%Y-%m-%dT%H:%M:%SZ'
  fi
}

http_call() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  shift 3
  local -a headers=("$@")
  local -a args=(-sS -X "$method" "$url" -H 'Accept: application/json')
  local h
  for h in "${headers[@]}"; do
    args+=(-H "$h")
  done
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$body")
  fi

  local out
  out="$(curl "${args[@]}" -w $'\n%{http_code}')"
  HTTP_CODE="${out##*$'\n'}"
  HTTP_BODY="${out%$'\n'*}"
}

assert_http_code() {
  local expected="$1"
  local action="$2"
  if [[ "$HTTP_CODE" != "$expected" ]]; then
    echo "[fail] $action status=$HTTP_CODE body=$HTTP_BODY" >&2
    exit 1
  fi
}

assert_jq() {
  local json="$1"
  local message="$2"
  shift 2
  if ! jq -e "$@" >/dev/null <<<"$json"; then
    echo "[fail] $message body=$json" >&2
    exit 1
  fi
}

wait_for_get() {
  local url="$1"
  local expected_code="$2"
  local success_expr="$3"
  local timeout_sec="$4"
  local interval_sec="$5"
  local description="$6"
  shift 6
  local -a headers=("$@")
  local deadline_epoch=$(( $(date +%s) + timeout_sec ))
  while true; do
    http_call GET "$url" '' "${headers[@]}"
    if [[ "$HTTP_CODE" == "$expected_code" ]] && jq -e "$success_expr" >/dev/null <<<"$HTTP_BODY"; then
      return 0
    fi
    if [[ $(date +%s) -ge $deadline_epoch ]]; then
      echo "[fail] timeout waiting for $description status=$HTTP_CODE body=$HTTP_BODY" >&2
      exit 1
    fi
    sleep "$interval_sec"
  done
}

select_usage_invoice() {
  local invoices_json="$1"
  local expected_amount_cents="$2"
  local meter_key="$3"
  local candidate_id
  while IFS= read -r candidate_id; do
    [[ -n "$candidate_id" ]] || continue
    local candidate_id_enc
    candidate_id_enc="$(urlencode "$candidate_id")"
    http_call GET "$LAGO_API_URL/api/v1/invoices/$candidate_id_enc" '' "Authorization: Bearer $LAGO_API_KEY"
    if [[ "$HTTP_CODE" != "200" ]]; then
      continue
    fi
    if jq -e \
      --arg meter_key "$meter_key" \
      --argjson expected_amount_cents "$expected_amount_cents" \
      '.invoice.total_amount_cents == $expected_amount_cents
       and ((.invoice.fees // []) | length > 0)
       and any((.invoice.fees // [])[];
         ((.amount_details.billable_metric_code // .amount_details.metric_code // "") == $meter_key)
         or ((.item.code // "") == $meter_key)
         or ((.lago_subscription_id // "") | length > 0)
       )' >/dev/null <<<"$HTTP_BODY"; then
      SELECTED_INVOICE_ID="$candidate_id"
      SELECTED_INVOICE_JSON="$HTTP_BODY"
      return 0
    fi
  done < <(jq -r '.invoices | sort_by(.created_at // "") | reverse | .[] | (.lago_id // .id // empty)' <<<"$invoices_json")
  return 1
}

require_env ALPHA_API_BASE_URL
require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY
require_env PLATFORM_ADMIN_API_KEY
require_env LAGO_API_URL
require_env LAGO_API_KEY

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"
OUTPUT_FILE="${OUTPUT_FILE:-}"
TIMEOUT_SEC="${TIMEOUT_SEC:-600}"
POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}"
SUBSCRIPTION_STARTED_AT="${SUBSCRIPTION_STARTED_AT:-$(date_offset_utc 45)}"
USAGE_TIMESTAMP="${USAGE_TIMESTAMP:-$(date_offset_utc 35)}"

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"

subscription_summary_file="$(mktemp)"
trap 'rm -f "$subscription_summary_file"' EXIT

echo "[info] preparing backdated metered subscription fixture run_id=$RUN_ID"
ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
ALPHA_WRITER_API_KEY="$ALPHA_WRITER_API_KEY" \
ALPHA_READER_API_KEY="$ALPHA_READER_API_KEY" \
PLATFORM_ADMIN_API_KEY="$PLATFORM_ADMIN_API_KEY" \
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
TARGET_TENANT_ID="$TARGET_TENANT_ID" \
RUN_ID="$RUN_ID" \
SUBSCRIPTION_STARTED_AT="$SUBSCRIPTION_STARTED_AT" \
USAGE_TIMESTAMP="$USAGE_TIMESTAMP" \
OUTPUT_FILE="$subscription_summary_file" \
bash ./scripts/verify_staging_subscription_journey.sh >/dev/null

subscription_summary_json="$(cat "$subscription_summary_file")"
customer_external_id="$(jq -r '.entities.customer.external_id // empty' <<<"$subscription_summary_json")"
customer_id="$(jq -r '.entities.customer.id // empty' <<<"$subscription_summary_json")"
subscription_id="$(jq -r '.entities.subscription.id // empty' <<<"$subscription_summary_json")"
subscription_code="$(jq -r '.entities.subscription.code // empty' <<<"$subscription_summary_json")"
meter_key="$(jq -r '.entities.meter.key // empty' <<<"$subscription_summary_json")"
expected_amount_cents="$(jq -r '.verification.lago_customer_usage.amount_cents // empty' <<<"$subscription_summary_json")"
if [[ -z "$customer_external_id" || -z "$customer_id" || -z "$subscription_id" || -z "$subscription_code" || -z "$meter_key" || -z "$expected_amount_cents" ]]; then
  echo "[fail] subscription summary missing required invoice journey fields body=$subscription_summary_json" >&2
  exit 1
fi
if ! [[ "$expected_amount_cents" =~ ^[0-9]+$ ]] || [[ "$expected_amount_cents" -le 0 ]]; then
  echo "[fail] expected amount cents must be positive body=$subscription_summary_json" >&2
  exit 1
fi

customer_external_id_enc="$(urlencode "$customer_external_id")"
invoice_list_url="$LAGO_API_URL/api/v1/invoices?external_customer_id=$customer_external_id_enc"

echo "[info] waiting for lago invoice generation customer_external_id=$customer_external_id"
wait_for_get "$invoice_list_url" 200 '.invoices | length > 0' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "generated lago invoice" "Authorization: Bearer $LAGO_API_KEY"
lago_invoice_list_json="$HTTP_BODY"

SELECTED_INVOICE_ID=""
SELECTED_INVOICE_JSON=""
if ! select_usage_invoice "$lago_invoice_list_json" "$expected_amount_cents" "$meter_key"; then
  echo "[fail] could not identify usage-backed invoice for customer body=$lago_invoice_list_json" >&2
  exit 1
fi

invoice_id="$SELECTED_INVOICE_ID"
lago_invoice_json="$SELECTED_INVOICE_JSON"
invoice_id_enc="$(urlencode "$invoice_id")"

invoice_status="$(jq -r '.invoice.status // empty' <<<"$lago_invoice_json")"
if [[ "$invoice_status" == "draft" ]]; then
  echo "[info] finalizing draft lago invoice id=$invoice_id"
  http_call PUT "$LAGO_API_URL/api/v1/invoices/$invoice_id_enc/finalize" '' "Authorization: Bearer $LAGO_API_KEY"
  assert_http_code 200 "finalize lago invoice"
  lago_invoice_json="$HTTP_BODY"
fi
assert_jq "$lago_invoice_json" "lago invoice did not finalize to expected issued state" \
  --arg invoice_id "$invoice_id" \
  --arg customer_external_id "$customer_external_id" \
  --argjson expected_amount_cents "$expected_amount_cents" \
  '.invoice.lago_id == $invoice_id
   and .invoice.customer.external_id == $customer_external_id
   and .invoice.status == "finalized"
   and .invoice.total_amount_cents == $expected_amount_cents'

alpha_invoice_list_url="$ALPHA_API_BASE_URL/v1/invoices?customer_external_id=$customer_external_id_enc"
echo "[info] waiting for alpha invoice list visibility invoice_id=$invoice_id"
wait_for_get "$alpha_invoice_list_url" 200 ".items | map(select(.invoice_id == \"$invoice_id\" and .invoice_status == \"finalized\")) | length >= 1" "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "alpha invoice list visibility" "X-API-Key: $ALPHA_READER_API_KEY"
alpha_invoice_list_json="$HTTP_BODY"

http_call GET "$ALPHA_API_BASE_URL/v1/invoices/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get alpha invoice detail"
alpha_invoice_detail_json="$HTTP_BODY"
assert_jq "$alpha_invoice_detail_json" "alpha invoice detail mismatch" \
  --arg invoice_id "$invoice_id" \
  --arg customer_external_id "$customer_external_id" \
  --argjson expected_amount_cents "$expected_amount_cents" \
  '.invoice_id == $invoice_id
   and .customer_external_id == $customer_external_id
   and .invoice_status == "finalized"
   and .total_amount_cents == $expected_amount_cents
   and (.billing_entity_code // "") != ""'

http_call GET "$ALPHA_API_BASE_URL/v1/invoices/$invoice_id/explainability?limit=50&page=1" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get invoice explainability"
alpha_explainability_json="$HTTP_BODY"
assert_jq "$alpha_explainability_json" "invoice explainability missing usage-backed line items" \
  --arg invoice_id "$invoice_id" \
  --arg meter_key "$meter_key" \
  --argjson expected_amount_cents "$expected_amount_cents" \
  '.invoice_id == $invoice_id
   and .invoice_status == "finalized"
   and .line_items_count >= 1
   and any(.line_items[];
     .total_amount_cents == $expected_amount_cents
     and ((.billable_metric_code // "") == $meter_key or (.item_code // "") == $meter_key or (.subscription_id // "") != "")
   )'

summary_json="$(jq -n \
  --arg run_id "$RUN_ID" \
  --arg tenant_id "$TARGET_TENANT_ID" \
  --arg started_at "$SUBSCRIPTION_STARTED_AT" \
  --arg usage_timestamp "$USAGE_TIMESTAMP" \
  --arg invoice_id "$invoice_id" \
  --arg customer_external_id "$customer_external_id" \
  --arg subscription_id "$subscription_id" \
  --arg subscription_code "$subscription_code" \
  --arg meter_key "$meter_key" \
  --argjson expected_amount_cents "$expected_amount_cents" \
  --slurpfile subscription_summary <(printf '%s\n' "$subscription_summary_json") \
  --slurpfile lago_invoice_list <(printf '%s\n' "$lago_invoice_list_json") \
  --slurpfile lago_invoice <(printf '%s\n' "$lago_invoice_json") \
  --slurpfile alpha_invoice_list <(printf '%s\n' "$alpha_invoice_list_json") \
  --slurpfile alpha_invoice_detail <(printf '%s\n' "$alpha_invoice_detail_json") \
  --slurpfile alpha_explainability <(printf '%s\n' "$alpha_explainability_json") \
  '{
    run_id: $run_id,
    tenant_id: $tenant_id,
    journey: "usage_to_issued_invoice",
    fixture: {
      subscription_started_at: $started_at,
      usage_timestamp: $usage_timestamp,
      customer_external_id: $customer_external_id,
      subscription_id: $subscription_id,
      subscription_code: $subscription_code,
      meter_key: $meter_key,
      expected_amount_cents: $expected_amount_cents
    },
    invoice_id: $invoice_id,
    verification: {
      subscription_journey: $subscription_summary[0],
      lago_invoice_list: $lago_invoice_list[0],
      lago_invoice: $lago_invoice[0],
      alpha_invoice_list: $alpha_invoice_list[0],
      alpha_invoice_detail: $alpha_invoice_detail[0],
      alpha_explainability: $alpha_explainability[0]
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

echo "[pass] staging usage-to-issued-invoice journey completed run_id=$RUN_ID invoice_id=$invoice_id customer_external_id=$customer_external_id"
printf '%s\n' "$summary_json"
