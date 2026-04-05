# Staging Validation Checklist

## Endpoints

| Service | URL |
|---------|-----|
| UI | https://staging.sagarwaidande.org |
| API | https://api-staging.sagarwaidande.org |

---

## Part 1 — Manual Walkthrough

Walk in order. Each step has a pass condition.

### Step 1 — Register + workspace

1. Go to `/register`, create an account
2. **Pass:** auto-creates workspace, lands on `/control-plane`

### Step 2 — Pricing catalog

1. **Pricing > Metrics > New** — create a usage metric (e.g. `api_calls`, `sum`)
2. **Pricing > Plans > New** — create a plan with that metric + per-unit price
3. **Pass:** metric and plan appear in list screens

### Step 3 — Customer

1. **Customers > New** — create a customer with billing profile (legal name, email, address, currency)
2. **Pass:** readiness panel shows `billing_profile_status = ready`

### Step 4 — Subscription

1. **Subscriptions > New** — subscribe customer to the plan
2. **Pass:** subscription shows `active` status

### Step 5 — Payment setup

1. From customer detail, click **Send payment setup request**
2. Complete checkout with Stripe test card `4242 4242 4242 4242`
3. Refresh payment setup
4. **Pass:** `payment_setup_status = ready`

### Step 6 — Invoice + payment

1. **Invoices** — verify invoice exists
2. Trigger **collect payment**
3. **Pass:** `payment_status` = `succeeded`

### Step 7 — Sanity

| Check | Pass |
|-------|------|
| Hard refresh on any detail page | Restores correctly |
| Breadcrumbs | All links work |
| Reader account | View only, actions disabled |
| Workspace switcher | Can switch between workspaces |

---

## Part 2 — Automated Gates

```bash
# Health + rate limiting
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_READER_API_KEY='...' \
make verify-staging-runtime

# Browser E2E
make test-browser-staging-smoke
```

---

## Part 3 — Release Gate

Promote to prod when:

- [ ] CI green (all pipeline stages pass)
- [ ] Manual walkthrough passes
- [ ] Automated gates exit 0

---

## Deploy

```bash
IMAGE_TAG=$(git rev-parse --short HEAD)
API_IMAGE_REPOSITORY=139831607173.dkr.ecr.us-east-1.amazonaws.com/alpha-staging/api
WEB_IMAGE_REPOSITORY=139831607173.dkr.ecr.us-east-1.amazonaws.com/alpha-staging/web

make build-staging-images IMAGE_TAG=$IMAGE_TAG \
  API_IMAGE_REPOSITORY=$API_IMAGE_REPOSITORY \
  WEB_IMAGE_REPOSITORY=$WEB_IMAGE_REPOSITORY

make deploy-staging IMAGE_TAG=$IMAGE_TAG \
  API_IMAGE_REPOSITORY=$API_IMAGE_REPOSITORY \
  WEB_IMAGE_REPOSITORY=$WEB_IMAGE_REPOSITORY
```
