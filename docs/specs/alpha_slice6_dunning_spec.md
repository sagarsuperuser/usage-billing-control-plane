# Slice 6 Spec: Dunning and Collections Policy

Purpose:
- make Alpha own revenue recovery policy instead of stopping at lifecycle hints and manual retry buttons

Read with:
- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Capability Ownership Map](../models/alpha_capability_ownership_map.md)
- [Alpha Notification Architecture](../models/alpha_notification_architecture.md)
- [Slice 4 Spec: Invoices Visibility](./alpha_slice4_invoices_spec.md)
- [Slice 5 Spec: Payments Visibility](./alpha_slice5_payments_spec.md)
- [End-to-End Product Journeys](../runbooks/end-to-end-product-journeys.md)

---

## Why This Slice Exists

Alpha already has:
- payment lifecycle classification
- retry-payment actions
- collect-payment workflow
- overdue and failure visibility

That is useful, but it is not dunning.

Today Alpha can tell an operator:
- `retry_payment`
- `collect_payment`
- `investigate`

What Alpha cannot do yet is own the timed collections workflow that follows.

Without this slice:
- revenue recovery still depends on operator memory and manual action
- Alpha remains weaker than serious billing products on collections maturity
- the product boundary is incomplete even if Lago remains hidden from end users

---

## Product Goal

Alpha must become the owner of dunning policy and collections workflow.

For operators and customers, this should look entirely Alpha-owned:
- Alpha decides when recovery steps happen
- Alpha records which step is active
- Alpha decides when to retry, remind, or escalate
- Alpha shows the current collections status in normal payment and invoice surfaces
- Alpha exposes pause/resume/manual override controls

Lago may remain an execution backend for some billing-delivery mechanics initially, but it should not own the product workflow.

---

## Scope

Dunning v1 scope:
- one Alpha-owned dunning policy model
- one per-invoice dunning state machine
- scheduled retry and reminder execution
- operator visibility and basic controls
- Alpha-owned notification intent and audit
- integration with existing invoice/payment/customer surfaces

Out of scope for v1:
- multiple customer-segment-specific campaigns
- AI or adaptive retry timing
- account suspension / entitlement enforcement
- multi-channel outreach beyond email and product state
- full replacement of Lago collections execution
- collections analytics dashboards beyond basic history and status

---

## Product Model

### Core principle

Dunning is an Alpha workflow layer over invoice and payment execution state.

That means:
- Alpha does not replace canonical payment execution truth
- Alpha does own the policy and orchestration for recovery steps
- Alpha uses invoice and payment execution state as inputs to a dunning state machine

### Entities

#### 1. Dunning policy

Represents the configured collections policy Alpha applies.

Minimum fields for v1:
- `id`
- `tenant_id`
- `name`
- `enabled`
- `retry_schedule`
  - ordered delays, for example `1d`, `3d`, `5d`
- `max_retry_attempts`
- `collect_payment_reminder_schedule`
- `final_action`
  - `manual_review`
  - `pause`
  - `write_off_later` placeholder if needed
- `grace_period_days`
- `created_at`
- `updated_at`

Initial simplification:
- one active policy per tenant

#### 2. Invoice dunning run

Represents one invoice currently under collections policy.

Minimum fields for v1:
- `id`
- `tenant_id`
- `invoice_id`
- `customer_external_id`
- `policy_id`
- `state`
- `reason`
- `attempt_count`
- `last_attempt_at`
- `next_action_at`
- `next_action_type`
- `paused`
- `resolved_at`
- `resolution`
- `created_at`
- `updated_at`

#### 3. Dunning event log

Append-only history of actions and decisions.

Examples:
- `dunning_started`
- `retry_scheduled`
- `retry_attempted`
- `retry_succeeded`
- `retry_failed`
- `collect_payment_requested`
- `payment_reminder_sent`
- `payment_setup_pending`
- `payment_setup_ready`
- `paused`
- `resumed`
- `escalated`
- `resolved`

---

## State Machine

Suggested v1 states:
- `scheduled`
- `retry_due`
- `awaiting_payment_setup`
- `awaiting_retry_result`
- `resolved`
- `paused`
- `escalated`
- `exhausted`

Suggested transitions:

1. failed finalized invoice enters dunning
- initial state based on readiness and lifecycle
- if customer has usable payment setup: `retry_due`
- if customer lacks usable payment setup: `awaiting_payment_setup`

2. scheduled retry fires
- execute retry through existing Alpha invoice/payment action path
- move to `awaiting_retry_result`

3. successful payment webhook converges
- move to `resolved`

4. failed retry webhook converges
- if attempts remain: schedule next retry and return to `scheduled`
- if attempts exhausted: `exhausted` or `escalated`

5. collect-payment reminder sent while no usable method exists
- stay in `awaiting_payment_setup`
- advance next reminder time

6. payment setup becomes ready
- move to `retry_due`

7. operator pause
- move to `paused`

8. operator resume
- return to `scheduled` or `retry_due` based on current readiness

---

## Policy Rules

Dunning v1 should use explicit deterministic rules.

### Start condition

Start dunning only when all are true:
- invoice is `finalized`
- payment status is failed, overdue, or otherwise action-required
- invoice is not already resolved, voided, or succeeded
- no active dunning run already exists for the invoice

### Retry branch

Use when:
- customer payment setup is `ready`
- lifecycle recommendation is compatible with retry

Action:
- schedule retry according to policy
- record attempt history

### Collect-payment branch

Use when:
- customer payment setup is missing or pending
- lifecycle recommendation is `collect_payment`

Action:
- send payment setup reminder intent
- surface customer action required status
- continue reminders until setup becomes ready or policy exhausts

### Resolution

Resolve when:
- payment succeeds
- invoice is voided
- invoice is otherwise no longer collectible
- operator manually resolves or escalates

---

## Backend Scope

### New Alpha service boundary

Add a first-class service such as:
- `DunningService`

Responsibilities:
- start runs for eligible invoices
- evaluate next action based on policy + readiness + lifecycle
- schedule retry or reminder execution
- persist state transitions and event history
- expose operator-facing read models
- resolve runs when invoice/payment state changes

### New persistence

Likely new tables:
- `dunning_policies`
- `invoice_dunning_runs`
- `invoice_dunning_events`

### New scheduler / executor

Alpha needs background execution for:
- discovering eligible invoices
- dispatching due actions
- reconciling results after retry/reminder actions

Good v1 approach:
- scheduled worker path in Alpha
- reuse existing job infrastructure style already used for other async operational work

### Integration points

Dunning should integrate with:
- existing invoice detail and payment detail APIs
- customer payment readiness state
- existing retry-payment action path
- notification service boundary
- webhook ingestion that updates invoice/payment status views

### Notification execution rule

Alpha owns notification intent.

For v1:
- Alpha should create a notification intent record for each reminder/escalation decision
- Alpha may delegate actual billing-email execution to Lago initially where that is already practical
- the product API and audit trail remain Alpha-owned

---

## API Scope

Add product-facing APIs such as:
- `GET /v1/dunning/policy`
- `PUT /v1/dunning/policy`
- `GET /v1/payments/{id}/dunning`
- `GET /v1/invoices/{id}/dunning`
- `GET /v1/dunning/runs`
- `POST /v1/dunning/runs/{id}/pause`
- `POST /v1/dunning/runs/{id}/resume`
- `POST /v1/dunning/runs/{id}/retry-now`
- `POST /v1/dunning/runs/{id}/collect-payment-now`
- `POST /v1/dunning/runs/{id}/resolve`

Compatibility rule:
- dunning should enrich invoice and payment detail responses
- operators should not have to open a separate deep admin console for basic collections actions

---

## UI Scope

### 1. Payment detail

Extend the existing payment detail surface with:
- dunning status
- next scheduled action
- attempt count
- last reminder / retry event
- pause/resume/manual retry controls

### 2. Invoice detail

Show the same dunning summary where it is commercially relevant:
- current collections state
- next action time
- reminder history summary

### 3. Customer detail

Expose customer collections context when relevant:
- invoices awaiting payment setup
- current payment readiness blocking dunning progression
- reminder/request history summary

### 4. Billing configuration

Add a tenant-level dunning policy surface:
- policy detail
- retry cadence
- reminder cadence
- final action
- enabled/disabled state

### 5. Optional list view

If needed after v1 detail integration:
- a dunning queue or collections list for operators

This should not be required before the detail surfaces are strong.

---

## Notification Model

Dunning should extend the Alpha notification boundary, not bypass it.

V1 notification types may include:
- `dunning.payment_failed`
- `dunning.payment_method_required`
- `dunning.retry_scheduled`
- `dunning.final_attempt`
- `dunning.escalated`

Each dispatch should capture:
- target invoice
- target customer
- policy step
- delivery backend
- delivery status
- audit metadata

---

## End-to-End Journey

Add a canonical journey after implementation:

### Dunning and collections journey

Purpose:
- prove policy-driven collections behavior, not only manual retry

Real flow:
1. create a collectible finalized invoice in failed state
2. verify dunning run starts under Alpha policy
3. if no payment setup exists, verify Alpha sends the expected collect-payment reminder intent
4. complete payment setup
5. verify the dunning run moves to retry scheduling
6. execute or wait for retry
7. verify success resolves the dunning run
8. verify operator surfaces show the full history in Alpha

Current state:
- `planned`

---

## Testing

Backend:
- policy validation
- run creation for eligible invoices only
- state transitions for retry and collect-payment branches
- pause/resume/manual override behavior
- resolution on payment success / voided invoice
- notification intent recording

Web:
- payment detail dunning summary
- invoice detail dunning summary
- policy configuration flow
- pause/resume/manual retry controls

Journey:
- real staging dunning journey after v1 implementation

---

## Delivery Order

Recommended implementation order:

1. persistence + service model
2. run creation and state transitions
3. scheduler/executor
4. notification intent layer
5. payment/invoice detail integration
6. policy configuration UI
7. staging journey automation

---

## Success Criteria

This slice is successful when:
- Alpha owns the collections policy rather than only showing lifecycle hints
- operators can see and control dunning state entirely in Alpha
- payment setup and retry are orchestrated by Alpha policy, not operator memory
- invoice and payment detail clearly show recovery progress
- end users and operators do not need Lago UI to understand or operate collections
