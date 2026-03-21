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

require_env ALPHA_API_BASE_URL
require_env ALPHA_WRITER_API_KEY
require_env ALPHA_READER_API_KEY

RUN_ID="${RUN_ID:-$(date -u +%Y%m%d%H%M%S)-$RANDOM}"
METRIC_KEY="${METRIC_KEY:-pricing_journey_metric_${RUN_ID}}"
METRIC_NAME="${METRIC_NAME:-Pricing Journey Metric ${RUN_ID}}"
METRIC_UNIT="${METRIC_UNIT:-event}"
METRIC_AGGREGATION="${METRIC_AGGREGATION:-sum}"
CURRENCY="${CURRENCY:-USD}"
PLAN_CODE="${PLAN_CODE:-pricing_journey_plan_${RUN_ID}}"
PLAN_NAME="${PLAN_NAME:-Pricing Journey Plan ${RUN_ID}}"
PLAN_DESCRIPTION="${PLAN_DESCRIPTION:-Pricing journey plan for run ${RUN_ID}}"
BILLING_INTERVAL="${BILLING_INTERVAL:-monthly}"
PLAN_STATUS="${PLAN_STATUS:-active}"
BASE_AMOUNT_CENTS="${BASE_AMOUNT_CENTS:-4900}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

ALPHA_API_BASE_URL="$(trim_trailing_slash "$ALPHA_API_BASE_URL")"
EXPECTED_RULE_KEY="${METRIC_KEY}_default"
EXPECTED_RULE_NAME="${METRIC_NAME} default rule"

create_metric_payload="$(jq -nc \
  --arg key "$METRIC_KEY" \
  --arg name "$METRIC_NAME" \
  --arg unit "$METRIC_UNIT" \
  --arg aggregation "$METRIC_AGGREGATION" \
  --arg currency "$CURRENCY" \
  '{key: $key, name: $name, unit: $unit, aggregation: $aggregation, currency: $currency}')"

echo "[info] creating pricing metric key=$METRIC_KEY"
http_call POST "$ALPHA_API_BASE_URL/v1/pricing/metrics" "$create_metric_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create pricing metric"
metric_json="$HTTP_BODY"
metric_id="$(jq -r '.id // empty' <<<"$metric_json")"
rating_rule_version_id="$(jq -r '.rating_rule_version_id // empty' <<<"$metric_json")"
if [[ -z "$metric_id" || -z "$rating_rule_version_id" ]]; then
  echo "[fail] create pricing metric returned missing id or rating_rule_version_id body=$metric_json" >&2
  exit 1
fi
assert_jq "$metric_json" "pricing metric create response mismatch" \
  --arg id "$metric_id" \
  --arg key "$METRIC_KEY" \
  --arg name "$METRIC_NAME" \
  --arg unit "$METRIC_UNIT" \
  --arg aggregation "$METRIC_AGGREGATION" \
  --arg rule_id "$rating_rule_version_id" \
  '.id == $id and .key == $key and .name == $name and .unit == $unit and .aggregation == $aggregation and .rating_rule_version_id == $rule_id'

echo "[info] verifying pricing metric detail id=$metric_id"
http_call GET "$ALPHA_API_BASE_URL/v1/pricing/metrics/$metric_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get pricing metric"
metric_detail_json="$HTTP_BODY"
assert_jq "$metric_detail_json" "pricing metric detail mismatch" \
  --arg id "$metric_id" \
  --arg key "$METRIC_KEY" \
  --arg unit "$METRIC_UNIT" \
  --arg aggregation "$METRIC_AGGREGATION" \
  --arg rule_id "$rating_rule_version_id" \
  '.id == $id and .key == $key and .unit == $unit and .aggregation == $aggregation and .rating_rule_version_id == $rule_id'

echo "[info] verifying pricing metric listing contains created metric"
http_call GET "$ALPHA_API_BASE_URL/v1/pricing/metrics" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "list pricing metrics"
metrics_list_json="$HTTP_BODY"
assert_jq "$metrics_list_json" "pricing metrics list missing created metric" --arg id "$metric_id" 'map(select(.id == $id)) | length == 1'

echo "[info] verifying generated default rating rule id=$rating_rule_version_id"
http_call GET "$ALPHA_API_BASE_URL/v1/rating-rules/$rating_rule_version_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get generated rating rule"
rating_rule_json="$HTTP_BODY"
assert_jq "$rating_rule_json" "generated rating rule mismatch" \
  --arg id "$rating_rule_version_id" \
  --arg rule_key "$EXPECTED_RULE_KEY" \
  --arg name "$EXPECTED_RULE_NAME" \
  --arg currency "$CURRENCY" \
  '.id == $id and .rule_key == $rule_key and .name == $name and .version == 1 and .lifecycle_state == "draft" and .mode == "flat" and .currency == $currency and .flat_amount_cents == 0'

echo "[info] verifying rule listing by rule_key returns generated rule"
http_call GET "$ALPHA_API_BASE_URL/v1/rating-rules?rule_key=$EXPECTED_RULE_KEY&latest_only=true" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "list generated rating rule"
rating_rule_list_json="$HTTP_BODY"
assert_jq "$rating_rule_list_json" "generated rating rule list missing created rule" --arg id "$rating_rule_version_id" 'map(select(.id == $id)) | length == 1'

create_plan_payload="$(jq -nc \
  --arg code "$PLAN_CODE" \
  --arg name "$PLAN_NAME" \
  --arg description "$PLAN_DESCRIPTION" \
  --arg currency "$CURRENCY" \
  --arg billing_interval "$BILLING_INTERVAL" \
  --arg status "$PLAN_STATUS" \
  --argjson base_amount_cents "$BASE_AMOUNT_CENTS" \
  --arg meter_id "$metric_id" \
  '{code: $code, name: $name, description: $description, currency: $currency, billing_interval: $billing_interval, status: $status, base_amount_cents: $base_amount_cents, meter_ids: [$meter_id]}')"

echo "[info] creating plan code=$PLAN_CODE"
http_call POST "$ALPHA_API_BASE_URL/v1/plans" "$create_plan_payload" "X-API-Key: $ALPHA_WRITER_API_KEY"
assert_http_code 201 "create plan"
plan_json="$HTTP_BODY"
plan_id="$(jq -r '.id // empty' <<<"$plan_json")"
if [[ -z "$plan_id" ]]; then
  echo "[fail] create plan returned missing id body=$plan_json" >&2
  exit 1
fi
assert_jq "$plan_json" "plan create response mismatch" \
  --arg id "$plan_id" \
  --arg code "$PLAN_CODE" \
  --arg name "$PLAN_NAME" \
  --arg currency "$CURRENCY" \
  --arg billing_interval "$BILLING_INTERVAL" \
  --arg status "$PLAN_STATUS" \
  --arg meter_id "$metric_id" \
  --argjson base_amount_cents "$BASE_AMOUNT_CENTS" \
  '.id == $id and .code == $code and .name == $name and .currency == $currency and .billing_interval == $billing_interval and .status == $status and .base_amount_cents == $base_amount_cents and .meter_ids == [$meter_id]'

echo "[info] verifying plan detail id=$plan_id"
http_call GET "$ALPHA_API_BASE_URL/v1/plans/$plan_id" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "get plan"
plan_detail_json="$HTTP_BODY"
assert_jq "$plan_detail_json" "plan detail mismatch" \
  --arg id "$plan_id" \
  --arg code "$PLAN_CODE" \
  --arg meter_id "$metric_id" \
  '.id == $id and .code == $code and (.meter_ids | index($meter_id) != null) and (.meter_ids | length == 1)'

echo "[info] verifying plan listing contains created plan"
http_call GET "$ALPHA_API_BASE_URL/v1/plans" '' "X-API-Key: $ALPHA_READER_API_KEY"
assert_http_code 200 "list plans"
plans_list_json="$HTTP_BODY"
assert_jq "$plans_list_json" "plan list missing created plan" --arg id "$plan_id" 'map(select(.id == $id)) | length == 1'

summary_json="$(jq -n \
  --arg run_id "$RUN_ID" \
  --arg alpha_api_base_url "$ALPHA_API_BASE_URL" \
  --arg metric_id "$metric_id" \
  --arg metric_key "$METRIC_KEY" \
  --arg metric_name "$METRIC_NAME" \
  --arg metric_unit "$METRIC_UNIT" \
  --arg metric_aggregation "$METRIC_AGGREGATION" \
  --arg rating_rule_version_id "$rating_rule_version_id" \
  --arg expected_rule_key "$EXPECTED_RULE_KEY" \
  --arg expected_rule_name "$EXPECTED_RULE_NAME" \
  --arg plan_id "$plan_id" \
  --arg plan_code "$PLAN_CODE" \
  --arg plan_name "$PLAN_NAME" \
  --arg plan_description "$PLAN_DESCRIPTION" \
  --arg currency "$CURRENCY" \
  --arg billing_interval "$BILLING_INTERVAL" \
  --arg status "$PLAN_STATUS" \
  --argjson base_amount_cents "$BASE_AMOUNT_CENTS" \
  --argjson metric_create "$metric_json" \
  --argjson metric_detail "$metric_detail_json" \
  --argjson metric_list "$metrics_list_json" \
  --argjson rating_rule "$rating_rule_json" \
  --argjson rating_rule_list "$rating_rule_list_json" \
  --argjson plan_create "$plan_json" \
  --argjson plan_detail "$plan_detail_json" \
  --argjson plan_list "$plans_list_json" \
  '{
    run_id: $run_id,
    fixture_source: "staging_pricing_journey",
    alpha_api_base_url: $alpha_api_base_url,
    journey: {
      metric: {
        id: $metric_id,
        key: $metric_key,
        name: $metric_name,
        unit: $metric_unit,
        aggregation: $metric_aggregation,
        rating_rule_version_id: $rating_rule_version_id
      },
      generated_rating_rule: {
        id: $rating_rule_version_id,
        rule_key: $expected_rule_key,
        name: $expected_rule_name,
        currency: $currency,
        mode: "flat",
        lifecycle_state: "draft"
      },
      plan: {
        id: $plan_id,
        code: $plan_code,
        name: $plan_name,
        description: $plan_description,
        currency: $currency,
        billing_interval: $billing_interval,
        status: $status,
        base_amount_cents: $base_amount_cents,
        meter_ids: [$metric_id]
      }
    },
    verification: {
      metric_create: $metric_create,
      metric_detail: $metric_detail,
      metric_list_count: ($metric_list | length),
      generated_rating_rule: $rating_rule,
      generated_rating_rule_list_count: ($rating_rule_list | length),
      plan_create: $plan_create,
      plan_detail: $plan_detail,
      plan_list_count: ($plan_list | length)
    }
  }')"

if [[ -n "$OUTPUT_FILE" ]]; then
  printf '%s\n' "$summary_json" >"$OUTPUT_FILE"
fi

printf '%s\n' "$summary_json"
