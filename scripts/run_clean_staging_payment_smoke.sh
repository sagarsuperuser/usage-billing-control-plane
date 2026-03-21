#!/usr/bin/env bash
set -euo pipefail

required_cmds=(bash go jq mktemp)
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
  printf "%s" "$value"
}

ensure_alpha_customer() {
  local external_id="$1"
  local display_name="$2"
  local email="$3"
  local output
  if ! output="$(go run ./cmd/admin ensure-alpha-customers \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -writer-api-key "$ALPHA_WRITER_API_KEY" \
    -customer "$external_id|$display_name|$email")"; then
    echo "[fail] alpha customer bootstrap failed external_id=$external_id" >&2
    exit 1
  fi
  local outcome
  outcome="$(jq -r '.results[0].outcome // empty' <<<"$output")"
  case "$outcome" in
    created)
      echo "[pass] alpha customer created external_id=$external_id"
      ;;
    existing)
      echo "[info] alpha customer already exists external_id=$external_id"
      ;;
    *)
      echo "[fail] alpha customer bootstrap produced unexpected outcome external_id=$external_id output=$output" >&2
      exit 1
      ;;
  esac
}

ensure_tenant_lago_mapping() {
  local tenant_id="$1"
  local org_id="$2"
  local provider_code="$3"
  if ! go run ./cmd/admin ensure-tenant-lago-mapping \
    -alpha-api-base-url "$ALPHA_API_BASE_URL" \
    -platform-api-key "$PLATFORM_ADMIN_API_KEY" \
    -tenant-id "$tenant_id" \
    -organization-id "$org_id" \
    -provider-code "$provider_code" >/dev/null; then
    echo "[fail] alpha tenant lago mapping failed tenant_id=$tenant_id organization_id=$org_id provider_code=$provider_code" >&2
    exit 1
  fi
  echo "[pass] alpha tenant lago mapping ensured tenant_id=$tenant_id organization_id=$org_id provider_code=$provider_code"
}

require_env ALPHA_API_BASE_URL
require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY
require_env PLATFORM_ADMIN_API_KEY
require_env LAGO_API_URL
require_env LAGO_API_KEY

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
SUCCESS_CUSTOMER_EXTERNAL_ID="${SUCCESS_CUSTOMER_EXTERNAL_ID:-cust_payment_smoke_success_${RUN_ID}}"
FAILURE_CUSTOMER_EXTERNAL_ID="${FAILURE_CUSTOMER_EXTERNAL_ID:-cust_payment_smoke_failure_${RUN_ID}}"
SUCCESS_ADD_ON_CODE="${SUCCESS_ADD_ON_CODE:-alpha-real-payment-fixture-success-${RUN_ID}}"
FAILURE_ADD_ON_CODE="${FAILURE_ADD_ON_CODE:-alpha-real-payment-fixture-failure-${RUN_ID}}"
SUCCESS_CUSTOMER_EMAIL="${SUCCESS_CUSTOMER_EMAIL:-billing+${RUN_ID}-success@alpha.test}"
FAILURE_CUSTOMER_EMAIL="${FAILURE_CUSTOMER_EMAIL:-billing+${RUN_ID}-failure@alpha.test}"
TIMEOUT_SEC="${TIMEOUT_SEC:-600}"
POLL_INTERVAL_SEC="${POLL_INTERVAL_SEC:-5}"
TARGET_TENANT_ID="${TARGET_TENANT_ID:-default}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"

bootstrap_json_file="$(mktemp)"
success_fixture_json_file="$(mktemp)"
failure_fixture_json_file="$(mktemp)"
success_result_json_file="$(mktemp)"
failure_result_json_file="$(mktemp)"
trap 'rm -f "$bootstrap_json_file" "$success_fixture_json_file" "$failure_fixture_json_file" "$success_result_json_file" "$failure_result_json_file"' EXIT

echo "[info] bootstrapping clean payment smoke customers run_id=$RUN_ID"
RUN_ID="$RUN_ID" \
SUCCESS_CUSTOMER_EXTERNAL_ID="$SUCCESS_CUSTOMER_EXTERNAL_ID" \
FAILURE_CUSTOMER_EXTERNAL_ID="$FAILURE_CUSTOMER_EXTERNAL_ID" \
LAGO_WEBHOOK_URL="${LAGO_WEBHOOK_URL:-$ALPHA_API_BASE_URL/internal/lago/webhooks}" \
bash ./scripts/bootstrap_lago_stripe_staging.sh >"$bootstrap_json_file"

bootstrap_org_id="$(jq -r '.organization.id // empty' "$bootstrap_json_file")"
bootstrap_provider_code="$(jq -r '.stripe_provider.code // empty' "$bootstrap_json_file")"
if [[ -z "$bootstrap_org_id" || -z "$bootstrap_provider_code" ]]; then
  echo "[fail] bootstrap did not produce organization.id and stripe_provider.code" >&2
  exit 1
fi

echo "[info] ensuring tenant billing mapping for lago organization"
ensure_tenant_lago_mapping "$TARGET_TENANT_ID" "$bootstrap_org_id" "$bootstrap_provider_code"

echo "[info] ensuring matching alpha customers exist for payment projection"
ensure_alpha_customer "$SUCCESS_CUSTOMER_EXTERNAL_ID" "Payment Smoke Success ${RUN_ID}" "$SUCCESS_CUSTOMER_EMAIL"
ensure_alpha_customer "$FAILURE_CUSTOMER_EXTERNAL_ID" "Payment Smoke Failure ${RUN_ID}" "$FAILURE_CUSTOMER_EMAIL"

echo "[info] preparing success invoice fixture customer=$SUCCESS_CUSTOMER_EXTERNAL_ID"
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
CUSTOMER_EXTERNAL_ID="$SUCCESS_CUSTOMER_EXTERNAL_ID" \
ADD_ON_CODE="$SUCCESS_ADD_ON_CODE" \
OUTPUT_FILE="$success_fixture_json_file" \
bash ./scripts/prepare_real_payment_invoice_fixture.sh
success_invoice_id="$(jq -r '.invoice_id // empty' "$success_fixture_json_file")"
if [[ -z "$success_invoice_id" ]]; then
  echo "[fail] success fixture did not produce invoice_id" >&2
  exit 1
fi

echo "[info] preparing failure invoice fixture customer=$FAILURE_CUSTOMER_EXTERNAL_ID"
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
CUSTOMER_EXTERNAL_ID="$FAILURE_CUSTOMER_EXTERNAL_ID" \
ADD_ON_CODE="$FAILURE_ADD_ON_CODE" \
OUTPUT_FILE="$failure_fixture_json_file" \
bash ./scripts/prepare_real_payment_invoice_fixture.sh
failure_invoice_id="$(jq -r '.invoice_id // empty' "$failure_fixture_json_file")"
if [[ -z "$failure_invoice_id" ]]; then
  echo "[fail] failure fixture did not produce invoice_id" >&2
  exit 1
fi

echo "[info] running success payment smoke invoice=$success_invoice_id"
ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
ALPHA_WRITER_API_KEY="$ALPHA_WRITER_API_KEY" \
ALPHA_READER_API_KEY="$ALPHA_READER_API_KEY" \
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
INVOICE_ID="$success_invoice_id" \
EXPECTED_FINAL_STATUS="succeeded" \
TIMEOUT_SEC="$TIMEOUT_SEC" \
POLL_INTERVAL_SEC="$POLL_INTERVAL_SEC" \
OUTPUT_FILE="$success_result_json_file" \
bash ./scripts/test_real_payment_e2e.sh

echo "[info] running failure payment smoke invoice=$failure_invoice_id"
ALPHA_API_BASE_URL="$ALPHA_API_BASE_URL" \
ALPHA_WRITER_API_KEY="$ALPHA_WRITER_API_KEY" \
ALPHA_READER_API_KEY="$ALPHA_READER_API_KEY" \
LAGO_API_URL="$LAGO_API_URL" \
LAGO_API_KEY="$LAGO_API_KEY" \
INVOICE_ID="$failure_invoice_id" \
EXPECTED_FINAL_STATUS="failed" \
EXPECTED_LIFECYCLE_ACTION="collect_payment" \
EXPECTED_LIFECYCLE_REQUIRES_ACTION="true" \
EXPECTED_LIFECYCLE_RETRY_RECOMMENDED="false" \
TIMEOUT_SEC="$TIMEOUT_SEC" \
POLL_INTERVAL_SEC="$POLL_INTERVAL_SEC" \
OUTPUT_FILE="$failure_result_json_file" \
bash ./scripts/test_real_payment_e2e.sh

summary_json="$(
jq -n \
  --arg run_id "$RUN_ID" \
  --arg success_customer_external_id "$SUCCESS_CUSTOMER_EXTERNAL_ID" \
  --arg failure_customer_external_id "$FAILURE_CUSTOMER_EXTERNAL_ID" \
  --arg success_invoice_id "$success_invoice_id" \
  --arg failure_invoice_id "$failure_invoice_id" \
  --arg success_customer_email "$SUCCESS_CUSTOMER_EMAIL" \
  --arg failure_customer_email "$FAILURE_CUSTOMER_EMAIL" \
  --slurpfile bootstrap "$bootstrap_json_file" \
  --slurpfile success_fixture "$success_fixture_json_file" \
  --slurpfile failure_fixture "$failure_fixture_json_file" \
  --slurpfile success_result "$success_result_json_file" \
  --slurpfile failure_result "$failure_result_json_file" \
  '{
    run_id: $run_id,
    fixture_source: "clean_staging_payment_smoke",
    execution_model: {
      cleanup: "explicit cluster cleanup command",
      bootstrap: "dedicated lago payment bootstrap job",
      fixture_ids: "per-run",
      alpha_customer_mapping: "explicit per-run alpha customer bootstrap",
      tenant_lago_mapping: "explicit platform patch to tenant billing mapping"
    },
    customers: {
      success_external_id: $success_customer_external_id,
      failure_external_id: $failure_customer_external_id,
      success_email: $success_customer_email,
      failure_email: $failure_customer_email
    },
    invoices: {
      success_invoice_id: $success_invoice_id,
      failure_invoice_id: $failure_invoice_id
    },
    bootstrap: ($bootstrap[0] // null),
    fixtures: {
      success: ($success_fixture[0] // null),
      failure: ($failure_fixture[0] // null)
    },
    payment_e2e: {
      success: ($success_result[0] // null),
      failure: ($failure_result[0] // null)
    }
  }'
)"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
