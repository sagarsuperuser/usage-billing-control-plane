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
    -billing-address-line1 "123 Payment Setup Street" \
    -billing-city "New York" \
    -billing-state "NY" \
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

CUSTOMER_EXTERNAL_ID="${CUSTOMER_EXTERNAL_ID:-cust_payment_setup_journey_${RUN_ID}}"
CUSTOMER_NAME="${CUSTOMER_NAME:-Payment Setup Journey Customer ${RUN_ID}}"
CUSTOMER_EMAIL="${CUSTOMER_EMAIL:-billing+payment-setup-${RUN_ID}@alpha.test}"
ADD_ON_CODE="${ADD_ON_CODE:-alpha-payment-setup-journey-${RUN_ID}}"
PAYMENT_METHOD_FIXTURE="${PAYMENT_METHOD_FIXTURE:-pm_card_visa}"
BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID="${BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_payment_setup_bootstrap_success_${RUN_ID}}"

bootstrap_json_file="$(mktemp)"
fixture_json_file="$(mktemp)"
failure_result_json_file="$(mktemp)"
success_result_json_file="$(mktemp)"
reconcile_json_file="$(mktemp)"
trap 'rm -f "$bootstrap_json_file" "$fixture_json_file" "$failure_result_json_file" "$success_result_json_file" "$reconcile_json_file"' EXIT

echo "[info] bootstrapping staging lago organization/provider run_id=$RUN_ID"
RUN_ID="$RUN_ID" \
SUCCESS_CUSTOMER_EXTERNAL_ID="$BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID" \
FAILURE_CUSTOMER_EXTERNAL_ID="$CUSTOMER_EXTERNAL_ID" \
LAGO_WEBHOOK_URL="${LAGO_WEBHOOK_URL:-$ALPHA_API_BASE_URL/internal/lago/webhooks}" \
bash ./scripts/bootstrap_lago_stripe_staging.sh >"$bootstrap_json_file"

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

echo "[info] ensuring alpha customer exists external_id=$CUSTOMER_EXTERNAL_ID"
create_alpha_customer "$CUSTOMER_EXTERNAL_ID" "$CUSTOMER_NAME" "$CUSTOMER_EMAIL"

echo "[info] syncing customer billing profile to lago"
billing_profile_result_json="$(upsert_alpha_billing_profile "$CUSTOMER_EXTERNAL_ID" "$CUSTOMER_NAME" "$CUSTOMER_EMAIL")"
billing_profile_json="$(jq -c '.response' <<<"$billing_profile_result_json")"
assert_jq "$billing_profile_json" "billing profile is not ready after sync" '.profile_status == "ready" and (.last_sync_error // "") == "" and (.last_synced_at != null)'

customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"

echo "[info] confirming initial readiness is pending without verified payment method" 

http_call GET "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/readiness" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get customer readiness"
initial_readiness_json="$HTTP_BODY"
assert_jq "$initial_readiness_json" "initial customer readiness should be pending before payment setup" \
  '.status == "pending"
   and (.payment_setup_status == "missing" or .payment_setup_status == "pending")
   and .default_payment_method_verified == false'

echo "[info] preparing collectible invoice fixture for payment-setup journey"
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
CUSTOMER_EXTERNAL_ID="$CUSTOMER_EXTERNAL_ID" \
ADD_ON_CODE="$ADD_ON_CODE" \
OUTPUT_FILE="$fixture_json_file" \
bash ./scripts/prepare_real_payment_invoice_fixture.sh

invoice_id="$(jq -r '.invoice_id // empty' "$fixture_json_file")"
if [[ -z "$invoice_id" ]]; then
  echo "[fail] payment-setup fixture did not produce invoice_id" >&2
  exit 1
fi

echo "[info] forcing failed payment outcome before collect-payment recovery invoice=$invoice_id"
ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
ALPHA_WRITER_API_KEY="$ALPHA_WRITER_API_KEY" \
ALPHA_READER_API_KEY="$ALPHA_READER_API_KEY" \
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
INVOICE_ID="$invoice_id" \
EXPECTED_FINAL_STATUS="failed" \
EXPECTED_LIFECYCLE_ACTION="collect_payment" \
EXPECTED_LIFECYCLE_REQUIRES_ACTION="true" \
EXPECTED_LIFECYCLE_RETRY_RECOMMENDED="false" \
OUTPUT_FILE="$failure_result_json_file" \
bash ./scripts/test_real_payment_e2e.sh

echo "[info] verifying payment detail recommends collect_payment"
http_call GET "$ALPHA_API_BASE_URL/v1/payments/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get failed payment detail"
failed_payment_detail_json="$HTTP_BODY"
assert_jq "$failed_payment_detail_json" "failed payment detail must recommend collect_payment before setup" \
  --arg invoice_id "$invoice_id" \
  --arg external_id "$CUSTOMER_EXTERNAL_ID" \
  '.invoice_id == $invoice_id
   and .customer_external_id == $external_id
   and .lifecycle.recommended_action == "collect_payment"
   and .lifecycle.retry_recommended == false'

echo "[info] sending payment setup request from alpha"
http_call POST "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/payment-setup/request" '{"payment_method_type":"card"}' "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "request customer payment setup"
request_result_json="$HTTP_BODY"
assert_jq "$request_result_json" "payment setup request did not produce checkout url and sent status" \
  '.action == "requested"
   and ((.checkout_url // "") | length > 0)
   and .payment_setup.last_request_status == "sent"
   and ((.payment_setup.last_request_to_email // "") | length > 0)'

echo "[info] completing provider-side payment setup deterministically"
LAGO_ORG_ID="$bootstrap_org_id" \
STRIPE_PROVIDER_CODE="$bootstrap_provider_code" \
CUSTOMER_EXTERNAL_ID="$CUSTOMER_EXTERNAL_ID" \
PAYMENT_METHOD_ACTION="attach_default" \
PAYMENT_METHOD_FIXTURE="$PAYMENT_METHOD_FIXTURE" \
bash ./scripts/reconcile_lago_stripe_customer_payment_method.sh >"$reconcile_json_file"

echo "[info] refreshing customer payment setup in alpha"
http_call POST "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/payment-setup/refresh" '{}' "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "refresh customer payment setup"
refresh_result_json="$HTTP_BODY"
assert_jq "$refresh_result_json" "refreshed readiness should be payment ready" \
  '.readiness.status == "ready"
   and .readiness.payment_setup_status == "ready"
   and .readiness.default_payment_method_verified == true
   and .payment_setup.default_payment_method_present == true'

echo "[info] verifying payment detail now recommends retry_payment"
http_call GET "$ALPHA_API_BASE_URL/v1/payments/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get recovered payment detail"
ready_payment_detail_json="$HTTP_BODY"
assert_jq "$ready_payment_detail_json" "payment detail should switch to retry_payment after payment setup is ready" \
  '.lifecycle.recommended_action == "retry_payment"
   and .lifecycle.retry_recommended == true'

echo "[info] retrying the same invoice after payment setup completion invoice=$invoice_id"
ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
ALPHA_WRITER_API_KEY="$ALPHA_WRITER_API_KEY" \
ALPHA_READER_API_KEY="$ALPHA_READER_API_KEY" \
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
INVOICE_ID="$invoice_id" \
EXPECTED_FINAL_STATUS="succeeded" \
OUTPUT_FILE="$success_result_json_file" \
bash ./scripts/test_real_payment_e2e.sh

echo "[info] verifying final payment detail shows success"
http_call GET "$ALPHA_API_BASE_URL/v1/payments/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get final payment detail"
final_payment_detail_json="$HTTP_BODY"
assert_jq "$final_payment_detail_json" "final payment detail should no longer require collection action" \
  '.payment_status == "succeeded"
   and .lifecycle.recommended_action == "none"
   and .lifecycle.requires_action == false
   and .lifecycle.retry_recommended == false'

summary_json="$(
jq -n \
  --arg run_id "$RUN_ID" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg customer_email "$CUSTOMER_EMAIL" \
  --arg invoice_id "$invoice_id" \
  --arg payment_method_fixture "$PAYMENT_METHOD_FIXTURE" \
  --arg target_tenant_id "$TARGET_TENANT_ID" \
  --slurpfile bootstrap "$bootstrap_json_file" \
  --slurpfile fixture "$fixture_json_file" \
  --slurpfile failed "$failure_result_json_file" \
  --slurpfile success "$success_result_json_file" \
  --slurpfile reconcile "$reconcile_json_file" \
  --argjson initial_readiness "$initial_readiness_json" \
  --argjson request_result "$request_result_json" \
  --argjson refresh_result "$refresh_result_json" \
  --argjson failed_payment_detail "$failed_payment_detail_json" \
  --argjson ready_payment_detail "$ready_payment_detail_json" \
  --argjson final_payment_detail "$final_payment_detail_json" \
  '{
    run_id: $run_id,
    customer: {
      external_id: $customer_external_id,
      email: $customer_email
    },
    invoice_id: $invoice_id,
    payment_method_fixture: $payment_method_fixture,
    target_tenant_id: $target_tenant_id,
    bootstrap: ($bootstrap[0] // null),
    fixture: ($fixture[0] // null),
    initial_readiness: $initial_readiness,
    payment_setup_request: $request_result,
    provider_completion: ($reconcile[0] // null),
    refresh_result: $refresh_result,
    failed_before_setup: {
      payment_detail: $failed_payment_detail,
      payment_e2e: ($failed[0] // null)
    },
    ready_before_retry: {
      payment_detail: $ready_payment_detail
    },
    succeeded_after_setup: {
      payment_detail: $final_payment_detail,
      payment_e2e: ($success[0] // null)
    }
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
