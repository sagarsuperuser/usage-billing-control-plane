# Assisted Tenant Onboarding Runbook

Use this runbook when you want real users to start using `usage-billing-control-plane` today.

This is the recommended onboarding model for the current product state:
- assisted tenant setup by the platform operator
- tenant-scoped API keys for access control
- browser sessions created from those API keys
- operator-guided billing setup through Alpha, with some Lago/Stripe steps still remaining behind the scenes today

Current-state note:
- this runbook describes the working operator-assisted path today
- the long-term target is that Alpha remains the only normal entrypoint and Lago becomes an implementation detail behind Alpha
- use [alpha-lago-boundary.md](./alpha-lago-boundary.md) for the target system boundary
- use [alpha-implementation-roadmap.md](./alpha-implementation-roadmap.md) for the phased delivery plan

This is the right model now because the core operator features are real, but tenant bootstrap and billing setup are not yet a full self-serve product flow.

## 1. What Users Can Meaningfully Do Today

The product is already useful for these user groups:
- billing operators
- finance or support readers
- tenant technical admins

Meaningful features available now:
- `Payment Operations`
  - inspect invoice payment status
  - open invoice webhook timeline
  - retry failed payments when the session role allows it
- `Invoice Explainability`
  - inspect invoice digest, metadata, and line items
  - answer why an invoice looks the way it does
- `Replay / Recovery Operations`
  - queue replay jobs
  - inspect replay diagnostics
  - retry failed replay jobs when the session role allows it
- `Reconciliation`
  - compare usage-derived expected billing with recorded billed entries
  - verify replay closed a mismatch
- `API Key Governance`
  - create tenant-scoped reader/writer keys
  - revoke and rotate keys
  - inspect audit history

Current role model:
- `reader`
  - read-only access to payment visibility, replay diagnostics, explainability, reconciliation
- `writer`
  - can queue replay jobs and retry payment operations
- `admin`
  - can do writer actions and manage API keys for the same tenant

Important current limitation:
- this is not yet a self-serve signup and billing-configuration product
- platform-admin onboarding and Stripe/Lago billing bootstrap are still operator-assisted

## 2. Recommended Onboarding Model

Use a white-glove assisted flow for each tenant:
1. collect tenant inputs
2. create or confirm the tenant record
3. bootstrap the tenant's first admin key through the internal operator API
4. let the tenant admin create reader and writer keys
5. configure the tenant's Lago and Stripe billing path
6. create or confirm the tenant pricing model
7. create the first billing-ready customer in Alpha
8. prove payment, explainability, and replay paths
9. hand off the UI and API usage to the tenant users

Do not start with open self-serve signup.

## 3. Inputs to Collect Before Onboarding

Collect these before touching the system:
- tenant name
- tenant identifier you want to use in alpha
- technical owner name and email
- finance or support reader emails
- operator/writer emails
- Lago organization decision:
  - existing Lago org to map
  - or new Lago org to create
- billing provider decision:
  - Stripe test or live
  - billing currency
- first customer external ids to seed
- first meter and rating rule requirements
- one sample invoice or replay use case you can verify after setup

## 4. End-to-End Onboarding Flow

### Step 1: Bootstrap the Platform Admin Key

This is the root bootstrap step.

Why:
- `/internal/*` operator routes now require `platform_admin`
- tenant creation and tenant-admin bootstrap are now internal API flows
- the system still needs one explicit root-of-trust credential to start from

Current supported model:
- use the dedicated operator bootstrap command below once to mint a `platform_admin` key
- after that, use the internal operator APIs for tenant creation and tenant-admin bootstrap

Operator bootstrap command:

```bash
DATABASE_URL='postgres://...' \
PLATFORM_KEY_NAME='alpha-platform-root' \
make bootstrap-platform-admin-key
```

If the environment uses a private database and your laptop cannot reach it directly, use the in-cluster bootstrap path instead:

```bash
PLATFORM_KEY_NAME='alpha-platform-root' \
make bootstrap-platform-admin-key-cluster
```

Behavior:
- creates one `platform_admin` API key
- prints the one-time `secret`
- refuses to run if active platform keys already exist unless `ALLOW_EXISTING_ACTIVE_KEYS=1` is set
- supports optional expiry with `EXPIRES_AT=<RFC3339 timestamp>`
- the cluster path reuses the deployed API workload image, service account, config map, and runtime secret so it runs inside cluster network boundaries

### Optional: Mint Fresh Live UI E2E Keys In Cluster

If you need fresh staging browser-smoke credentials without depending on stale shared secrets, use:

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
TARGET_TENANT_ID='default' \
OUTPUT='shell' \
make mint-live-e2e-keys-cluster
```

This prints export-ready values for:
- `PLAYWRIGHT_LIVE_PLATFORM_API_KEY`
- `PLAYWRIGHT_LIVE_WRITER_API_KEY`
- `PLAYWRIGHT_LIVE_READER_API_KEY`
- `PLAYWRIGHT_LIVE_TENANT_ID`

Behavior notes:
- existing active keys with the same minted names are revoked before new keys are created
- minted live E2E keys default to a 24h expiry unless `EXPIRES_AT` is set explicitly

Recommended operator handling:
- capture the JSON output once
- store the `secret` in the operator secret manager
- do not store the cleartext secret in docs or chat logs after handoff

### Step 2: Create the Tenant and Bootstrap Its First Admin Key

Use the platform key for both operations.

Create or ensure the tenant:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/internal/tenants" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "tenant_acme",
    "name": "Acme"
  }'
```

Bootstrap the first tenant admin:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/internal/tenants/tenant_acme/bootstrap-admin-key" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-platform-admin"
  }'
```

Behavior:
- returns a tenant-scoped `admin` key plus the one-time `secret`
- refuses by default if the tenant already has active keys
- supports:
  - `expires_at`
  - `allow_existing_active_keys`

After the first tenant admin key exists, that tenant admin can create more keys with:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/api-keys" \
  -H "X-API-Key: $TENANT_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tenant-runtime-writer",
    "role": "writer"
  }'
```

And a read-only key:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/api-keys" \
  -H "X-API-Key: $TENANT_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tenant-finance-reader",
    "role": "reader"
  }'
```

Notes:
- the `secret` is returned only once on create or rotate
- `writer` cannot create new keys
- `admin` is the tenant role that manages tenant API keys

### Step 3: First Browser Sign-In

The browser session model is API-key backed.

User flow:
1. browse to the alpha UI root, for example `https://staging.sagarwaidande.org`
2. the app redirects to `/control-plane`
3. use the session login card
4. paste the tenant API key into the `API Key` field
5. optionally set `API Base URL` when UI and API are split across hosts
6. click `Sign In`

Under the hood the UI creates a session by calling:
- `POST /v1/ui/sessions/login`
- payload:

```json
{
  "api_key": "tenant-reader-or-writer-or-admin-key"
}
```

This is the practical onboarding experience today.

### Step 4: Create or Map the Billing Side

This is still operator-assisted today.
In the target architecture, Alpha owns the operator workflow and Lago stays behind Alpha as the billing execution engine.
Right now some Lago and Stripe setup still remains operationally visible during environment bring-up and tenant activation.

You need:
- a Lago organization for the tenant
- a working Lago API key for that organization
- the tenant record in alpha updated with:
  - `lago_organization_id`
  - `lago_billing_provider_code`
- the alpha Lago webhook endpoint configured in Lago
- if payments are enabled, a Stripe provider in Lago

Recommended operator path:
1. create or confirm the Lago organization
2. create or confirm the tenant in alpha
3. write the billing mapping fields onto the tenant record

Example:

```bash
curl -sS -X PATCH "$ALPHA_API_BASE_URL/internal/tenants/$TENANT_ID" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "lago_organization_id": "org_acme",
    "lago_billing_provider_code": "stripe_test"
  }'
```

Why this matters:
- Alpha owns the canonical tenant record and billing mapping
- Lago webhook routing now resolves `organization_id` through the tenant record in Alpha
- `LAGO_ORG_TENANT_MAP` is no longer the production mapping mechanism
- unmapped Lago organizations fail closed instead of silently routing into `default`

For staging, the proven bootstrap path is:

```bash
STRIPE_SECRET_KEY='sk_test_replace_me_if_provider_missing' \
LAGO_WEBHOOK_URL='https://api-staging.sagarwaidande.org/internal/lago/webhooks' \
make lago-staging-bootstrap-payments
```

What that already automates in staging:
- ensures the Lago Stripe provider exists
- ensures the Lago webhook endpoint exists with `jwt` signing
- ensures known payment test customers exist
- ensures the success customer has a usable Stripe payment method and billing address

For environment-specific billing details, follow:
- [real-payment-e2e-runbook.md](./real-payment-e2e-runbook.md)
- [lago-staging-bootstrap.md](./lago-staging-bootstrap.md)

### Step 5: Seed Tenant Pricing Metadata

At minimum, a tenant needs:
- one rating rule
- one meter linked to that rule

Example rating rule:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/rating-rules" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "rule_key": "api_calls_v1",
    "name": "API Calls v1",
    "version": 1,
    "lifecycle_state": "active",
    "mode": "graduated",
    "currency": "USD",
    "graduated_tiers": [
      {"up_to": 1000, "unit_amount_cents": 2},
      {"up_to": 0, "unit_amount_cents": 1}
    ]
  }'
```

Example meter linked to that rule version:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/meters" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "api_calls",
    "name": "API Calls",
    "unit": "call",
    "aggregation": "sum",
    "rating_rule_version_id": "<rule_version_id>"
  }'
```

Important current product truth:
- pricing bootstrap is API/operator-driven
- this is not yet exposed as a polished self-serve billing-config UI

### Step 6: Seed the First Real Billing Objects

Create or confirm:
- one or more customer external ids
- one payment-capable customer if payment flows are in scope
- one failure-path customer if you want to prove retry/recovery behavior

Preferred happy-path flow: use the customer onboarding workflow endpoint.

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

What Alpha does now:
- creates or reconciles the customer in Alpha
- applies the billing profile and syncs it to Lago behind the scenes when it is complete
- optionally starts payment-method setup and returns the checkout URL in the same response
- records sync and verification failures back onto Alpha readiness state instead of requiring operators to work in Lago directly

Low-level customer primitives remain available when you need to resume or debug a partially completed flow.

Create the first customer in Alpha directly:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "cust_acme_primary",
    "display_name": "Acme Primary Customer",
    "email": "billing@acme.test"
  }'
```

Set the billing profile in Alpha directly:

```bash
curl -sS -X PUT "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/billing-profile" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "legal_name": "Acme Primary Customer LLC",
    "email": "billing@acme.test",
    "billing_address_line1": "1 Billing Street",
    "billing_city": "Bengaluru",
    "billing_postal_code": "560001",
    "billing_country": "IN",
    "currency": "USD",
    "provider_code": "stripe_default"
  }'
```

Retry billing-profile sync if Alpha recorded a transient `sync_error`:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/billing-profile/retry-sync" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Start payment-method setup in Alpha:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/payment-setup/checkout-url" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "payment_method_type": "card"
  }'
```

What Alpha does now:
- requests a provider checkout URL from Lago behind the scenes
- marks local payment setup as pending in Alpha
- does not mutate readiness state through `GET` calls
- automatically refreshes payment setup when Lago later emits `customer.payment_provider_created`

Refresh payment setup after the customer completes checkout:

```bash
curl -sS -X POST "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/payment-setup/refresh" \
  -H "X-API-Key: $TENANT_WRITER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'
```

What Alpha does now:
- syncs and verifies payment-method state against Lago explicitly
- updates the local readiness record with the verified default payment method state
- this remains the manual fallback if webhook-driven refresh has not completed yet

Inspect customer readiness:

```bash
curl -sS "$ALPHA_API_BASE_URL/v1/customers/cust_acme_primary/readiness" \
  -H "X-API-Key: $TENANT_READER_API_KEY"
```

Inspect overall tenant onboarding readiness:

```bash
curl -sS "$ALPHA_API_BASE_URL/internal/onboarding/tenants/tenant_acme" \
  -H "X-API-Key: $PLATFORM_ADMIN_API_KEY"
```

Expected outcome:
- `billing_integration.status = ready`
- `first_customer.status = ready`
- top-level onboarding `status = ready`

If customer readiness stays pending, inspect:
- `billing_provider_configured`
- `lago_customer_synced`
- `default_payment_method_verified`
- `billing_profile.profile_status`
- `payment_setup.setup_status`

For staging payment fixture creation and verification, use the existing runbook and scripts:
- [real-payment-e2e-runbook.md](./real-payment-e2e-runbook.md)
- `make lago-staging-bootstrap-payments`
- `make verify-staging-acceptance`

### Step 7: Prove the Tenant Can Actually Use the System

Do not hand off the tenant until these checks pass.

#### 7a. Payment path

Run the acceptance gate:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='replace_me' \
ALPHA_READER_API_KEY='replace_me' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='replace_me' \
SUCCESS_INVOICE_ID='56251c97-597a-4cec-9a22-8106d746def8' \
FAILURE_INVOICE_ID='baa27549-32d4-47cd-9f14-d98b61c8b0fa' \
make verify-staging-acceptance
```

This proves:
- runtime health
- invoice payment visibility
- success payment E2E
- failure payment E2E

#### 7b. Replay / recovery path

Run the replay smoke:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='replace_me' \
ALPHA_READER_API_KEY='replace_me' \
OUTPUT_FILE='/tmp/replay-smoke.json' \
make verify-replay-smoke-staging
```

This proves:
- replay mismatch detection
- replay job processing
- replay adjustment creation
- reconciliation closure

#### 6c. Browser UX and RBAC path

Run the live browser smoke:

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_PLATFORM_API_KEY='replace_me' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_JOB_ID='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_METER_ID='replace_me' \
make web-e2e-live
```

This proves:
- platform overview loads with live workspace attention data
- writer session can use payment and replay operations
- reader session is correctly read-only
- explainability and replay diagnostics open in the real UI

### Step 7: Handoff to Tenant Users

After proof is complete, hand off the tenant with these instructions:
- `reader` users should use:
  - payment status inspection
  - event timeline inspection
  - invoice explainability
  - replay diagnostics and reconciliation views
- `writer` users should use:
  - payment retry
  - replay queue and replay retry
- `admin` users should use:
  - API key creation, revoke, rotate, and audit
  - then delegate routine operations to reader/writer users

Recommended first-session walkthrough:
1. sign into `/control-plane`
2. open `Payment Operations`
3. inspect one failed or succeeded invoice
4. open `Invoice Explainability`
5. inspect one invoice digest and line items
6. open `Replay Operations`
7. inspect one known replay job
8. if the user is a writer, queue one replay job or retry one safe staging operation

## 5. What Is Still Manual Today

Target-state note:
- the items below are current operational realities, not the desired permanent boundary
- the long-term goal is to absorb normal onboarding and billing setup flows into Alpha and keep Lago behind adapter and webhook boundaries

Keep these expectations explicit.

Still operator-assisted:
- first platform admin key bootstrap
- tenant billing mapping setup (`lago_organization_id`, `lago_billing_provider_code`)
- Stripe provider configuration in Lago
- first customer and payment bootstrap when bringing up a new environment
- pricing/rating bootstrap if the tenant is not using a prebuilt template

Already reproducible or automated:
- staging Stripe/Lago payment bootstrap
- payment success/failure verification
- replay live smoke
- browser live smoke for payment ops, explainability, and replay ops
- backup/restore drill
- rollback rehearsal

## 6. What to Document Per Tenant

When onboarding a real tenant, record these in the tenant handoff doc:
- tenant id
- tenant display name
- platform admin key owner
- tenant admin key owner
- Lago organization id
- Lago billing provider code
- first customer external ids
- first meter ids and rating rule ids
- verified success invoice id
- verified failure invoice id
- verified replay job id
- date of last acceptance run

Do not leave this as tribal knowledge.

## 7. Recommended Operator Workflow

Use this exact sequence for each new tenant today:
1. bootstrap or retrieve the platform admin key
2. create the tenant through `/internal/tenants`
3. write `lago_organization_id` and `lago_billing_provider_code` onto the tenant record
4. bootstrap the tenant admin key through `/internal/tenants/{id}/bootstrap-admin-key`
5. let the tenant admin mint reader and writer keys
6. seed one meter and one rating rule
7. verify payment flows
8. verify replay flows
9. verify browser RBAC flows
10. hand off the tenant with recorded evidence

This keeps onboarding reproducible and auditable.

As Alpha absorbs more billing setup behind its own APIs, this workflow should get shorter rather than more complex.

## 8. Related Runbooks

Use these together with this onboarding doc:
- [alpha-lago-boundary.md](./alpha-lago-boundary.md)
- [alpha-implementation-roadmap.md](./alpha-implementation-roadmap.md)
- [staging-go-live-checklist.md](./staging-go-live-checklist.md)
- [real-payment-e2e-runbook.md](./real-payment-e2e-runbook.md)
- [replay-recovery-live-runbook.md](./replay-recovery-live-runbook.md)
- [lago-staging-bootstrap.md](./lago-staging-bootstrap.md)
- [infra-rollout-runbook.md](./infra-rollout-runbook.md)
