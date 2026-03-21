# End-to-End Product Journeys

This document defines Alpha's canonical product journeys for end-to-end validation.

The purpose is not to turn every test into a giant staging drill. The purpose is to make the product journeys explicit so we know:

- what the real operator and customer flows are
- which flows are already automated
- which flows are only partially covered today
- what a true release-confidence check should prove

If a product area does not map cleanly to one of these journeys, the current testing model is incomplete.

---

## Core Rule

Alpha should validate product behavior through a small number of canonical journeys.

Each journey should answer three questions:

1. what setup is required
2. what real user or operator steps happen
3. what state transitions Alpha must prove at the end

Do not confuse infrastructure smoke with product journey validation.

---

## Coverage Terms

Use these terms consistently:

- `implemented`: automated and already usable as a staging journey
- `partial`: some critical parts are automated, but the full user journey is not
- `planned`: product journey is defined here, but automation is not complete yet

---

## Journey Set

| Journey | Purpose | Current State |
| --- | --- | --- |
| Pricing configuration journey | prove metrics, generated rating rules, and plans are commercially usable | implemented |
| Subscription billing journey | prove subscriptions become billable from configured pricing and usage | implemented |
| Payment setup and collect-payment journey | prove customer payment setup can move a blocked customer into a payable state | planned |
| Payment retry and failure journey | prove Alpha payment recovery against real Lago and Stripe wiring | implemented |
| Replay and recovery journey | prove recovery tooling works against fresh replay fixtures | implemented |
| Browser operator journey | prove core operator surfaces load and route correctly in staging | partial |

---

## 1. Pricing Configuration Journey

### Purpose

Prove that Alpha pricing setup is not just CRUD. It must be commercially executable.

### Product surfaces involved

- rating rules
- meters
- plans
- plan pricing associations

### Real journey

1. create a pricing metric through Alpha's pricing API
2. verify Alpha generated the default draft rating rule version behind that metric
3. create a plan linked to the metric
4. verify the metric -> generated rule -> plan graph is visible and internally consistent

### End-state assertions

- the metric is linked to the intended rating rule version
- the plan references the intended metric pricing
- no missing pricing boundary remains between metric and plan
- the configuration is ready for a subscription billing journey

### Current automation state

- `implemented`
- staging journey entrypoint:

```bash
make test-staging-pricing-journey
```

- the implemented journey creates a per-run pricing metric, verifies the generated default rating rule, creates a plan linked to that metric, and verifies the resulting graph through Alpha APIs

### Implemented entrypoint

The journey is intentionally per-run and does not rely on legacy fixed fixture ids:

```bash
make test-staging-pricing-journey
```

---

## 2. Subscription Billing Journey

### Purpose

Prove that configured pricing actually produces a billable subscription flow.

### Product surfaces involved

- customers
- plans
- subscriptions
- invoices
- payment visibility

### Real journey

1. start from a valid pricing configuration
2. create a customer
3. create a subscription on the target plan
4. emit or prepare usage that exercises the configured metric
5. emit subscription-targeted usage through Alpha
6. verify Lago received the persisted subscription event
7. request deterministic current usage from Lago for that persisted subscription

### End-state assertions

- the subscription exists in Alpha with the correct customer and plan linkage
- Alpha may keep the subscription in `pending_payment_setup` until the customer has a verified default payment method
- the persisted Lago subscription matches the Alpha subscription
- the usage event is present in both Alpha and Lago
- a real Lago current-usage response returns a positive billed amount for the configured subscription
- downstream payment and invoice-visibility journeys can start from the same synced pricing and subscription state

### Current automation state

- `implemented`
- staging journey entrypoint:

```bash
make test-staging-subscription-journey LAGO_API_KEY='...'
```

- the implemented journey proves:
  - Alpha creates real pricing, customer, and subscription state
  - customer billing profile sync reaches Lago
  - subscription-targeted usage reaches Lago
  - Lago can compute real current usage from the persisted subscription and usage
  - Alpha keeps payment readiness explicit instead of falsely reporting an active ready-to-collect subscription

### Required future automation

Current boundary:

- this journey intentionally proves deterministic billable state through real Lago current usage
- it does not wait for scheduled recurring billing or require a persisted issued invoice
- persisted invoice collection and payment convergence remain covered by the payment journeys

---

## 3. Payment Setup and Collect-Payment Journey

### Purpose

Prove Alpha's real collect-payment workflow, not just billing outcomes.

This is the missing product journey behind the `collect_payment` recommendation.

### Product surfaces involved

- customers
- payment setup request and resend
- payment detail
- customer detail payment-setup status
- invoices and payments

### Real journey

1. create or identify a customer without a usable payment method
2. create a collectible invoice for that customer
3. verify Alpha lifecycle recommends `collect_payment`
4. send payment setup request from Alpha
5. confirm the hosted setup link exists
6. complete hosted setup as the customer
7. refresh customer payment setup state in Alpha
8. retry payment from Alpha
9. verify the payment succeeds and the lifecycle changes appropriately

### End-state assertions

- customer payment setup transitions from missing or incomplete to ready
- payment detail no longer recommends `collect_payment`
- retry/payment result converges through Alpha webhook projections
- customer and payment surfaces agree on the new state

### Current automation state

- `planned`
- current payment smoke intentionally covers:
  - success billing outcome with an attached payment method
  - failure billing outcome with no default payment method
- that is useful billing plumbing coverage, but it is not the full collect-payment journey

### Required future automation

Add a dedicated payment setup journey that performs both Alpha and customer-side steps.

Recommended future entrypoint:

```bash
make test-staging-payment-setup-journey
```

This journey should remain separate from the narrower payment smoke.

---

## 4. Payment Retry and Failure Journey

### Purpose

Prove Alpha payment recovery against real staging billing wiring.

### Product surfaces involved

- invoice retry payment
- payment visibility
- invoice payment status projections
- payment lifecycle recommendation
- webhook ingestion

### Real journey

1. bootstrap fresh per-run Lago fixture customers
2. ensure the Alpha tenant is mapped to the Lago organization
3. create a success invoice fixture
4. create a failure invoice fixture
5. verify success path converges through Alpha
6. verify failure path converges through Alpha

### End-state assertions

- Lago reaches the expected terminal payment state
- Alpha payment projection converges for both invoices
- Alpha lifecycle summary matches the expected recommendation
- event timeline exists and is coherent

### Current automation state

- `implemented`

### Current entrypoint

```bash
make test-staging-payment-smoke LAGO_API_KEY='...'
```

### Current shape

- mints fresh platform, writer, and reader keys automatically
- patches tenant billing mapping automatically
- uses per-run fixture customer ids
- verifies both success and failure outcomes

### Important boundary

This journey proves billing execution and Alpha projection correctness.
It does not prove the full customer payment setup flow.

---

## 5. Replay and Recovery Journey

### Purpose

Prove that Alpha recovery tooling works against fresh replay data.

### Product surfaces involved

- replay operations
- replay diagnostics
- replay queue
- recovery visibility

### Real journey

1. create a fresh replay fixture
2. queue replay work
3. inspect replay diagnostics
4. verify replay execution state is visible in Alpha

### End-state assertions

- replay job exists and is queryable
- diagnostics surface points to the correct scope
- recovery operator flow remains usable on fresh fixtures

### Current automation state

- `implemented`

### Current entrypoint

```bash
make test-staging-replay-smoke
```

---

## 6. Browser Operator Journey

### Purpose

Prove that the main operator surfaces are reachable and render against live staging state.

### Product surfaces involved

- control-plane overview
- payments
- replay operations
- invoice explainability
- login and session state

### Real journey

1. sign in through browser session login
2. open core operator surfaces
3. verify page-specific ready states
4. optionally deep-link into a known invoice or replay job when fixture ids are supplied

### End-state assertions

- browser session is authenticated
- target routes load successfully
- live UI contracts remain stable enough for operators

### Current automation state

- `partial`
- browser smoke covers route-level and page-readiness behavior
- it does not yet prove the full pricing, subscription, or payment setup journeys through the UI

### Current entrypoint

```bash
make test-browser-staging-smoke
```

---

## How These Journeys Relate

Use the journeys in dependency order:

1. pricing configuration journey
2. subscription billing journey
3. payment setup and collect-payment journey
4. payment retry and failure journey
5. replay and recovery journey
6. browser operator journey

Not every deploy needs all of them.

### Minimum release-confidence set today

Use:

1. `make verify-staging-runtime`
2. `make test-staging-payment-smoke LAGO_API_KEY='...'`
3. `make test-staging-replay-smoke`
4. `make test-browser-staging-smoke`

### Long-term release-confidence set

Use the minimum set above plus:

1. `make test-staging-pricing-journey`
2. `make test-staging-subscription-journey`
3. `make test-staging-payment-setup-journey`

---

## Data Rules for Journey Tests

All journey automation must follow these rules:

- use per-run fixture ids
- keep cleanup separate from bootstrap
- never rely on fixed customer ids like `cust_e2e_success`
- never rely on stale tenant billing mapping being present
- prefer explicit operator-owned setup steps when the product really depends on them

---

## Current Source of Truth

Use this document together with:

- [Testing Strategy](../standards/testing-strategy.md)
- [Real Payment E2E Runbook](./real-payment-e2e-runbook.md)
- [Replay Recovery Live Runbook](./replay-recovery-live-runbook.md)
