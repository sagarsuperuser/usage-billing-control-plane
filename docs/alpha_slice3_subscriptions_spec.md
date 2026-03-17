# Alpha Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup

This document defines the third Wave 1 implementation slice for Alpha: the first real subscription domain, including payer-completed payment-method setup.

Read together with:

- [Alpha Import Goal](./alpha_import_goal.md)
- [Alpha Import Matrix](./alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)
- [Slice 2 Spec: Pricing Foundation](./alpha_slice2_pricing_spec.md)

---

## Objective

Alpha must own the core subscription lifecycle without forcing users into Lago UI.

For Wave 1, that means Alpha should support:

- subscriptions list and detail
- create subscription
- update subscription
- request payment setup from the subscription flow
- resend payment setup
- track payment-method status after the payer completes setup

This slice intentionally treats payment-method linking as part of subscription activation, not as an operator-only payment task.

---

## Product Scope

### In scope

- `Subscriptions` tenant domain
- subscription list
- subscription detail
- create subscription flow
- update subscription flow
- operator action to request payment setup
- operator action to resend payment setup
- payer-completed payment-method status tracking
- subscription and customer surfaces that clearly show payment setup state

### Out of scope

- advanced entitlements UI
- alerting and alert policies
- deep upgrade/downgrade optimization UX
- every Lago subscription mutation variant
- customer portal as a full product surface
- operator-entered payment method capture

---

## User Stories

1. A tenant operator can create a subscription for a customer in Alpha.
2. A tenant operator can select a plan and understand the subscription's current commercial state in Alpha.
3. A tenant operator can trigger payment setup from Alpha without handling card or bank details directly.
4. A payer can complete payment-method setup through a secure hosted flow.
5. Alpha can show whether payment setup is not requested, pending, ready, or requires attention.
6. A tenant operator can resend payment setup if the payer did not complete it.
7. A tenant operator can inspect subscription detail in Alpha without needing Lago UI.

---

## Product Rules

1. Subscription creation and payment setup are connected, but not the same action.
2. The operator initiates payment setup; the payer completes it.
3. Do not make payment-method linking feel like an internal admin task.
4. Keep subscription setup simple enough for non-billing-expert operators.
5. Hide engine-specific subscription and checkout plumbing from the primary UX.
6. Use clear lifecycle language such as:
   - `draft`
   - `pending payment setup`
   - `active`
   - `action required`

---

## Target Product Surface

### Routes

- `/subscriptions`
- `/subscriptions/new`
- `/subscriptions/[id]`

### Navigation placement

- one top-level tenant nav item: `Subscriptions`

### Primary actions

- create subscription
- request payment setup
- resend payment setup
- inspect subscription detail

### Secondary actions

- edit subscription
- change plan later
- view linked customer and pricing objects

---

## Backend Scope

### Domain boundary

Alpha should own a subscription service boundary that exposes Alpha-native subscription state even if execution and checkout are backed by Lago or provider-hosted flows underneath.

### Required backend work

1. Subscription model and APIs
- create
- list
- get detail
- update basic editable fields

2. Customer-plan linkage
- customer external ID or customer record linkage
- plan linkage using Alpha-owned plan identifiers

3. Payment setup orchestration
- explicit action to request payment setup
- hosted payment-setup link generation or retrieval
- resend or regenerate semantics where appropriate

4. Payment-method status model
- not requested
- pending
- ready
- action required
- optional last verification or last failure summary

5. State reconciliation
- webhook-driven or polling-driven update of payment-method status
- stable projection into Alpha detail APIs

6. Permission model
- tenant-scoped only
- reader can inspect
- writer/admin can create and request payment setup

### Suggested APIs

Use Alpha-owned tenant APIs for:

- subscriptions CRUD within Wave 1 scope
- payment setup request/resend actions
- subscription detail read

The exact route shape can vary, but the product model should remain:

- tenant-scoped
- Alpha-native
- centered on subscription lifecycle rather than raw backend objects

### Response expectations

Subscription responses should emphasize:

- subscription name or identifier
- customer
- plan
- lifecycle status
- billing interval
- start/effective state
- payment setup status
- timestamps

Responses should avoid:

- raw provider checkout internals
- backend-specific subscription jargon when Alpha can abstract it cleanly

---

## UI Scope

### Subscriptions list

Must show:

- customer
- plan
- subscription status
- payment setup status
- next action if attention is required

Must support:

- create
- inspect detail

### Subscription create

Must optimize for:

- selecting the customer
- selecting the plan
- setting basic subscription terms
- deciding whether to request payment setup immediately

Should avoid:

- overwhelming technical or financial configuration upfront

### Subscription detail

Must provide:

- summary first
- customer and plan context
- current payment setup state
- primary actions for request/resend setup
- clear explanation when the payer still needs to act

Advanced sections may include:

- more detailed backend or provider state
- event or verification history

but should not dominate the page.

### Customer detail relationship

Customer detail should surface subscription-linked payment setup state, but subscription detail remains the primary lifecycle surface.

---

## UX Notes

This slice is high risk for actor confusion.

Guardrails:

1. Do not imply that operators should add cards or bank accounts themselves.
2. Make it obvious when the next action belongs to the payer.
3. Keep the hosted payment setup flow externalized and safe.
4. Treat payment setup as a lifecycle state, not as a hidden technical integration step.
5. Keep `Payments` as a later visibility and operations surface, not the place where subscription activation begins.

Preferred framing:

- Subscription = what the customer is signing up for
- Payment setup = how the payer completes billing readiness

That is much clearer than exposing raw checkout or provider mechanics.

---

## Testing Requirements

### Backend

- service tests for subscription creation and update
- orchestration tests for request/resend payment setup
- state transition tests for payment-method status reconciliation
- API tests for:
  - tenant scoping
  - reader vs writer behavior
  - validation rules
  - create/detail/list flows
  - payment setup request/resend actions

### UI

- tenant session route coverage
- subscriptions list/create/detail flow coverage
- payment setup request and resend coverage
- state rendering coverage for:
  - not requested
  - pending
  - ready
  - action required

### Live or staging verification

Once staging is ready for this slice, verify:

- subscription creation in Alpha
- payment setup request from Alpha
- payer-completed hosted setup path
- payment-method status returning to Alpha correctly

---

## Exit Criteria

This slice is complete when:

1. Tenant operators can create and inspect subscriptions in Alpha.
2. Tenant operators can request or resend payment setup from Alpha.
3. Payers complete payment-method linking through a secure externalized flow.
4. Alpha correctly reflects payment setup status without requiring Lago UI.
5. The user journey is simple and does not make payment-method capture feel like an internal admin operation.
