# Staging Validation Checklist

---

## Endpoints

| Service | URL |
|---------|-----|
| UI | https://staging.sagarwaidande.org |
| API | https://api-staging.sagarwaidande.org |
| Lago | https://lago-api-staging.sagarwaidande.org |

> Stripe account is India-based — billing address on success-path customers must match India or a Stripe-accepted region.

---

## Part 1 — Operator E2E Walkthrough (Manual)

Walk these in order. Each has a clear pass condition. Do this before every demo and before promoting to prod.

### Step 1 — Platform login

1. Go to `staging.sagarwaidande.org/login`
2. Sign in with a **platform account**
3. **Pass:** lands on billing connections or control plane overview, nav shows Platform section

---

### Step 2 — Billing connection

1. Go to **Billing Connections → New**
2. Create a connection with a Stripe test key, set environment to `test`
3. **Pass:** connection appears in list with status `connected`; detail screen shows provider + environment

---

### Step 3 — Workspace setup

1. Go to **Workspaces → New**
2. Create a workspace, assign the billing connection from Step 2
3. **Pass:** workspace appears in list; detail screen shows billing connection assigned

---

### Step 4 — Pricing catalog (switch to workspace account)

1. Sign in with a **workspace writer account** for the workspace created in Step 3
2. Go to **Pricing → Metrics → New** — create a usage metric (e.g. `api_calls`, `sum` aggregation)
3. Go to **Pricing → Plans → New** — create a plan using that metric with a per-unit price
4. **Pass:** metric and plan appear in their list screens with correct detail

---

### Step 5 — Customer onboarding

1. Go to **Customers → New** — create a customer
2. Open the customer detail, fill the billing profile: legal name, email, billing address, currency, billing connection code
3. **Pass:** readiness panel shows `billing_profile_status = ready`; no missing steps for billing profile

---

### Step 6 — Subscription

1. Go to **Subscriptions → New** — create a subscription for the customer on the plan from Step 4
2. **Pass:** subscription appears with `active` status; customer readiness reflects the subscription

---

### Step 7 — Payment setup

1. From the customer detail, click **Send payment setup request**
2. Open the checkout link, complete with Stripe test card `4242 4242 4242 4242`
3. Return to the customer detail, click **Refresh payment setup**
4. **Pass:** `payment_setup_status = ready`, `default_payment_method_verified = true`

---

### Step 8 — Invoice + payment collection

1. Go to **Invoices** — verify an invoice exists for the customer
2. From the invoice detail, trigger **collect payment**
3. **Pass:** invoice `payment_status` transitions to `succeeded`

---

### Step 9 — Invoice explainability

1. Open the invoice from Step 8
2. Go to **Explainability** tab — verify the lifetime event breakdown loads
3. **Pass:** shows usage events, charges, and line items that explain the invoice total

---

### Step 10 — Quick sanity checks

| Check | Pass condition |
|-------|---------------|
| Hard refresh on customer detail | Page restores correctly, no blank screen |
| Hard refresh on invoice detail | Page restores correctly |
| Breadcrumbs | All links navigate correctly |
| No "Lago" leaked in UI | No raw provider names, sync states, or internal IDs visible to operators |
| Reader account | Can view but save/action buttons are disabled |

---

## Part 2 — Automated Gates

Run these after deploy. All must pass before promoting to prod.

```bash
# 1. Health + rate limiting
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_READER_API_KEY='...' \
make verify-staging-runtime

# 2. Pricing journey (no external deps)
make test-staging-pricing-journey

# 3. Subscription journey
LAGO_API_KEY='...' make test-staging-subscription-journey

# 4. Payment smoke (real Stripe, success + failure)
LAGO_API_KEY='...' make test-staging-payment-smoke

# 5. Replay smoke
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' \
ALPHA_READER_API_KEY='...' \
make test-staging-replay-smoke

# 6. Browser smoke (Playwright)
make test-browser-staging-smoke
```

**All 6 must exit 0.**

---

## Part 3 — Release Gate Decision

Promote to prod only when:

- [ ] CI green (`integration-smoke`, `integration-full`, `web-ui-e2e`)
- [ ] All 10 manual walkthrough steps pass
- [ ] All 6 automated gates exit 0
- [ ] `make preflight-staging` zero failures

---

## Deploy Reference

```bash
# Build + push
IMAGE_TAG=$(git rev-parse --short HEAD)
API_IMAGE_REPOSITORY=139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/api
WEB_IMAGE_REPOSITORY=139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/web
make build-staging-images IMAGE_TAG=$IMAGE_TAG \
  API_IMAGE_REPOSITORY=$API_IMAGE_REPOSITORY \
  WEB_IMAGE_REPOSITORY=$WEB_IMAGE_REPOSITORY

# Deploy
make deploy-staging IMAGE_TAG=$IMAGE_TAG \
  API_IMAGE_REPOSITORY=$API_IMAGE_REPOSITORY \
  WEB_IMAGE_REPOSITORY=$WEB_IMAGE_REPOSITORY

# Rollback if needed
make rollback-staging REVISION=<previous_revision>
```
