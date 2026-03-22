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
    -billing-address-line1 "123 Dunning Street" \
    -billing-city "New York" \
    -billing-state "NY" \
    -billing-postal-code "10001" \
    -billing-country "US" \
    -currency "USD"
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
PAYMENT_METHOD_FIXTURE="${PAYMENT_METHOD_FIXTURE:-pm_card_visa}"

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"

CUSTOMER_EXTERNAL_ID="${CUSTOMER_EXTERNAL_ID:-cust_dunning_journey_${RUN_ID}}"
CUSTOMER_NAME="${CUSTOMER_NAME:-Dunning Journey Customer ${RUN_ID}}"
CUSTOMER_EMAIL="${CUSTOMER_EMAIL:-billing+dunning-${RUN_ID}@alpha.test}"
ADD_ON_CODE="${ADD_ON_CODE:-alpha-dunning-journey-${RUN_ID}}"
BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID="${BOOTSTRAP_SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_dunning_bootstrap_success_${RUN_ID}}"

bootstrap_json_file="$(mktemp)"
fixture_json_file="$(mktemp)"
reconcile_json_file="$(mktemp)"
detach_json_file="$(mktemp)"
trap 'rm -f "$bootstrap_json_file" "$fixture_json_file" "$reconcile_json_file" "$detach_json_file"' EXIT

customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"

policy_payload='{"name":"Default dunning policy","enabled":true,"retry_schedule":["0d","1d"],"max_retry_attempts":2,"collect_payment_reminder_schedule":["0d","1d"],"final_action":"manual_review","grace_period_days":0}'

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

echo "[info] forcing provider-side payment method absence before failure path"
LAGO_ORG_ID="$bootstrap_org_id" \
STRIPE_PROVIDER_CODE="$bootstrap_provider_code" \
CUSTOMER_EXTERNAL_ID="$CUSTOMER_EXTERNAL_ID" \
PAYMENT_METHOD_ACTION="detach_all" \
bash ./scripts/reconcile_lago_stripe_customer_payment_method.sh >"$detach_json_file"

echo "[info] installing deterministic dunning policy"
http_call PUT "$ALPHA_API_BASE_URL/v1/dunning/policy" "$policy_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 200 "put dunning policy"
dunning_policy_json="$HTTP_BODY"
assert_jq "$dunning_policy_json" "dunning policy did not persist deterministic schedules" '.policy.retry_schedule == ["0d","1d"] and .policy.collect_payment_reminder_schedule == ["0d","1d"] and .policy.max_retry_attempts == 2 and .policy.final_action == "manual_review"'

echo "[info] confirming initial readiness is pending without verified payment method"
http_call GET "$ALPHA_API_BASE_URL/v1/customers/$customer_external_id_enc/readiness" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get customer readiness"
initial_readiness_json="$HTTP_BODY"
assert_jq "$initial_readiness_json" "initial customer readiness should be pending before payment setup" '.status == "pending" and (.payment_setup_status == "missing" or .payment_setup_status == "pending") and .default_payment_method_verified == false'

echo "[info] preparing collectible invoice fixture for dunning journey"
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
CUSTOMER_EXTERNAL_ID="$CUSTOMER_EXTERNAL_ID" \
ADD_ON_CODE="$ADD_ON_CODE" \
OUTPUT_FILE="$fixture_json_file" \
bash ./scripts/prepare_real_payment_invoice_fixture.sh

invoice_id="$(jq -r '.invoice_id // empty' "$fixture_json_file")"
if [[ -z "$invoice_id" ]]; then
  echo "[fail] dunning fixture did not produce invoice_id" >&2
  exit 1
fi

echo "[info] triggering initial collection attempt for dunning invoice=$invoice_id"
http_call POST "$ALPHA_API_BASE_URL/v1/invoices/$invoice_id/retry-payment" {} "X-API-Key: $ALPHA_WRITER_API_KEY"
if [[ "$HTTP_CODE" != "200" && "$HTTP_CODE" != "202" ]]; then
  echo "[fail] initial retry-payment failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi
initial_retry_response_json="$HTTP_BODY"

echo "[info] waiting for Alpha lifecycle to recommend collect_payment"
wait_for_get "$ALPHA_API_BASE_URL/v1/invoice-payment-statuses/$invoice_id/lifecycle" 200 '.recommended_action == "collect_payment" and .payment_status == "pending" and .requires_action == true and .retry_recommended == false' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "collect-payment lifecycle" "X-API-Key: $ALPHA_READER_API_KEY"
pending_lifecycle_json="$HTTP_BODY"

invoice_id_enc="$(urlencode "$invoice_id")"
runs_url="$ALPHA_API_BASE_URL/v1/dunning/runs?invoice_id=$invoice_id_enc&active_only=true"
echo "[info] waiting for active dunning run to appear"
wait_for_get "$runs_url" 200 '(.items | length > 0) and .items[0].state == "awaiting_payment_setup" and .items[0].next_action_type == "collect_payment_reminder"' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "active dunning run" "X-API-Key: $ALPHA_READER_API_KEY"
dunning_run_list_json="$HTTP_BODY"
run_id="$(jq -r '.items[0].id // empty' <<<"$dunning_run_list_json")"
if [[ -z "$run_id" ]]; then
  echo "[fail] dunning run list missing run id" >&2
  exit 1
fi

echo "[info] waiting for scheduler-dispatched collect-payment reminder run_id=$run_id"
wait_for_get "$ALPHA_API_BASE_URL/v1/dunning/runs/$run_id" 200 '.run.state == "awaiting_payment_setup" and .run.next_action_type == "collect_payment_reminder" and (.notification_intents | length) > 0 and any(.notification_intents[]; .status == "dispatched") and any(.events[]; .event_type == "payment_setup_pending")' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "dispatched collect-payment reminder" "X-API-Key: $ALPHA_READER_API_KEY"
reminder_run_detail_json="$HTTP_BODY"
assert_jq "$reminder_run_detail_json" "expected payment method required reminder intent" 'any(.notification_intents[]; .intent_type == "dunning.payment_method_required" and .status == "dispatched")'

echo "[info] verifying payment and invoice detail surfaces show active dunning summary"
http_call GET "$ALPHA_API_BASE_URL/v1/payments/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get payment detail with dunning summary"
payment_detail_with_dunning_json="$HTTP_BODY"
assert_jq "$payment_detail_with_dunning_json" "payment detail missing active dunning summary" --arg run_id "$run_id" '.dunning.run_id == $run_id and .dunning.state == "awaiting_payment_setup" and .dunning.next_action_type == "collect_payment_reminder" and .dunning.last_notification_status == "dispatched"'

http_call GET "$ALPHA_API_BASE_URL/v1/invoices/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get invoice detail with dunning summary"
invoice_detail_with_dunning_json="$HTTP_BODY"
assert_jq "$invoice_detail_with_dunning_json" "invoice detail missing active dunning summary" --arg run_id "$run_id" '.dunning.run_id == $run_id and .dunning.state == "awaiting_payment_setup" and .dunning.next_action_type == "collect_payment_reminder" and .dunning.last_notification_status == "dispatched"'

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
assert_jq "$refresh_result_json" "refreshed readiness should be payment ready" '.readiness.status == "ready" and .readiness.payment_setup_status == "ready" and .readiness.default_payment_method_verified == true and .payment_setup.default_payment_method_present == true'

echo "[info] waiting for dunning run to observe payment setup readiness"
wait_for_get "$ALPHA_API_BASE_URL/v1/dunning/runs/$run_id" 200 'any(.events[]; .event_type == "payment_setup_ready") and (.run.state == "retry_due" or .run.state == "awaiting_retry_result" or .run.state == "resolved")' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "dunning transition into retry flow" "X-API-Key: $ALPHA_READER_API_KEY"
post_refresh_dunning_json="$HTTP_BODY"

echo "[info] waiting for scheduler-driven retry and dunning resolution"
wait_for_get "$ALPHA_API_BASE_URL/v1/dunning/runs/$run_id" 200 '.run.state == "resolved" and .run.resolution == "payment_succeeded" and any(.events[]; .event_type == "retry_attempted") and any(.events[]; .event_type == "retry_succeeded")' "$TIMEOUT_SEC" "$POLL_INTERVAL_SEC" "resolved dunning run after successful retry" "X-API-Key: $ALPHA_READER_API_KEY"
resolved_dunning_json="$HTTP_BODY"

http_call GET "$ALPHA_API_BASE_URL/v1/payments/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get final payment detail with resolved dunning summary"
final_payment_detail_json="$HTTP_BODY"
assert_jq "$final_payment_detail_json" "final payment detail should show resolved dunning summary and success" --arg run_id "$run_id" '.payment_status == "succeeded" and .lifecycle.recommended_action == "none" and .dunning.run_id == $run_id and .dunning.state == "resolved" and .dunning.resolution == "payment_succeeded" and .dunning.last_event_type == "retry_succeeded"'

http_call GET "$ALPHA_API_BASE_URL/v1/invoices/$invoice_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get final invoice detail with resolved dunning summary"
final_invoice_detail_json="$HTTP_BODY"
assert_jq "$final_invoice_detail_json" "final invoice detail should show resolved dunning summary" --arg run_id "$run_id" '.dunning.run_id == $run_id and .dunning.state == "resolved" and .dunning.resolution == "payment_succeeded" and .dunning.last_event_type == "retry_succeeded"'

summary_json="$(
jq -n \
  --arg run_id "$RUN_ID" \
  --arg tenant_id "$TARGET_TENANT_ID" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg customer_email "$CUSTOMER_EMAIL" \
  --arg invoice_id "$invoice_id" \
  --arg dunning_run_id "$run_id" \
  --arg payment_method_fixture "$PAYMENT_METHOD_FIXTURE" \
  --slurpfile bootstrap "$bootstrap_json_file" \
  --slurpfile fixture "$fixture_json_file" \
    --slurpfile reconcile "$reconcile_json_file" \
  --slurpfile detach "$detach_json_file" \
  --argjson policy "$dunning_policy_json" \
  --argjson initial_readiness "$initial_readiness_json" \
  --argjson refresh_result "$refresh_result_json" \
  --argjson run_list "$dunning_run_list_json" \
  --argjson reminder_detail "$reminder_run_detail_json" \
  --argjson post_refresh_detail "$post_refresh_dunning_json" \
  --argjson resolved_detail "$resolved_dunning_json" \
  --argjson initial_retry_response "$initial_retry_response_json" \
  --argjson pending_lifecycle "$pending_lifecycle_json" \
  --argjson payment_detail_active "$payment_detail_with_dunning_json" \
  --argjson invoice_detail_active "$invoice_detail_with_dunning_json" \
  --argjson payment_detail_final "$final_payment_detail_json" \
  --argjson invoice_detail_final "$final_invoice_detail_json" \
  '{
    run_id: $run_id,
    tenant_id: $tenant_id,
    customer: {
      external_id: $customer_external_id,
      email: $customer_email
    },
    invoice_id: $invoice_id,
    dunning_run_id: $dunning_run_id,
    payment_method_fixture: $payment_method_fixture,
    policy: $policy.policy,
    bootstrap: ($bootstrap[0] // null),
    fixture: ($fixture[0] // null),
    initial_readiness: $initial_readiness,
    initial_retry_response: $initial_retry_response,
    pending_lifecycle: $pending_lifecycle,
    active_run_list: $run_list,
    reminder_detail: $reminder_detail,
    initial_provider_detach: ($detach[0] // null),
    provider_completion: ($reconcile[0] // null),
    refresh_result: $refresh_result,
    post_refresh_detail: $post_refresh_detail,
    resolved_detail: $resolved_detail,
    payment_detail_active: $payment_detail_active,
    invoice_detail_active: $invoice_detail_active,
    payment_detail_final: $payment_detail_final,
    invoice_detail_final: $invoice_detail_final
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

echo "[pass] staging dunning journey completed run_id=$RUN_ID invoice_id=$invoice_id dunning_run_id=$run_id"
printf '%s\n' "$summary_json"
