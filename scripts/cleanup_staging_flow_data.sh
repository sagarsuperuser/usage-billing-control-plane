#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    echo "missing required environment variable: $key" >&2
    exit 1
  fi
}

require_cmd psql
require_env DATABASE_URL

ENVIRONMENT="${ENVIRONMENT:-staging}"
CONFIRM_STAGING_FLOW_CLEANUP="${CONFIRM_STAGING_FLOW_CLEANUP:-}"
APPLY="${APPLY:-0}"

if [[ "$ENVIRONMENT" != "staging" ]]; then
  echo "cleanup_staging_flow_data.sh is restricted to ENVIRONMENT=staging" >&2
  exit 1
fi

if [[ "$APPLY" != "0" && "$APPLY" != "1" ]]; then
  echo "APPLY must be 0 or 1" >&2
  exit 1
fi

if [[ "$APPLY" == "1" && "$CONFIRM_STAGING_FLOW_CLEANUP" != "YES_I_UNDERSTAND" ]]; then
  echo "set CONFIRM_STAGING_FLOW_CLEANUP=YES_I_UNDERSTAND to apply staging cleanup" >&2
  exit 1
fi

read -r -d '' REPORT_SQL <<'SQL' || true
WITH
  replay_customers AS (
    SELECT external_id
    FROM customers
    WHERE external_id LIKE 'cust_replay_smoke_%'
  ),
  payment_fixture_customers AS (
    SELECT id, external_id
    FROM customers
    WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
       OR external_id LIKE 'cust_payment_smoke_%'
  ),
  replay_meters AS (
    SELECT id, meter_key
    FROM meters
    WHERE meter_key LIKE 'replay_smoke_meter_%'
  ),
  replay_rules AS (
    SELECT id, name
    FROM rating_rule_versions
    WHERE name LIKE 'Replay Smoke Flat %'
  ),
  live_platform_keys AS (
    SELECT id, name
    FROM platform_api_keys
    WHERE name LIKE 'playwright-live-%'
  ),
  live_tenant_keys AS (
    SELECT id, name
    FROM api_keys
    WHERE name LIKE 'playwright-live-%'
  ),
  live_users AS (
    SELECT id, email
    FROM users
    WHERE email LIKE 'playwright-live-%@alpha.test'
  )
SELECT jsonb_pretty(
  jsonb_build_object(
    'environment', 'staging',
    'replay_smoke_customers', (SELECT count(*) FROM replay_customers),
    'replay_smoke_customer_billing_profiles', (
      SELECT count(*)
      FROM customer_billing_profiles
      WHERE customer_id IN (SELECT id FROM customers WHERE external_id IN (SELECT external_id FROM replay_customers))
    ),
    'replay_smoke_customer_payment_setup', (
      SELECT count(*)
      FROM customer_payment_setup
      WHERE customer_id IN (SELECT id FROM customers WHERE external_id IN (SELECT external_id FROM replay_customers))
    ),
    'replay_smoke_usage_events', (
      SELECT count(*) FROM usage_events
      WHERE customer_id IN (SELECT external_id FROM replay_customers)
    ),
    'replay_smoke_billed_entries', (
      SELECT count(*) FROM billed_entries
      WHERE customer_id IN (SELECT external_id FROM replay_customers)
    ),
    'replay_smoke_replay_jobs', (
      SELECT count(*)
      FROM replay_jobs
      WHERE customer_id IN (SELECT external_id FROM replay_customers)
         OR meter_id IN (SELECT id FROM replay_meters)
         OR idempotency_key LIKE 'replay-smoke-%'
    ),
    'replay_smoke_meters', (SELECT count(*) FROM replay_meters),
    'replay_smoke_rating_rule_versions', (SELECT count(*) FROM replay_rules),
    'payment_smoke_customers', (SELECT count(*) FROM payment_fixture_customers),
    'payment_smoke_customer_billing_profiles', (
      SELECT count(*)
      FROM customer_billing_profiles
      WHERE customer_id IN (SELECT id FROM payment_fixture_customers)
    ),
    'payment_smoke_customer_payment_setup', (
      SELECT count(*)
      FROM customer_payment_setup
      WHERE customer_id IN (SELECT id FROM payment_fixture_customers)
    ),
    'payment_smoke_invoice_payment_status_views', (
      SELECT count(*)
      FROM invoice_payment_status_views
      WHERE customer_external_id IN (SELECT external_id FROM payment_fixture_customers)
    ),
    'payment_smoke_lago_webhook_events', (
      SELECT count(*)
      FROM lago_webhook_events
      WHERE customer_external_id IN (SELECT external_id FROM payment_fixture_customers)
    ),
    'playwright_live_platform_api_keys', (SELECT count(*) FROM live_platform_keys),
    'playwright_live_tenant_api_keys', (SELECT count(*) FROM live_tenant_keys),
    'playwright_live_api_key_audit_events', (
      SELECT count(*)
      FROM api_key_audit_events
      WHERE api_key_id IN (
        SELECT id FROM live_tenant_keys
        UNION ALL
        SELECT id FROM live_platform_keys
      )
    ),
    'playwright_live_api_key_export_jobs', (
      SELECT count(*)
      FROM api_key_audit_export_jobs
      WHERE requested_by_api_key_id IN (
        SELECT id FROM live_tenant_keys
      )
    ),
    'playwright_live_users', (SELECT count(*) FROM live_users),
    'playwright_live_memberships', (
      SELECT count(*)
      FROM user_tenant_memberships
      WHERE user_id IN (SELECT id FROM live_users)
    ),
    'playwright_live_password_credentials', (
      SELECT count(*)
      FROM user_password_credentials
      WHERE user_id IN (SELECT id FROM live_users)
    ),
    'playwright_live_password_reset_tokens', (
      SELECT count(*)
      FROM password_reset_tokens
      WHERE user_id IN (SELECT id FROM live_users)
    ),
    'playwright_live_workspace_invitations', (
      SELECT count(*)
      FROM workspace_invitations
      WHERE lower(email) LIKE 'playwright-live-%@alpha.test'
    )
  )
);
SQL

read -r -d '' APPLY_SQL <<'SQL' || true
BEGIN;

WITH live_platform_keys AS (
  SELECT id FROM platform_api_keys WHERE name LIKE 'playwright-live-%'
),
live_tenant_keys AS (
  SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%'
),
replay_customers AS (
  SELECT id, external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
),
payment_fixture_customers AS (
  SELECT id, external_id
  FROM customers
  WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
     OR external_id LIKE 'cust_payment_smoke_%'
),
replay_meters AS (
  SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%'
),
replay_rules AS (
  SELECT id FROM rating_rule_versions WHERE name LIKE 'Replay Smoke Flat %'
)
DELETE FROM api_key_audit_events
WHERE api_key_id IN (
  SELECT id FROM live_tenant_keys
  UNION ALL
  SELECT id FROM live_platform_keys
);

WITH live_platform_keys AS (
  SELECT id FROM platform_api_keys WHERE name LIKE 'playwright-live-%'
),
live_tenant_keys AS (
  SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%'
)
DELETE FROM api_key_audit_export_jobs
WHERE requested_by_api_key_id IN (
  SELECT id FROM live_tenant_keys
);

DELETE FROM platform_api_keys WHERE name LIKE 'playwright-live-%';
DELETE FROM api_keys WHERE name LIKE 'playwright-live-%';
DELETE FROM workspace_invitations WHERE lower(email) LIKE 'playwright-live-%@alpha.test';
DELETE FROM users WHERE lower(email) LIKE 'playwright-live-%@alpha.test';

DELETE FROM lago_webhook_events
WHERE customer_external_id IN (
  SELECT external_id
  FROM customers
  WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
     OR external_id LIKE 'cust_payment_smoke_%'
);

DELETE FROM invoice_payment_status_views
WHERE customer_external_id IN (
  SELECT external_id
  FROM customers
  WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
     OR external_id LIKE 'cust_payment_smoke_%'
);

DELETE FROM replay_jobs
WHERE customer_id IN (
    SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
  )
   OR meter_id IN (
    SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%'
  )
   OR idempotency_key LIKE 'replay-smoke-%';

DELETE FROM billed_entries
WHERE customer_id IN (
    SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
  )
   OR meter_id IN (
    SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%'
  )
   OR idempotency_key LIKE 'replay-smoke-%';

DELETE FROM usage_events
WHERE customer_id IN (
    SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
  )
   OR meter_id IN (
    SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%'
  )
   OR idempotency_key LIKE 'replay-smoke-%';

DELETE FROM customer_payment_setup
WHERE customer_id IN (
  SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
);

DELETE FROM customer_payment_setup
WHERE customer_id IN (
  SELECT id
  FROM customers
  WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
     OR external_id LIKE 'cust_payment_smoke_%'
);

DELETE FROM customer_billing_profiles
WHERE customer_id IN (
  SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'
);

DELETE FROM customer_billing_profiles
WHERE customer_id IN (
  SELECT id
  FROM customers
  WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
     OR external_id LIKE 'cust_payment_smoke_%'
);

DELETE FROM customers
WHERE external_id LIKE 'cust_replay_smoke_%';

DELETE FROM customers
WHERE external_id IN ('cust_e2e_success', 'cust_e2e_failure')
   OR external_id LIKE 'cust_payment_smoke_%';

DELETE FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%';
DELETE FROM rating_rule_versions WHERE name LIKE 'Replay Smoke Flat %';

COMMIT;
SQL

echo "[info] staging flow-data cleanup report"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -qAt -c "$REPORT_SQL"

if [[ "$APPLY" == "1" ]]; then
  echo "[info] applying staging flow-data cleanup"
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -X -q -c "$APPLY_SQL"
  echo "[pass] staging flow-data cleanup applied"
else
  echo "[info] dry run only; export APPLY=1 CONFIRM_STAGING_FLOW_CLEANUP=YES_I_UNDERSTAND to apply"
fi
