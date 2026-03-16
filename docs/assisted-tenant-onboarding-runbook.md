# Assisted Tenant Onboarding Runbook

Use this runbook when you want real users to start using `usage-billing-control-plane` today.

This is the recommended onboarding model for the current product state:
- assisted tenant setup by the platform operator
- tenant-scoped API keys for access control
- browser sessions created from those API keys
- operator-guided billing setup in Lago/Stripe

This is the right model now because the core operator features are real, but tenant bootstrap is not yet a full self-serve product flow.

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
- the first tenant admin and Stripe/Lago billing bootstrap are still operator-assisted

## 2. Recommended Onboarding Model

Use a white-glove assisted flow for each tenant:
1. collect tenant inputs
2. bootstrap the tenant's first admin key
3. let the tenant admin create reader and writer keys
4. configure the tenant's Lago and Stripe billing path
5. create or confirm the tenant pricing model
6. prove payment, explainability, and replay paths
7. hand off the UI and API usage to the tenant users

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

### Step 1: Bootstrap the Tenant's First Admin Key

Today this first key is an operator step.

Why:
- API key creation is tenant-scoped
- once a tenant has an `admin` key, that tenant can self-manage additional keys
- but the first tenant admin key still needs platform bootstrap

Current supported model:
- use the dedicated operator bootstrap command below to mint the first tenant `admin` key
- after that, keep all routine access management inside the tenant through the API key endpoints

Operator bootstrap command:

```bash
DATABASE_URL='postgres://...' \
TENANT_ID='tenant_acme' \
KEY_NAME='acme-platform-admin' \
make bootstrap-tenant-admin-key
```

Behavior:
- creates one `admin` API key for the requested tenant
- prints the one-time `secret` so it can be handed to the tenant admin securely
- refuses to run if the tenant already has active keys unless `ALLOW_EXISTING_ACTIVE_KEYS=1` is set
- supports optional expiry with `EXPIRES_AT=<RFC3339 timestamp>`

Recommended operator handling:
- capture the JSON output once
- hand the `secret` to the tenant's technical owner through a secure channel
- do not store the cleartext secret in docs or chat logs after handoff

After the first admin key exists, that tenant admin can create more keys with:

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

### Step 2: First Browser Sign-In

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

### Step 3: Create or Map the Lago Billing Side

This is still operator-assisted.

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
  -H "X-API-Key: $DEFAULT_OPERATOR_ADMIN_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "lago_organization_id": "org_acme",
    "lago_billing_provider_code": "stripe_test"
  }'
```

Why this matters:
- Lago webhook routing now resolves `organization_id` through the canonical tenant record
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

### Step 4: Seed Tenant Pricing Metadata

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

### Step 5: Seed the First Real Billing Objects

Create or confirm:
- one or more customer external ids
- one payment-capable customer if payment flows are in scope
- one failure-path customer if you want to prove retry/recovery behavior

For staging payment setup and fixture creation, use the existing runbook and scripts:
- [real-payment-e2e-runbook.md](./real-payment-e2e-runbook.md)
- `make lago-staging-bootstrap-payments`
- `make verify-staging-acceptance`

### Step 6: Prove the Tenant Can Actually Use the System

Do not hand off the tenant until these checks pass.

#### 6a. Payment path

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

#### 6b. Replay / recovery path

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
PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_JOB_ID='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID='replace_me' \
PLAYWRIGHT_LIVE_REPLAY_METER_ID='replace_me' \
make web-e2e-live
```

This proves:
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

Keep these expectations explicit.

Still operator-assisted:
- first tenant admin key bootstrap
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

Use this exact sequence for each new tenant:
1. create or map the tenant billing side
2. write `lago_organization_id` and `lago_billing_provider_code` onto the tenant record
3. bootstrap the tenant admin key
4. let the tenant admin mint reader and writer keys
5. seed one meter and one rating rule
6. verify payment flows
7. verify replay flows
8. verify browser RBAC flows
9. hand off the tenant with recorded evidence

This keeps onboarding reproducible and auditable.

## 8. Related Runbooks

Use these together with this onboarding doc:
- [staging-go-live-checklist.md](./staging-go-live-checklist.md)
- [real-payment-e2e-runbook.md](./real-payment-e2e-runbook.md)
- [replay-recovery-live-runbook.md](./replay-recovery-live-runbook.md)
- [lago-staging-bootstrap.md](./lago-staging-bootstrap.md)
- [infra-rollout-runbook.md](./infra-rollout-runbook.md)
