# Assisted Tenant Onboarding

White-glove onboarding model for the current product state. Platform operator assists with tenant setup, billing mapping, and first-run verification.

---

## What Tenants Can Do Today

- Payment operations: inspect status, open timeline, retry failed payments
- Invoice explainability: inspect digest, metadata, line items
- Replay/recovery: queue jobs, inspect diagnostics, retry failures
- Reconciliation: compare usage-derived billing with recorded billed entries
- API key governance: create, revoke, rotate, audit

Roles: `reader` (read-only), `writer` (queue/retry operations), `admin` (key management + writer).

---

## Step 1 — Bootstrap Platform Admin Key

```bash
# Direct DB access
DATABASE_URL='postgres://...' \
PLATFORM_KEY_NAME='alpha-platform-root' \
make bootstrap-platform-admin-key

# In-cluster (no direct DB access)
PLATFORM_KEY_NAME='alpha-platform-root' \
make bootstrap-platform-admin-key-cluster
```

Creates one `platform_admin` key. Prints the one-time secret. Refuses if active platform keys already exist unless `ALLOW_EXISTING_ACTIVE_KEYS=1`.

### Mint Fresh Live E2E Keys (Optional)

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
TARGET_TENANT_ID='default' \
OUTPUT='shell' \
make mint-live-e2e-keys-cluster
```

Prints export-ready values for `PLAYWRIGHT_LIVE_PLATFORM_API_KEY`, `PLAYWRIGHT_LIVE_WRITER_API_KEY`, `PLAYWRIGHT_LIVE_READER_API_KEY`, `PLAYWRIGHT_LIVE_TENANT_ID`. Keys default to 24h expiry.

---

## Step 2 — Create Tenant + Bootstrap Admin Key

```bash
# Create tenant
curl -sS -X POST "$ALPHA_API_BASE_URL/internal/tenants" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"id": "tenant_acme", "name": "Acme"}'

# Bootstrap first tenant admin
curl -sS -X POST "$ALPHA_API_BASE_URL/internal/tenants/tenant_acme/bootstrap-admin-key" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "acme-platform-admin"}'
```

Then the tenant admin can create additional keys:

```bash
# Writer key
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/api-keys" \
  -H "X-API-Key: $TENANT_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "tenant-runtime-writer", "role": "writer"}'

# Reader key
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/api-keys" \
  -H "X-API-Key: $TENANT_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "tenant-finance-reader", "role": "reader"}'
```

---

## Step 3 — Browser Sign-In

1. Open the Alpha UI root (`https://staging.sagarwaidande.org`)
2. Session login card → paste the tenant API key → click Sign In

Internally: `POST /v1/ui/sessions/login` with `{"api_key": "..."}`.

---

## Step 4 — Map Billing

Write Lago org and billing provider onto the tenant record:

```bash
curl -sS -X PATCH "$ALPHA_API_BASE_URL/internal/tenants/$TENANT_ID" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"lago_organization_id": "org_acme", "lago_billing_provider_code": "stripe_test"}'
```

Bootstrap staging Stripe/Lago fixtures (if not already done):

```bash
STRIPE_SECRET_KEY='sk_test_...' \
LAGO_WEBHOOK_URL='https://api-staging.sagarwaidande.org/internal/lago/webhooks' \
make lago-staging-bootstrap-payments
```

---

## Step 5 — Seed Pricing

```bash
# Rating rule
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/rating-rules" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "rule_key": "api_calls_v1", "name": "API Calls v1", "version": 1,
    "lifecycle_state": "active", "mode": "graduated", "currency": "USD",
    "graduated_tiers": [
      {"up_to": 1000, "unit_amount_cents": 2},
      {"up_to": 0, "unit_amount_cents": 1}
    ]
  }'

# Meter
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/meters" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "api_calls", "name": "API Calls", "unit": "call",
    "aggregation": "sum", "rating_rule_version_id": "<rule_version_id>"
  }'
```

---

## Step 6 — Create First Customer

Preferred: use the guided onboarding endpoint (creates customer, syncs billing, optionally starts payment setup in one call):

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customer-onboarding" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "cust_acme_primary",
    "display_name": "Acme Primary Customer",
    "email": "billing@acme.test",
    "start_payment_setup": true,
    "payment_method_type": "card",
    "billing_profile": {
      "legal_name": "Acme Primary Customer LLC",
      "email": "billing@acme.test",
      "billing_address_line1": "1 Billing Street",
      "billing_city": "Bengaluru",
      "billing_postal_code": "560001",
      "billing_country": "IN",
      "currency": "USD",
      "provider_code": "stripe_default"
    }
  }'
```

Low-level primitives (use for debug/resume):

```bash
# Create customer
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"external_id": "cust_acme_primary", "display_name": "Acme Primary Customer", "email": "billing@acme.test"}'

# Set billing profile
curl -sS -X PUT "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/billing-profile" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "legal_name": "Acme Primary Customer LLC", "email": "billing@acme.test",
    "billing_address_line1": "1 Billing Street", "billing_city": "Bengaluru",
    "billing_postal_code": "560001", "billing_country": "IN",
    "currency": "USD", "provider_code": "stripe_default"
  }'

# Retry sync if transient error
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/billing-profile/retry-sync" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" -H "Content-Type: application/json" -d '{}'

# Start payment setup
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/payment-setup/checkout-url" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"payment_method_type": "card"}'

# Refresh payment setup after customer completes checkout
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/payment-setup/refresh" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" -H "Content-Type: application/json" -d '{}'

# Inspect customer readiness
curl -sS "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/readiness" \
  -H "X-API-Key: $TENANT_READER_API_KEY"

# Inspect tenant onboarding readiness
curl -sS "$ALPHA_API_BASE_URL/internal/onboarding/tenants/tenant_acme" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY"
```

Expected readiness: `billing_integration.status = ready`, `first_customer.status = ready`.

---

## Step 7 — Verify Before Handoff

### Payment path

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' ALPHA_READER_API_KEY='...' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='...' \
SUCCESS_INVOICE_ID='...' FAILURE_INVOICE_ID='...' \
make verify-staging-acceptance
```

### Replay path

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' ALPHA_READER_API_KEY='...' \
OUTPUT_FILE='/tmp/replay-smoke.json' \
make verify-replay-smoke-staging
```

### Browser + RBAC path

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_PLATFORM_API_KEY='...' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='...' \
PLAYWRIGHT_LIVE_READER_API_KEY='...' \
PLAYWRIGHT_LIVE_REPLAY_JOB_ID='...' \
PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID='...' \
PLAYWRIGHT_LIVE_REPLAY_METER_ID='...' \
make web-e2e-live
```

---

## What Is Still Manual Today

- First platform admin key bootstrap
- Tenant billing mapping (`lago_organization_id`, `lago_billing_provider_code`)
- Stripe provider configuration in Lago
- Pricing/rating bootstrap for new tenants without a prebuilt template

---

## Per-Tenant Handoff Record

Document for each onboarded tenant:
- tenant id, display name
- platform admin key owner, tenant admin key owner
- Lago organization id, billing provider code
- first customer external ids
- first meter ids and rating rule ids
- verified success invoice id, verified failure invoice id
- verified replay job id
- date of last acceptance run
