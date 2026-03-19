# Alpha Slice 4 Spec: Invoices Visibility

This document defines the fourth Wave 1 implementation slice for Alpha: a product-grade invoices visibility surface.

Read together with:

- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Import Matrix](../goals/alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](../roadmaps/alpha_wave1_roadmap.md)
- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](../specs/alpha_slice3_subscriptions_spec.md)

---

## Objective

Alpha must let tenant operators browse invoices in Alpha without depending on Lago UI or advanced payment-ops tooling.

For Wave 1, that means Alpha should support:

- invoices list
- invoice detail
- invoice-to-customer navigation
- clear status, amount, and due-state visibility
- reuse of existing retry-payment and explainability actions from invoice detail when needed

This slice intentionally focuses on normal invoice visibility first. It does not attempt to deliver every invoice mutation or every back-office finance workflow.

---

## Why This Slice Comes Before Payments

Invoices should land before a normal payments surface.

Current code reality already gives Alpha:

- invoice detail fetch capability through the invoice billing adapter
- invoice explainability
- retry-payment action on invoice detail
- invoice payment status projections and events for advanced operations

What Alpha does not have yet is the normal user-facing product surface:

- invoices list
- invoice detail page in the tenant product IA
- clean customer-to-invoice navigation

Payments should build after this slice because current payment visibility is still mostly an operations and projection surface, not a primary product-grade payments domain.

---

## Product Scope

### In scope

- `Invoices` tenant domain
- invoices list
- invoice detail
- filter, sort, and pagination
- customer-to-invoice navigation
- invoice summary data such as:
  - customer
  - invoice number or identifier
  - invoice status
  - payment status
  - due date
  - total amount
  - outstanding amount
- invoice detail actions for:
  - retry payment
  - explainability
  when those are relevant and already supported

### Out of scope

- manual invoice creation
- void/regenerate
- credit notes
- tax-document administration
- full payments list/detail
- invoice lifecycle editing beyond existing retry-payment support
- exposing raw Lago invoice payloads as the primary UX

---

## User Stories

1. A tenant operator can browse invoices in Alpha without opening Lago UI.
2. A tenant operator can filter invoices by customer, invoice status, payment status, and due state.
3. A tenant operator can open invoice detail and understand what the invoice is, who it belongs to, and what financial action is pending.
4. A tenant operator can move from a customer to that customer's invoices and back.
5. A tenant operator can reach advanced actions such as retry payment or explainability from invoice detail without those actions dominating the normal list experience.

---

## Product Rules

1. `Invoices` is a normal tenant product surface, not an operator-only diagnostics page.
2. The list should optimize for scanning and triage, not raw payload inspection.
3. Invoice detail should show the commercial and operational story first.
4. Advanced actions such as retry-payment and explainability belong on detail pages, not on the main list.
5. Alpha should use product language such as:
   - `draft`
   - `finalized`
   - `paid`
   - `pending`
   - `overdue`
   - `action required`
6. Avoid exposing Lago-specific invoice jargon unless there is direct product value.

---

## Target Product Surface

### Routes

- `/invoices`
- `/invoices/[id]`

### Navigation placement

- one top-level tenant nav item: `Invoices`

### Primary actions

- browse invoices
- inspect invoice detail
- move to linked customer

### Secondary actions

- retry payment
- open explainability
- move into advanced payment operations only when needed

---

## Backend Scope

### Domain boundary

Alpha should expose an invoice visibility boundary that presents Alpha-native invoice summaries and detail even if detail fetches and payment retries still route through Lago-backed execution underneath.

### Required backend work

1. Invoice list API
- list invoices
- tenant-scoped filtering
- sort and pagination
- customer linkage
- normalized invoice and payment status fields

2. Invoice detail API
- stable Alpha-owned detail response
- invoice/customer linkage
- amount and due-state summary
- important timestamps
- action affordances where already supported

3. Existing action reuse
- keep retry-payment on invoice detail
- keep explainability as a detail sub-surface
- do not duplicate advanced payment-ops behavior into the base invoice list

4. Permission model
- tenant-scoped only
- reader can inspect
- writer/admin can trigger retry-payment

### Suggested APIs

Recommended Wave 1 route shape:

- `GET /v1/invoices`
- `GET /v1/invoices/{id}`
- keep existing:
  - `POST /v1/invoices/{id}/retry-payment`
  - `GET /v1/invoices/{id}/explainability`

### Suggested query model for list

Support filters such as:

- `customer_id`
- `invoice_status`
- `payment_status`
- `overdue`
- `sort_by`
- `order`
- `limit`
- `offset`

### Response expectations

List responses should emphasize:

- invoice id / invoice number
- customer summary
- invoice status
- payment status
- due date
- total amount
- outstanding amount
- updated timestamps

Detail responses should additionally include:

- issued/finalized timestamps
- currency
- line-item summary where cheaply available
- payment error summary if relevant
- links to explainability and retry-payment actions

Avoid:

- dumping raw backend invoice payloads as the normal response contract
- forcing the UI to infer meaning from low-level provider fields

---

## Current Implementation Starting Point

This slice should build on existing capabilities already present in Alpha:

- invoice detail fetch via the invoice billing adapter
- invoice explainability service and UI
- retry-payment API
- invoice payment status projections, summary, lifecycle, and event timelines
- payment operations console for advanced triage

That means this slice is not starting from zero. The main missing work is to turn those primitives into a normal `Invoices` product surface.

---

## UI Scope

### Invoices list

Must show:

- invoice identifier
- customer
- invoice status
- payment status
- due date
- total amount
- outstanding amount when applicable

Must support:

- filtering
- sort
- pagination
- open detail
- clear empty state

Should avoid:

- showing explainability or event timelines inline
- feeling like a diagnostics console

### Invoice detail

Must provide:

- summary first
- customer context
- invoice and payment status
- key amounts and dates
- next action when attention is required

Advanced sections may include:

- explainability entrypoint
- payment retry action
- more detailed finance state

but those should remain secondary.

### Customer relationship

Customer detail should link into invoices, but invoices detail remains the primary invoice lifecycle surface.

---

## UX Notes

This slice should make invoices feel routine and trustworthy.

Guardrails:

1. Prefer calm financial summaries over ops-console language.
2. Show `what happened`, `what is due`, and `what to do next` before diagnostics.
3. Keep advanced payment operations behind a clear secondary path.
4. Reuse the current admin-console UI direction, not glossy or diagnostic-heavy styling.

Preferred framing:

- Invoices = normal financial visibility
- Payments = separate visibility surface later
- Payment Operations = advanced exception handling
- Explainability = advanced detail, not the default entrypoint

---

## Testing Requirements

### Backend

Add coverage for:

- invoice list filtering
- pagination and sort behavior
- tenant scoping
- detail response normalization
- retry-payment permission behavior

### UI

Add coverage for:

- invoices list rendering
- filter and pagination state
- navigation from list to detail
- navigation from customer detail to invoices
- conditional visibility of retry-payment / explainability actions

### Manual/staging

Validate:

1. a tenant operator can browse invoices without needing Lago UI
2. invoice detail feels product-grade, not like a raw proxy
3. retry-payment and explainability remain reachable but secondary
4. invoice-to-customer navigation is obvious and stable

---

## Suggested Delivery Order Inside This Slice

1. Define invoice summary/detail DTOs and the list API contract.
2. Implement invoice list API with tenant filtering and pagination.
3. Normalize invoice detail response shape around existing detail fetch capability.
4. Build `/invoices` list and `/invoices/[id]` detail screens.
5. Wire customer-to-invoice navigation.
6. Attach existing retry-payment and explainability actions as detail-page secondary actions.

---

## Follow-on Slice Dependency

After this slice lands, Slice 5 should deliver a normal `Payments` surface.

That later slice should:

- keep current payment operations as advanced workflows
- add payment list and detail as normal tenant pages
- link payments cleanly to invoices and customers

That sequencing keeps Alpha product IA clean:

- `Invoices` first for normal billing visibility
- `Payments` next for financial completion and collections visibility
- `Payment Operations` remains advanced exception handling
