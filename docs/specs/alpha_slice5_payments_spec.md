# Slice 5 Spec: Payments Visibility

Purpose:
- turn Alpha payments from an advanced operations console into a normal tenant product surface

Read with:
- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Wave 1 Roadmap](../roadmaps/alpha_wave1_roadmap.md)
- [Slice 4 Spec: Invoices Visibility](./alpha_slice4_invoices_spec.md)
- [Alpha Notification Architecture](../models/alpha_notification_architecture.md)

---

## Why This Slice Exists

Alpha already has the underlying payment-status projection and retry/recovery mechanics.

What is missing is the normal tenant-facing product surface:

- `/payments`
- `/payments/{id}`

Without that, payments still feel like an operator-only area instead of a standard part of the billing product.

---

## Scope

Wave 1 scope:

- payments list
- payment detail
- invoice/customer linkage
- retry action from payment detail
- recent event timeline for payment history
- clear action-required and overdue signaling

Out of scope for this slice:

- a separate canonical payment object independent of invoices
- manual payment collection flows
- refunds
- chargebacks
- payout or settlement accounting

---

## Product Model

For Wave 1, `Payments` is invoice-centric.

That means:

- each payment row is keyed by the invoice payment state Alpha already projects
- Alpha does not pretend there is a separate stable payment entity when the backend does not have one yet
- the route `/payments/{id}` uses the invoice identifier as the payment reference for this slice

This is the correct production tradeoff for now.

It keeps the product honest while still delivering a first-class payments surface.

---

## Backend Scope

Add product-facing APIs:

- `GET /v1/payments`
- `GET /v1/payments/{id}`
- `GET /v1/payments/{id}/events`
- `POST /v1/payments/{id}/retry`

Implementation rule:

- build on top of the existing invoice payment status projection and lifecycle analysis
- keep `/v1/invoice-payment-statuses*` as the advanced compatibility seam underneath

The product-facing payment DTO should expose:

- invoice reference
- customer reference
- payment status
- overdue state
- amount due / paid / total
- last payment error
- last event type and time
- lifecycle recommendation and action signal on detail

---

## UI Scope

Add tenant routes:

- `/payments`
- `/payments/{id}`

List requirements:

- filter by customer, invoice status, payment status, overdue state
- sort by last event, updated time, amount due, total amount
- show visible counts for failed, overdue, and action-required rows
- make invoice and customer linkage obvious

Detail requirements:

- show current payment and invoice state
- show due / paid / total amounts
- show lifecycle recommendation
- show recent payment-related webhook events
- allow retry when the session has write capability
- link back to invoice and customer detail

Compatibility rule:

- old `/payment-operations` should no longer be the primary navigation path
- it may redirect to `/payments` during migration

---

## Testing

Backend:

- payment list returns normalized payment summaries
- payment detail returns normalized payment detail with lifecycle
- payment events route returns invoice-scoped payment events
- retry route preserves writer/admin enforcement

Web:

- `typecheck`
- `lint`
- route renders for `/payments` and `/payments/{id}`

---

## Follow-on Dependency

After this slice:

- keep `Recovery` for deeper replay/reconciliation workflows
- later waves may introduce a true separate payment domain only if the backend identity model becomes real enough to justify it
