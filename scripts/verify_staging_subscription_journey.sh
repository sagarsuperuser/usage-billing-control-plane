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
  local payload
  payload="$(jq -nc \
    --arg lago_organization_id "$org_id" \
    --arg lago_billing_provider_code "$provider_code" \
    '{lago_organization_id: $lago_organization_id, lago_billing_provider_code: $lago_billing_provider_code}')"

  http_call PATCH "$ALPHA_API_BASE_URL/internal/tenants/$tenant_id" "$payload" "X-API-Key: $PLATFORM_ADMIN_API_KEY"
  assert_http_code 200 "patch tenant lago billing mapping"
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
USAGE_TRANSACTION_ID="${USAGE_TRANSACTION_ID:-usage-subjourney-${RUN_ID}}"
USAGE_TIMESTAMP="${USAGE_TIMESTAMP:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
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

bootstrap_org_id="$(jq -r '.organization.id // empty' "$bootstrap_json_file")"
bootstrap_provider_code="$(jq -r '.stripe_provider.code // empty' "$bootstrap_json_file")"
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

create_customer_payload="$(jq -nc \
  --arg external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg display_name "$CUSTOMER_NAME" \
  --arg email "$CUSTOMER_EMAIL" \
  '{external_id: $external_id, display_name: $display_name, email: $email}')"

echo "[info] creating alpha customer external_id=$CUSTOMER_EXTERNAL_ID"
http_call POST "$ALPHA_API_BASE_URL/v1/customers" "$create_customer_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create customer"
customer_json="$HTTP_BODY"
customer_id="$(jq -r '.id // empty' <<<"$customer_json")"
if [[ -z "$customer_id" ]]; then
  echo "[fail] missing customer id body=$customer_json" >&2
  exit 1
fi

billing_profile_payload="$(jq -nc \
  --arg legal_name "$CUSTOMER_NAME" \
  --arg email "$CUSTOMER_EMAIL" \
  --arg billing_address_line1 "123 Subscription Street" \
  --arg billing_city "New York" \
  --arg billing_postal_code "10001" \
  --arg billing_country "US" \
  --arg currency "USD" \
  '{
    legal_name: $legal_name,
    email: $email,
    billing_address_line1: $billing_address_line1,
    billing_city: $billing_city,
    billing_postal_code: $billing_postal_code,
    billing_country: $billing_country,
    currency: $currency
  }')"

echo "[info] syncing customer billing profile to lago"
customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"
http_call PUT "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/billing-profile" "$billing_profile_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "upsert billing profile"
billing_profile_json="$HTTP_BODY"
assert_jq "$billing_profile_json" "billing profile is not ready after sync" '.profile_status == "ready" and (.last_sync_error // "") == "" and (.last_synced_at != null)'

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
  --arg timestamp "$USAGE_TIMESTAMP" \
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

invoice_preview_payload="$(jq -nc \
  --arg external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg subscription_code "$SUBSCRIPTION_CODE" \
  '{
    customer: {
      external_id: $external_id
    },
    subscriptions: {
      external_ids: [$subscription_code]
    }
  }')"

echo "[info] requesting deterministic lago subscription invoice preview"
http_call POST "$LAGO_API_URL/api/v1/invoices/preview" "$invoice_preview_payload" "Authorization: Bearer $LAGO_API_KEY"
assert_http_code 200 "preview subscription invoice"
invoice_preview_json="$HTTP_BODY"
assert_jq "$invoice_preview_json" "subscription invoice preview mismatch" \
  --arg external_id "$CUSTOMER_EXTERNAL_ID" \
  '.invoice.invoice_type == "subscription" and .invoice.customer.external_id == $external_id and (.invoice.total_amount_cents > 0) and (.invoice.fees_amount_cents > 0) and ((.invoice.preview_subscriptions // []) | length >= 1)'

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
  --argjson bootstrap "$(<"$bootstrap_json_file")" \
  --argjson rating_rule "$rating_rule_json" \
  --argjson meter "$meter_json" \
  --argjson plan "$plan_json" \
  --argjson customer "$customer_detail_json" \
  --argjson billing_profile "$billing_profile_json" \
  --argjson subscription "$subscription_detail_json" \
  --argjson lago_subscription "$lago_subscription_json" \
  --argjson usage_event "$usage_event_json" \
  --argjson lago_event "$lago_event_json" \
  --argjson invoice_preview "$invoice_preview_json" \
  '{
    run_id: $run_id,
    fixture_source: "staging_subscription_journey",
    execution_model: {
      tenant_lago_mapping: "explicit platform patch after bootstrap",
      pricing_configuration: "real alpha rule -> meter -> plan create",
      customer_sync: "real alpha customer + billing profile sync to lago",
      subscription_sync: "real alpha subscription sync to lago",
      usage_sync: "real alpha subscription-targeted usage sync to lago",
      billing_proof: "real lago persisted subscription + event + invoice preview"
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
      bootstrap: $bootstrap,
      rating_rule: $rating_rule,
      meter: $meter,
      plan: $plan,
      customer: $customer,
      billing_profile: $billing_profile,
      subscription: $subscription,
      lago_subscription: $lago_subscription,
      alpha_usage_event: $usage_event,
      lago_event: $lago_event,
      lago_invoice_preview: $invoice_preview
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
