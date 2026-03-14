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

urlencode() {
  jq -nr --arg v "$1" '$v|@uri'
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

require_env LAGO_API_URL
require_env LAGO_API_KEY
require_env CUSTOMER_EXTERNAL_ID

ADD_ON_CODE="${ADD_ON_CODE:-alpha-real-payment-fixture}"
ADD_ON_NAME="${ADD_ON_NAME:-Alpha Real Payment Fixture}"
INVOICE_DISPLAY_NAME="${INVOICE_DISPLAY_NAME:-Alpha Real Payment Fixture Charge}"
DESCRIPTION="${DESCRIPTION:-Auto-generated fixture charge for real payment E2E}"
UNIT_AMOUNT_CENTS="${UNIT_AMOUNT_CENTS:-199}"
UNITS="${UNITS:-1}"
CURRENCY="${CURRENCY:-}"
FINALIZE_INVOICE="${FINALIZE_INVOICE:-1}"
REQUIRE_STRIPE_BILLING_CONFIG="${REQUIRE_STRIPE_BILLING_CONFIG:-1}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

if ! [[ "$UNIT_AMOUNT_CENTS" =~ ^[0-9]+$ ]] || [[ "$UNIT_AMOUNT_CENTS" -le 0 ]]; then
  echo "UNIT_AMOUNT_CENTS must be a positive integer" >&2
  exit 1
fi
if ! [[ "$UNITS" =~ ^[0-9]+$ ]] || [[ "$UNITS" -le 0 ]]; then
  echo "UNITS must be a positive integer" >&2
  exit 1
fi
if [[ "$FINALIZE_INVOICE" != "0" && "$FINALIZE_INVOICE" != "1" ]]; then
  echo "FINALIZE_INVOICE must be 0 or 1" >&2
  exit 1
fi
if [[ "$REQUIRE_STRIPE_BILLING_CONFIG" != "0" && "$REQUIRE_STRIPE_BILLING_CONFIG" != "1" ]]; then
  echo "REQUIRE_STRIPE_BILLING_CONFIG must be 0 or 1" >&2
  exit 1
fi

LAGO_API_URL="$(trim_trailing_slash "$LAGO_API_URL")"
CUSTOMER_EXTERNAL_ID="$(echo "$CUSTOMER_EXTERNAL_ID" | xargs)"
ADD_ON_CODE="$(echo "$ADD_ON_CODE" | xargs)"

if [[ -z "$CUSTOMER_EXTERNAL_ID" || -z "$ADD_ON_CODE" ]]; then
  echo "CUSTOMER_EXTERNAL_ID and ADD_ON_CODE must not be empty" >&2
  exit 1
fi

customer_external_id_enc="$(urlencode "$CUSTOMER_EXTERNAL_ID")"
add_on_code_enc="$(urlencode "$ADD_ON_CODE")"

echo "[info] fetching Lago customer external_id=$CUSTOMER_EXTERNAL_ID"
http_call "GET" "$LAGO_API_URL/api/v1/customers/$customer_external_id_enc" "" "Authorization: Bearer $LAGO_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] customer lookup failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

customer_currency="$(jq -r '.customer.currency // empty' <<<"$HTTP_BODY")"
billing_provider="$(jq -r '.customer.billing_configuration.payment_provider // .customer.payment_provider // empty' <<<"$HTTP_BODY")"
billing_provider_normalized="$(printf '%s' "$billing_provider" | tr '[:upper:]' '[:lower:]')"

if [[ "$REQUIRE_STRIPE_BILLING_CONFIG" == "1" ]]; then
  if [[ "$billing_provider_normalized" != "stripe" ]]; then
    echo "[fail] customer billing provider is not stripe (provider=${billing_provider:-<empty>})" >&2
    echo "       configure customer billing_configuration.payment_provider=stripe before real payment fixture prep" >&2
    exit 1
  fi
fi

if [[ -z "$CURRENCY" ]]; then
  CURRENCY="$customer_currency"
fi
if [[ -z "$CURRENCY" ]]; then
  CURRENCY="USD"
fi

echo "[info] ensuring add-on code=$ADD_ON_CODE"
http_call "GET" "$LAGO_API_URL/api/v1/add_ons/$add_on_code_enc" "" "Authorization: Bearer $LAGO_API_KEY"
if [[ "$HTTP_CODE" == "200" ]]; then
  echo "[pass] add-on exists"
elif [[ "$HTTP_CODE" == "404" ]]; then
  add_on_payload="$(jq -nc \
    --arg code "$ADD_ON_CODE" \
    --arg name "$ADD_ON_NAME" \
    --arg invoice_display_name "$INVOICE_DISPLAY_NAME" \
    --arg description "$DESCRIPTION" \
    --arg currency "$CURRENCY" \
    --argjson amount_cents "$UNIT_AMOUNT_CENTS" \
    '{
      add_on: {
        code: $code,
        name: $name,
        invoice_display_name: $invoice_display_name,
        amount_cents: $amount_cents,
        amount_currency: $currency,
        description: $description
      }
    }')"
  http_call "POST" "$LAGO_API_URL/api/v1/add_ons" "$add_on_payload" "Authorization: Bearer $LAGO_API_KEY"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[fail] add-on create failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
    exit 1
  fi
  echo "[pass] add-on created"
else
  echo "[fail] add-on lookup failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

echo "[info] creating one-off invoice fixture"
invoice_create_payload="$(jq -nc \
  --arg external_customer_id "$CUSTOMER_EXTERNAL_ID" \
  --arg currency "$CURRENCY" \
  --arg add_on_code "$ADD_ON_CODE" \
  --arg description "$DESCRIPTION" \
  --arg invoice_display_name "$INVOICE_DISPLAY_NAME" \
  --argjson unit_amount_cents "$UNIT_AMOUNT_CENTS" \
  --argjson units "$UNITS" \
  '{
    invoice: {
      external_customer_id: $external_customer_id,
      currency: $currency,
      fees: [
        {
          add_on_code: $add_on_code,
          unit_amount_cents: $unit_amount_cents,
          units: $units,
          description: $description,
          invoice_display_name: $invoice_display_name
        }
      ]
    }
  }')"

http_call "POST" "$LAGO_API_URL/api/v1/invoices" "$invoice_create_payload" "Authorization: Bearer $LAGO_API_KEY"
if [[ "$HTTP_CODE" != "200" ]]; then
  echo "[fail] invoice create failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
  exit 1
fi

invoice_id="$(jq -r '.invoice.lago_id // empty' <<<"$HTTP_BODY")"
invoice_status="$(jq -r '.invoice.status // empty' <<<"$HTTP_BODY")"
invoice_payment_status="$(jq -r '.invoice.payment_status // empty' <<<"$HTTP_BODY")"
if [[ -z "$invoice_id" ]]; then
  echo "[fail] invoice create response missing invoice.lago_id: $HTTP_BODY" >&2
  exit 1
fi

if [[ "$FINALIZE_INVOICE" == "1" && "$invoice_status" == "draft" ]]; then
  invoice_id_enc="$(urlencode "$invoice_id")"
  echo "[info] finalizing draft invoice id=$invoice_id"
  http_call "PUT" "$LAGO_API_URL/api/v1/invoices/$invoice_id_enc/finalize" "" "Authorization: Bearer $LAGO_API_KEY"
  if [[ "$HTTP_CODE" != "200" ]]; then
    echo "[fail] invoice finalize failed: status=$HTTP_CODE body=$HTTP_BODY" >&2
    exit 1
  fi
  invoice_status="$(jq -r '.invoice.status // empty' <<<"$HTTP_BODY")"
  invoice_payment_status="$(jq -r '.invoice.payment_status // empty' <<<"$HTTP_BODY")"
fi

result_json="$(jq -nc \
  --arg invoice_id "$invoice_id" \
  --arg invoice_status "$invoice_status" \
  --arg invoice_payment_status "$invoice_payment_status" \
  --arg customer_external_id "$CUSTOMER_EXTERNAL_ID" \
  --arg add_on_code "$ADD_ON_CODE" \
  --arg currency "$CURRENCY" \
  --argjson unit_amount_cents "$UNIT_AMOUNT_CENTS" \
  --argjson units "$UNITS" \
  '{
    invoice_id: $invoice_id,
    invoice_status: $invoice_status,
    invoice_payment_status: $invoice_payment_status,
    customer_external_id: $customer_external_id,
    add_on_code: $add_on_code,
    currency: $currency,
    unit_amount_cents: $unit_amount_cents,
    units: $units
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$result_json" >"$OUTPUT_FILE"
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'invoice_id=%s\n' "$invoice_id" >>"$GITHUB_OUTPUT"
fi

echo "[pass] prepared invoice fixture id=$invoice_id status=$invoice_status payment_status=$invoice_payment_status"
printf '%s\n' "$result_json"
