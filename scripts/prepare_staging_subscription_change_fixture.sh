#!/usr/bin/env bash
set -euo pipefail

required_cmds=(bash curl jq)
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
  jq -nr --arg value "$1" '$value|@uri'
}

http_call() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  shift 3
  local -a headers=("$@")
  local -a args=(-sS -X "$method" "$url" -H 'Accept: application/json')
  local header
  for header in "${headers[@]}"; do
    args+=(-H "$header")
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

require_env ALPHA_API_BASE_URL
require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY
require_env BILLING_PROVIDER_CODE
require_env LAGO_API_URL
require_env LAGO_API_KEY

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
OUTPUT_FILE="${OUTPUT_FILE:-}"
ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"

METRIC_KEY="${METRIC_KEY:-subchange_metric_${RUN_ID}}"
METRIC_NAME="${METRIC_NAME:-Subscription Change Metric ${RUN_ID}}"
CURRENT_PLAN_CODE="${CURRENT_PLAN_CODE:-subchange_current_${RUN_ID}}"
CURRENT_PLAN_NAME="${CURRENT_PLAN_NAME:-Subscription Current ${RUN_ID}}"
TARGET_PLAN_CODE="${TARGET_PLAN_CODE:-subchange_target_${RUN_ID}}"
TARGET_PLAN_NAME="${TARGET_PLAN_NAME:-Subscription Target ${RUN_ID}}"
CUSTOMER_EXTERNAL_ID="${CUSTOMER_EXTERNAL_ID:-cust_subchange_${RUN_ID}}"
CUSTOMER_NAME="${CUSTOMER_NAME:-Subscription Change Customer ${RUN_ID}}"
CUSTOMER_EMAIL="${CUSTOMER_EMAIL:-billing+subchange-${RUN_ID}@alpha.test}"
SUBSCRIPTION_CODE="${SUBSCRIPTION_CODE:-subchange_sub_${RUN_ID}}"
SUBSCRIPTION_NAME="${SUBSCRIPTION_NAME:-Subscription Change ${RUN_ID}}"

create_metric_payload="$(jq -nc \
  --arg key "$METRIC_KEY" \
  --arg name "$METRIC_NAME" \
  '{key: $key, name: $name, unit: "event", aggregation: "sum", currency: "USD"}')"

echo "[info] creating pricing metric key=$METRIC_KEY"
http_call POST "$ALPHA_API_BASE_URL/v1/pricing/metrics" "$create_metric_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create pricing metric"
metric_json="$HTTP_BODY"
metric_id="$(jq -r '.id // empty' <<<"$metric_json")"
if [[ -z "$metric_id" ]]; then
  echo "[fail] metric id missing body=$metric_json" >&2
  exit 1
fi

create_plan() {
  local code="$1"
  local name="$2"
  local base_amount_cents="$3"
  local payload
  payload="$(jq -nc \
    --arg code "$code" \
    --arg name "$name" \
    --arg meter_id "$metric_id" \
    --argjson base_amount_cents "$base_amount_cents" \
    '{code: $code, name: $name, description: $name, currency: "USD", billing_interval: "monthly", status: "active", base_amount_cents: $base_amount_cents, meter_ids: [$meter_id]}')"
  http_call POST "$ALPHA_API_BASE_URL/v1/plans" "$payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
  assert_http_code 201 "create plan $code"
  printf '%s' "$HTTP_BODY"
}

echo "[info] creating current plan code=$CURRENT_PLAN_CODE"
current_plan_json="$(create_plan "$CURRENT_PLAN_CODE" "$CURRENT_PLAN_NAME" 2900)"
CURRENT_PLAN_ID="$(jq -r '.id // empty' <<<"$current_plan_json")"

echo "[info] creating target plan code=$TARGET_PLAN_CODE"
target_plan_json="$(create_plan "$TARGET_PLAN_CODE" "$TARGET_PLAN_NAME" 5900)"
TARGET_PLAN_ID="$(jq -r '.id // empty' <<<"$target_plan_json")"

if [[ -z "$CURRENT_PLAN_ID" || -z "$TARGET_PLAN_ID" ]]; then
  echo "[fail] plan ids missing current=$CURRENT_PLAN_ID target=$TARGET_PLAN_ID" >&2
  exit 1
fi

echo "[info] creating customer external_id=$CUSTOMER_EXTERNAL_ID"
create_customer_payload="$(jq -nc \
  --arg external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg display_name "$CUSTOMER_NAME" \
  --arg email "$CUSTOMER_EMAIL" \
  '{external_id: $external_id, display_name: $display_name, email: $email}')"
http_call POST "$ALPHA_API_BASE_URL/v1/customers" "$create_customer_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create customer"
customer_json="$HTTP_BODY"
customer_id="$(jq -r '.id // empty' <<<"$customer_json")"

echo "[info] syncing customer billing profile"
billing_profile_payload="$(jq -nc \
  --arg legal_name "$CUSTOMER_NAME" \
  --arg email "$CUSTOMER_EMAIL" \
  --arg provider_code "$BILLING_PROVIDER_CODE" \
  '{legal_name: $legal_name, email: $email, billing_address_line1: "1 Billing Street", billing_city: "Bengaluru", billing_postal_code: "560001", billing_country: "IN", currency: "USD", provider_code: $provider_code}')"
customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"
http_call PUT "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/billing-profile" "$billing_profile_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "upsert billing profile"
billing_profile_json="$HTTP_BODY"
assert_jq "$billing_profile_json" "billing profile not ready" '.profile_status == "ready"'

echo "[info] creating subscription code=$SUBSCRIPTION_CODE"
create_subscription_payload="$(jq -nc \
  --arg code "$SUBSCRIPTION_CODE" \
  --arg display_name "$SUBSCRIPTION_NAME" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg plan_id "$CURRENT_PLAN_ID" \
  '{code: $code, display_name: $display_name, customer_external_id: $customer_external_id, plan_id: $plan_id}')"
http_call POST "$ALPHA_API_BASE_URL/v1/subscriptions" "$create_subscription_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create subscription"
subscription_create_json="$HTTP_BODY"
SUBSCRIPTION_ID="$(jq -r '.subscription.id // empty' <<<"$subscription_create_json")"
if [[ -z "$SUBSCRIPTION_ID" ]]; then
  echo "[fail] subscription id missing body=$subscription_create_json" >&2
  exit 1
fi

subscription_id_enc="$(urlencode "$SUBSCRIPTION_ID")"
echo "[info] forcing active subscription state"
http_call PATCH "$ALPHA_API_BASE_URL/v1/subscriptions/$subscription_id_enc" '{"status":"active"}' "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "activate subscription"
subscription_json="$HTTP_BODY"
assert_jq "$subscription_json" "subscription should reflect current plan" \
  --arg current_plan_id "$CURRENT_PLAN_ID" \
  --arg current_plan_code "$CURRENT_PLAN_CODE" \
  '.plan_id == $current_plan_id and .plan_code == $current_plan_code and (.status == "active" or .status == "pending_payment_setup")'

subscription_code_enc="$(urlencode "$SUBSCRIPTION_CODE")"
echo "[info] verifying current Lago subscription"
http_call GET "$LAGO_API_URL/api/v1/subscriptions/$subscription_code_enc" '' "Authorization: Bearer $LAGO_API_KEY"
assert_http_code 200 "get lago subscription"
lago_subscription_json="$HTTP_BODY"
assert_jq "$lago_subscription_json" "lago subscription should start on current plan" \
  --arg external_id "$SUBSCRIPTION_CODE" \
  --arg current_plan_code "$CURRENT_PLAN_CODE" \
  '.subscription.external_id == $external_id and .subscription.plan_code == $current_plan_code'

summary_json="$(jq -n \
  --arg run_id "$RUN_ID" \
  --arg subscription_id "$SUBSCRIPTION_ID" \
  --arg subscription_code "$SUBSCRIPTION_CODE" \
  --arg subscription_name "$SUBSCRIPTION_NAME" \
  --arg customer_id "$customer_id" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg customer_name "$CUSTOMER_NAME" \
  --arg customer_email "$CUSTOMER_EMAIL" \
  --arg metric_id "$metric_id" \
  --arg metric_key "$METRIC_KEY" \
  --arg current_plan_id "$CURRENT_PLAN_ID" \
  --arg current_plan_code "$CURRENT_PLAN_CODE" \
  --arg current_plan_name "$CURRENT_PLAN_NAME" \
  --arg target_plan_id "$TARGET_PLAN_ID" \
  --arg target_plan_code "$TARGET_PLAN_CODE" \
  --arg target_plan_name "$TARGET_PLAN_NAME" \
  --arg provider_code "$BILLING_PROVIDER_CODE" \
  --argjson customer "$customer_json" \
  --argjson billing_profile "$billing_profile_json" \
  --argjson subscription "$subscription_json" \
  --argjson lago_subscription "$lago_subscription_json" \
  '{
    run_id: $run_id,
    fixture_source: "staging_subscription_change_fixture",
    metric: {id: $metric_id, key: $metric_key},
    customer: {id: $customer_id, external_id: $customer_external_id, display_name: $customer_name, email: $customer_email},
    subscription: {id: $subscription_id, code: $subscription_code, display_name: $subscription_name},
    current_plan: {id: $current_plan_id, code: $current_plan_code, name: $current_plan_name},
    target_plan: {id: $target_plan_id, code: $target_plan_code, name: $target_plan_name},
    billing_provider_code: $provider_code,
    verification: {
      customer_create: $customer,
      billing_profile: $billing_profile,
      subscription: $subscription,
      lago_subscription: $lago_subscription
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
else
  printf '%s\n' "$summary_json"
fi
