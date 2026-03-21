#!/usr/bin/env bash
set -euo pipefail

required_cmds=(bash curl go jq mktemp)
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

ensure_tenant_lago_mapping() {
  local tenant_id="$1"
  local org_id="$2"
  local provider_code="$3"
  go run ./cmd/admin ensure-tenant-lago-mapping \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -platform-api-key "$PLATFORM_ADMIN_API_KEY" \
    -tenant-id "$tenant_id" \
    -organization-id "$org_id" \
    -provider-code "$provider_code" >/dev/null
}

create_alpha_customer() {
  local external_id="$1"
  local display_name="$2"
  local email="$3"
  go run ./cmd/admin ensure-alpha-customers \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -writer-api-key "$ALPHA_WRITER_API_KEY" \
    -conflict-is-error \
    -customer "$external_id|$display_name|$email" >/dev/null
}

upsert_alpha_billing_profile() {
  local customer_external_id="$1"
  local legal_name="$2"
  local email="$3"
  go run ./cmd/admin upsert-customer-billing-profile \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -writer-api-key "$ALPHA_WRITER_API_KEY" \
    -customer-external-id "$customer_external_id" \
    -legal-name "$legal_name" \
    -email "$email" \
    -billing-address-line1 "123 Subscription Street" \
    -billing-city "New York" \
    -billing-postal-code "10001" \
    -billing-country "US" \
    -currency "USD"
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

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"

RULE_KEY="${RULE_KEY:-subjourney_${RUN_ID}}"
RULE_NAME="${RULE_NAME:-Subscription Journey Rule ${RUN_ID}}"
METER_KEY="${METER_KEY:-subjourney_meter_${RUN_ID}}"
METER_NAME="${METER_NAME:-Subscription Journey Meter ${RUN_ID}}"
PLAN_CODE="${PLAN_CODE:-subjourney_plan_${RUN_ID}}"
PLAN_NAME="${PLAN_NAME:-Subscription Journey Plan ${RUN_ID}}"
CUSTOMER_EXTERNAL_ID="${CUSTOMER_EXTERNAL_ID:-cust_subscription_journey_${RUN_ID}}"
CUSTOMER_NAME="${CUSTOMER_NAME:-Subscription Journey Customer ${RUN_ID}}"
CUSTOMER_EMAIL="${CUSTOMER_EMAIL:-billing+subscription-${RUN_ID}@alpha.test}"
SUBSCRIPTION_CODE="${SUBSCRIPTION_CODE:-subjourney_sub_${RUN_ID}}"
SUBSCRIPTION_NAME="${SUBSCRIPTION_NAME:-Subscription Journey ${RUN_ID}}"
USAGE_QUANTITY="${USAGE_QUANTITY:-12}"
UNIT_AMOUNT_CENTS="${UNIT_AMOUNT_CENTS:-125}"
EXPECTED_USAGE_AMOUNT_CENTS="${EXPECTED_USAGE_AMOUNT_CENTS:-$((USAGE_QUANTITY * UNIT_AMOUNT_CENTS))}"
USAGE_TRANSACTION_ID="${USAGE_TRANSACTION_ID:-usage-subjourney-${RUN_ID}}"
USAGE_TIMESTAMP="${USAGE_TIMESTAMP:-}"
BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID="${BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_subscription_bootstrap_success_${RUN_ID}}"
BOOTSTRAP_FAILURE_CUSTOMER_EXTERNAL_ID="${BOOTSTRAP_FAILURE_CUSTOMER_EXTERNAL_ID:-cust_subscription_bootstrap_failure_${RUN_ID}}"

bootstrap_json_file="$(mktemp)"
trap 'rm -f "$bootstrap_json_file"' EXIT

echo "[info] bootstrapping staging lago organization/provider run_id=$RUN_ID"
RUN_ID="$RUN_ID" \
SUCCESS_CUSTOMER_EXTERNAL_ID="$BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID" \
FAILURE_CUSTOMER_EXTERNAL_ID="$BOOTSTRAP_FAILURE_CUSTOMER_EXTERNAL_ID" \
LAGO_WEBHOOK_URL="${LAGO_WEBHOOK_URL:-$ALPHA_API_BASE_URL/internal/lago/webhooks}" \
bash ./scripts/bootstrap_lago_stripe_staging.sh > "$bootstrap_json_file"

bootstrap_summary_json="$(jq -cs 'map(select(.organization? != null and .stripe_provider? != null and .customers? != null)) | last' "$bootstrap_json_file")"
if [[ -z "$bootstrap_summary_json" || "$bootstrap_summary_json" == "null" ]]; then
  echo "[fail] bootstrap output did not contain a summary object" >&2
  exit 1
fi

bootstrap_org_id="$(jq -r '.organization.id // empty' <<<"$bootstrap_summary_json")"
bootstrap_provider_code="$(jq -r '.stripe_provider.code // empty' <<<"$bootstrap_summary_json")"
if [[ -z "$bootstrap_org_id" || -z "$bootstrap_provider_code" ]]; then
  echo "[fail] bootstrap did not produce organization.id and stripe_provider.code" >&2
  exit 1
fi

echo "[info] ensuring tenant lago billing mapping tenant_id=$TARGET_TENANT_ID"
ensure_tenant_lago_mapping "$TARGET_TENANT_ID" "$bootstrap_org_id" "$bootstrap_provider_code"

create_rule_payload="$(jq -nc \
  --arg rule_key "$RULE_KEY" \
  --arg name "$RULE_NAME" \
  --arg currency "USD" \
  --argjson flat_amount_cents "$UNIT_AMOUNT_CENTS" \
  '{
    rule_key: $rule_key,
    name: $name,
    version: 1,
    lifecycle_state: "active",
    mode: "flat",
    currency: $currency,
    flat_amount_cents: $flat_amount_cents
  }')"

echo "[info] creating rating rule rule_key=$RULE_KEY"
http_call POST "$ALPHA_API_BASE_URL/v1/rating-rules" "$create_rule_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create rating rule"
rating_rule_json="$HTTP_BODY"
rating_rule_version_id="$(jq -r '.id // empty' <<<"$rating_rule_json")"
if [[ -z "$rating_rule_version_id" ]]; then
  echo "[fail] missing rating rule id body=$rating_rule_json" >&2
  exit 1
fi

create_meter_payload="$(jq -nc \
  --arg key "$METER_KEY" \
  --arg name "$METER_NAME" \
  --arg rating_rule_version_id "$rating_rule_version_id" \
  '{key: $key, name: $name, unit: "event", aggregation: "sum", rating_rule_version_id: $rating_rule_version_id}')"

echo "[info] creating meter key=$METER_KEY"
http_call POST "$ALPHA_API_BASE_URL/v1/meters" "$create_meter_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create meter"
meter_json="$HTTP_BODY"
meter_id="$(jq -r '.id // empty' <<<"$meter_json")"
if [[ -z "$meter_id" ]]; then
  echo "[fail] missing meter id body=$meter_json" >&2
  exit 1
fi

create_plan_payload="$(jq -nc \
  --arg code "$PLAN_CODE" \
  --arg name "$PLAN_NAME" \
  --arg meter_id "$meter_id" \
  '{
    code: $code,
    name: $name,
    description: "Subscription journey plan",
    currency: "USD",
    billing_interval: "monthly",
    status: "active",
    base_amount_cents: 0,
    meter_ids: [$meter_id]
  }')"

echo "[info] creating plan code=$PLAN_CODE"
http_call POST "$ALPHA_API_BASE_URL/v1/plans" "$create_plan_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create plan"
plan_json="$HTTP_BODY"
plan_id="$(jq -r '.id // empty' <<<"$plan_json")"
if [[ -z "$plan_id" ]]; then
  echo "[fail] missing plan id body=$plan_json" >&2
  exit 1
fi

echo "[info] creating alpha customer external_id=$CUSTOMER_EXTERNAL_ID"
create_alpha_customer "$CUSTOMER_EXTERNAL_ID" "$CUSTOMER_NAME" "$CUSTOMER_EMAIL"

echo "[info] syncing customer billing profile to lago"
billing_profile_result_json="$(upsert_alpha_billing_profile "$CUSTOMER_EXTERNAL_ID" "$CUSTOMER_NAME" "$CUSTOMER_EMAIL")"
billing_profile_json="$(jq -c '.response' <<<"$billing_profile_result_json")"
assert_jq "$billing_profile_json" "billing profile is not ready after sync" '.profile_status == "ready" and (.last_sync_error // "") == "" and (.last_synced_at != null)'

customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"

echo "[info] verifying alpha customer detail reflects lago sync"

http_call GET "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get synced customer"
customer_detail_json="$HTTP_BODY"
assert_jq "$customer_detail_json" "customer detail missing lago sync markers" --arg external_id "$CUSTOMER_EXTERNAL_ID" '.external_id == $external_id and ((.lago_customer_id // "") | length > 0)'

create_subscription_payload="$(jq -nc \
  --arg code "$SUBSCRIPTION_CODE" \
  --arg display_name "$SUBSCRIPTION_NAME" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg plan_id "$plan_id" \
  '{
    code: $code,
    display_name: $display_name,
    customer_external_id: $customer_external_id,
    plan_id: $plan_id
  }')"

echo "[info] creating subscription code=$SUBSCRIPTION_CODE"
http_call POST "$ALPHA_API_BASE_URL/v1/subscriptions" "$create_subscription_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create subscription"
subscription_create_json="$HTTP_BODY"
subscription_id="$(jq -r '.subscription.id // empty' <<<"$subscription_create_json")"
if [[ -z "$subscription_id" ]]; then
  echo "[fail] missing subscription id body=$subscription_create_json" >&2
  exit 1
fi

subscription_id_enc="$(urlencode "$subscription_id")"
echo "[info] requesting active subscription state"
http_call PATCH "$ALPHA_API_BASE_URL/v1/subscriptions/$subscription_id_enc" '{"status":"active"}' "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "patch subscription status"
subscription_detail_json="$HTTP_BODY"
assert_jq "$subscription_detail_json" "subscription detail mismatch after status patch" \
  --arg subscription_id "$subscription_id" \
  --arg subscription_code "$SUBSCRIPTION_CODE" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg plan_id "$plan_id" \
  '.id == $subscription_id
   and .code == $subscription_code
   and .customer_external_id == $customer_external_id
   and .plan_id == $plan_id
   and (.status == "pending_payment_setup" or .status == "active")
   and ((.payment_setup_status // "") == "pending" or (.payment_setup_status // "") == "ready")'

echo "[info] verifying lago subscription sync external_id=$SUBSCRIPTION_CODE"
subscription_code_enc="$(urlencode "$SUBSCRIPTION_CODE")"
http_call GET "$LAGO_API_URL/api/v1/subscriptions/$subscription_code_enc" '' "Authorization: Bearer $LAGO_API_KEY"
assert_http_code 200 "get lago subscription"
lago_subscription_json="$HTTP_BODY"
assert_jq "$lago_subscription_json" "lago subscription mismatch" \
  --arg external_id "$SUBSCRIPTION_CODE" \
  --arg external_customer_id "$CUSTOMER_EXTERNAL_ID" \
  --arg plan_code "$PLAN_CODE" \
  '.subscription.external_id == $external_id and .subscription.external_customer_id == $external_customer_id and .subscription.plan_code == $plan_code'

create_usage_payload="$(jq -nc \
  --arg customer_id "$customer_id" \
  --arg meter_id "$meter_id" \
  --arg subscription_id "$subscription_id" \
  --arg idempotency_key "$USAGE_TRANSACTION_ID" \
  --arg timestamp "${USAGE_TIMESTAMP:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" \
  --argjson quantity "$USAGE_QUANTITY" \
  '{
    customer_id: $customer_id,
    meter_id: $meter_id,
    subscription_id: $subscription_id,
    quantity: $quantity,
    idempotency_key: $idempotency_key,
    timestamp: $timestamp
  }')"

echo "[info] creating subscription-targeted usage event transaction_id=$USAGE_TRANSACTION_ID"
http_call POST "$ALPHA_API_BASE_URL/v1/usage-events" "$create_usage_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create usage event"
usage_event_json="$HTTP_BODY"
assert_jq "$usage_event_json" "alpha usage event mismatch" \
  --arg customer_id "$customer_id" \
  --arg meter_id "$meter_id" \
  --arg subscription_id "$subscription_id" \
  --arg idempotency_key "$USAGE_TRANSACTION_ID" \
  --argjson quantity "$USAGE_QUANTITY" \
  '.customer_id == $customer_id and .meter_id == $meter_id and .subscription_id == $subscription_id and .idempotency_key == $idempotency_key and .quantity == $quantity'

echo "[info] verifying alpha usage list contains subscription-targeted event"
http_call GET "$ALPHA_API_BASE_URL/v1/usage-events?meter_id=$meter_id&limit=100&order=desc" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "list usage events"
usage_list_json="$HTTP_BODY"
assert_jq "$usage_list_json" "alpha usage list missing subscription-targeted event" \
  --arg idempotency_key "$USAGE_TRANSACTION_ID" \
  --arg subscription_id "$subscription_id" \
  '.items | map(select(.idempotency_key == $idempotency_key and .subscription_id == $subscription_id)) | length == 1'

echo "[info] verifying lago event replication transaction_id=$USAGE_TRANSACTION_ID"
usage_transaction_id_enc="$(urlencode "$USAGE_TRANSACTION_ID")"
http_call GET "$LAGO_API_URL/api/v1/events/$usage_transaction_id_enc" '' "Authorization: Bearer $LAGO_API_KEY"
assert_http_code 200 "get lago event"
lago_event_json="$HTTP_BODY"
assert_jq "$lago_event_json" "lago event mismatch" \
  --arg code "$METER_KEY" \
  --arg transaction_id "$USAGE_TRANSACTION_ID" \
  '.event.code == $code and .event.transaction_id == $transaction_id'

echo "[info] requesting deterministic lago current usage for persisted subscription"
customer_usage_query="external_subscription_id=$(urlencode "$SUBSCRIPTION_CODE")&apply_taxes=false"
http_call GET "$LAGO_API_URL/api/v1/customers/$customer_external_id_enc/current_usage?$customer_usage_query" '' "Authorization: Bearer $LAGO_API_KEY"
assert_http_code 200 "get customer current usage"
customer_usage_json="$HTTP_BODY"
assert_jq "$customer_usage_json" "subscription current usage mismatch" \
  --arg meter_key "$METER_KEY" \
  --arg currency "USD" \
  --argjson expected_amount_cents "$EXPECTED_USAGE_AMOUNT_CENTS" \
  '.customer_usage.currency == $currency
   and .customer_usage.amount_cents == $expected_amount_cents
   and .customer_usage.total_amount_cents == $expected_amount_cents
   and (.customer_usage.charges_usage | map(select(.billable_metric.code == $meter_key and .amount_cents == $expected_amount_cents)) | length >= 1)'
customer_usage_summary_json="$(jq -c '.customer_usage' <<<"$customer_usage_json")"
rating_rule_summary_json="$(jq -c '.' <<<"$rating_rule_json")"
meter_summary_json="$(jq -c '.' <<<"$meter_json")"
plan_summary_json="$(jq -c '.' <<<"$plan_json")"
customer_summary_json="$(jq -c '.' <<<"$customer_detail_json")"
billing_profile_summary_json="$(jq -c '.' <<<"$billing_profile_json")"
subscription_summary_json="$(jq -c '.' <<<"$subscription_detail_json")"
lago_subscription_summary_json="$(jq -c '.' <<<"$lago_subscription_json")"
usage_event_summary_json="$(jq -c '.' <<<"$usage_event_json")"
lago_event_summary_json="$(jq -c '.' <<<"$lago_event_json")"

summary_json="$(jq -n \
  --arg run_id "$RUN_ID" \
  --arg alpha_api_base_url "$ALPHA_API_BASE_URL" \
  --arg lago_api_url "$LAGO_API_URL" \
  --arg target_tenant_id "$TARGET_TENANT_ID" \
  --arg bootstrap_org_id "$bootstrap_org_id" \
  --arg bootstrap_provider_code "$bootstrap_provider_code" \
  --arg rule_key "$RULE_KEY" \
  --arg rating_rule_version_id "$rating_rule_version_id" \
  --arg meter_id "$meter_id" \
  --arg meter_key "$METER_KEY" \
  --arg plan_id "$plan_id" \
  --arg plan_code "$PLAN_CODE" \
  --arg customer_id "$customer_id" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg subscription_id "$subscription_id" \
  --arg subscription_code "$SUBSCRIPTION_CODE" \
  --arg usage_transaction_id "$USAGE_TRANSACTION_ID" \
  --argjson usage_quantity "$USAGE_QUANTITY" \
  --slurpfile bootstrap <(printf '%s\n' "$bootstrap_summary_json") \
  --slurpfile rating_rule <(printf '%s\n' "$rating_rule_summary_json") \
  --slurpfile meter <(printf '%s\n' "$meter_summary_json") \
  --slurpfile plan <(printf '%s\n' "$plan_summary_json") \
  --slurpfile customer <(printf '%s\n' "$customer_summary_json") \
  --slurpfile billing_profile <(printf '%s\n' "$billing_profile_summary_json") \
  --slurpfile subscription <(printf '%s\n' "$subscription_summary_json") \
  --slurpfile lago_subscription <(printf '%s\n' "$lago_subscription_summary_json") \
  --slurpfile usage_event <(printf '%s\n' "$usage_event_summary_json") \
  --slurpfile lago_event <(printf '%s\n' "$lago_event_summary_json") \
  --slurpfile customer_usage <(printf '%s\n' "$customer_usage_summary_json") \
  '{
    run_id: $run_id,
    fixture_source: "staging_subscription_journey",
    execution_model: {
      tenant_lago_mapping: "explicit platform patch after bootstrap",
      pricing_configuration: "real alpha rule -> meter -> plan create",
      customer_sync: "real alpha customer + billing profile sync to lago",
      subscription_sync: "real alpha subscription sync to lago",
      usage_sync: "real alpha subscription-targeted usage sync to lago",
      billing_proof: "real lago persisted subscription + event + current usage"
    },
    environment: {
      alpha_api_base_url: $alpha_api_base_url,
      lago_api_url: $lago_api_url,
      tenant_id: $target_tenant_id,
      lago_organization_id: $bootstrap_org_id,
      lago_billing_provider_code: $bootstrap_provider_code
    },
    entities: {
      rating_rule: {
        key: $rule_key,
        version_id: $rating_rule_version_id
      },
      meter: {
        id: $meter_id,
        key: $meter_key
      },
      plan: {
        id: $plan_id,
        code: $plan_code
      },
      customer: {
        id: $customer_id,
        external_id: $customer_external_id
      },
      subscription: {
        id: $subscription_id,
        code: $subscription_code
      },
      usage_event: {
        transaction_id: $usage_transaction_id,
        quantity: $usage_quantity
      }
    },
    verification: {
      bootstrap: $bootstrap[0],
      rating_rule: $rating_rule[0],
      meter: $meter[0],
      plan: $plan[0],
      customer: $customer[0],
      billing_profile: $billing_profile[0],
      subscription: $subscription[0],
      lago_subscription: $lago_subscription[0],
      alpha_usage_event: $usage_event[0],
      lago_event: $lago_event[0],
      lago_customer_usage: $customer_usage[0]
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
