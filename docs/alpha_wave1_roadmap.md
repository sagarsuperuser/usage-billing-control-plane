# Alpha Wave 1 Roadmap

This document turns `Wave 1` from the Alpha import matrix into a concrete implementation roadmap.

Use it to sequence delivery across:

- product
- backend
- UI
- auth/admin
- integration hardening

Read together with:

- [Alpha Import Goal](./alpha_import_goal.md)
- [Alpha Import Matrix](./alpha_import_matrix.md)
- [Alpha Workspace Access Model](./alpha-workspace-access-model.md)
- [Slice 1 Spec: Billing Connections Hardening](./alpha_slice1_billing_connections_spec.md)
- [Slice 2 Spec: Pricing Foundation](./alpha_slice2_pricing_spec.md)
- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](./alpha_slice3_subscriptions_spec.md)

---

## Wave 1 Goal

Wave 1 is complete when Alpha feels like a credible billing control plane, not a partial wrapper around Lago.

That means Alpha must own these user-visible domains:

- Pricing
- Subscriptions
- Invoices visibility
- Payments visibility
- Team & Security basics
- Billing Connections hardening

Wave 1 does **not** attempt to deliver full parity across every billing and admin surface.

Wave 1 is about:

- market table stakes
- clear product ownership
- simple, non-intimidating workflows
- stable IA that later waves can extend without rework

---

## Wave 1 Success Criteria

Alpha should support the following end-to-end stories:

1. A platform admin can sign in, configure a billing connection, and manage workspace access.
2. A tenant operator can define pricing primitives and plans inside Alpha.
3. A tenant operator can create and manage subscriptions inside Alpha.
4. A tenant operator can request payment setup from Alpha, and the payer can complete payment-method linking without exposing Lago UI.
5. A tenant operator can browse invoices and payments inside Alpha without needing Lago UI.
6. An organization admin can invite users, manage roles, and enforce browser-auth ownership through Alpha.
7. After workspace setup, Alpha can hand tenant ownership to an invited workspace admin without requiring backend-only pre-provisioning.

If any of those still require Lago UI, Wave 1 is not complete.

---

## Product Constraints

These constraints remain hard requirements during Wave 1:

1. No screen should feel like an operator dump.
2. Keep top-level navigation compact.
3. Use Alpha-native language, not Lago engine language.
4. Prefer `List / Setup / Detail` surfaces.
5. Put advanced operational actions on detail pages, not primary pages.
6. Keep role-specific complexity separated.

---

## Recommended Wave 1 Nav Model

### Platform nav

- Overview
- Billing Connections
- Workspaces
- Team & Security

### Tenant nav

- Overview
- Customers
- Pricing
- Subscriptions
- Invoices
- Payments
- Recovery
- Explainability

Notes:

- `Pricing` should absorb billable metrics and plans at first
- avoid separate top-level nav for every billing object
- `Recovery` and `Explainability` stay advanced but available

---

## Delivery Order

The order matters. Build the seams first, then the main user flows, then the admin completion layer.

### Slice 1. Billing Connections hardening

Purpose:

- finish the provider-connect foundation so Alpha can credibly own billing connectivity

Backend:

- finalize Stripe connection lifecycle
- improve status modeling
- improve sync failure visibility
- clarify secret rotation limitations and fallback handling
- ensure workspace assignment through billing connection is fully stable

UI:

- strengthen Billing Connections list/detail/create UX
- show connection health, last sync result, and current status clearly
- make workspace assignment consume only Alpha-owned connection concepts

Outcome:

- Alpha clearly owns billing connectivity

Detailed implementation spec:

- [Slice 1 Spec: Billing Connections Hardening](./alpha_slice1_billing_connections_spec.md)

### Slice 2. Pricing domain foundation

Purpose:

- create the first real tenant-side billing product surface beyond customer setup

Wave 1 scope:

- billable metrics
- plans

Not in this slice:

- add-ons
- coupons
- taxes
- features

Backend:

- pricing domain models
- CRUD APIs for billable metrics
- CRUD APIs for plans
- validation and relationship rules
- clear Alpha-owned service boundary over Lago-backed execution

UI:

- `Pricing` landing page
- billable metrics list/create/detail
- plans list/create/detail
- simple flows and copy
- no advanced fee-shape sprawl in the first version unless necessary

Outcome:

- tenant operators can define the core pricing backbone inside Alpha

Detailed implementation spec:

- [Slice 2 Spec: Pricing Foundation](./alpha_slice2_pricing_spec.md)

### Slice 3. Subscriptions domain

Purpose:

- let Alpha own the core recurring billing workflow

Wave 1 scope:

- subscriptions list
- subscription detail
- create subscription
- update subscription
- request payment setup from the subscription flow
- resend payment setup request
- track payer-completed payment method status from subscription and customer detail

Not in this slice:

- alerts
- entitlements
- upgrade/downgrade optimization UI beyond a basic edit path

Backend:

- subscription read/write APIs
- customer-plan linkage
- hosted payment-setup request orchestration
- payment-method status model and webhook/state reconciliation
- basic lifecycle validation
- status and effective-period handling

UI:

- subscriptions list
- create subscription flow
- subscription detail
- edit flow from detail
- payment setup request/resend/status actions on subscription and customer detail

Outcome:

- Alpha owns a core subscription lifecycle without relying on Lago UI
- payment-method linking is payer-completed, not operator-driven

Detailed implementation spec:

- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](./alpha_slice3_subscriptions_spec.md)

Priority note:

- customer-owned payment-method setup belongs with `Subscriptions`, before normal payments visibility
- operators should initiate and track payment setup, but the payer should complete card or bank linking
- `Payments` remains a visibility and operations surface after payer setup exists

### Slice 4. Invoices visibility

Purpose:

- give tenant operators direct financial visibility in Alpha

Wave 1 scope:

- invoices list
- invoice detail

Not in this slice:

- manual invoice creation
- void/regenerate
- credit notes

Backend:

- invoice read/query APIs
- filter/sort/pagination
- customer/invoice linking

UI:

- invoice list
- invoice detail
- invoice-to-customer navigation
- clear status and amount presentation

Outcome:

- core invoice visibility exists fully in Alpha

### Slice 5. Payments visibility

Purpose:

- complement existing payment ops with a normal payments product surface

Wave 1 scope:

- payments list
- payment detail

Keep:

- existing payment operations / recovery as advanced flows

Backend:

- payment read/query APIs
- payment detail APIs
- normalized status mapping

UI:

- payments list
- payment detail
- clear linkages to invoice/customer
- route to advanced operations only when needed

Outcome:

- Alpha has a normal product-grade payments surface, not only an ops console

### Slice 6. Team & Security basics

Purpose:

- make Alpha truly own browser access and org administration

Wave 1 scope:

- members list
- invitations
- roles list
- role assignment

Not in this slice:

- full auth-provider admin UX beyond the current SSO foundation

Backend:

- membership APIs
- invitation APIs
- role APIs
- user-role assignment APIs
- email/invite lifecycle support

UI:

- Team & Security landing
- members list/actions
- invite flow
- roles list/detail/edit

Outcome:

- Alpha owns the core organization admin surface

---

## Suggested Backend Workstreams

To avoid coupling everything into one stream, use these workstreams in parallel where possible.

### Workstream A. Billing domain read models

Use for:

- pricing
- subscriptions
- invoices
- payments

Deliver:

- stable Alpha service boundaries
- read/query endpoints
- pagination/filter/sort conventions
- domain DTO consistency

### Workstream B. Billing domain write flows

Use for:

- metric creation
- plan creation
- subscription creation/update

Deliver:

- validation rules
- transactional workflow boundaries
- adapter calls to Lago-backed execution when needed

### Workstream C. Browser admin foundation

Use for:

- memberships
- invites
- roles
- future auth settings

Deliver:

- org admin APIs
- permission enforcement
- invite lifecycle and mail hooks

### Workstream D. Billing connections hardening

Use for:

- Stripe lifecycle
- status model
- sync UX
- provider abstraction hardening

---

## Suggested UI Workstreams

### UI Stream 1. Pricing

Pages:

- `/pricing`
- `/pricing/metrics`
- `/pricing/metrics/new`
- `/pricing/metrics/[id]`
- `/pricing/plans`
- `/pricing/plans/new`
- `/pricing/plans/[id]`

### UI Stream 2. Subscriptions

Pages:

- `/subscriptions`
- `/subscriptions/new`
- `/subscriptions/[id]`

### UI Stream 3. Finance visibility

Pages:

- `/invoices`
- `/invoices/[id]`
- `/payments`
- `/payments/[id]`

### UI Stream 4. Team & Security

Pages:

- `/team-security`
- `/team-security/members`
- `/team-security/roles`
- `/team-security/roles/[id]`

### UI Stream 5. Billing Connections refinement

Pages:

- `/billing-connections`
- `/billing-connections/new`
- `/billing-connections/[id]`

---

## Recommended Execution Sequence

This is the recommended build order.

### Phase A

- Billing Connections hardening
- Pricing backend foundation
- Team & Security backend foundation

### Phase B

- Pricing UI
- Subscriptions backend
- Team & Security UI

### Phase C

- Subscriptions UI
- Invoices read APIs and UI
- Payments read APIs and UI

### Phase D

- cross-domain polish
- role/permission hardening
- empty/success/error states
- staging validation

---

## Testing Expectations

Wave 1 should not ship on happy-path demos only.

Each slice should include:

### Backend

- unit tests for service behavior
- integration tests for persistence and boundary contracts
- API tests for permissions and validation

### UI

- session and permission coverage
- list/create/detail route coverage
- wrong-role and empty-state coverage

### Staging

- end-to-end smoke for:
  - billing connection setup
  - pricing object creation
  - subscription creation
  - invoice visibility
  - payment visibility
  - invite and role flows

---

## What Stays Out Of Wave 1

Explicitly keep these out unless a dependency forces them in:

- add-ons
- coupons
- taxes UI beyond what is essential for immediate billing correctness
- credit notes
- wallets
- dunning campaigns
- billing entities
- customer portal
- broad analytics suite
- developer tooling expansion
- broad multi-provider integration coverage beyond Stripe-first maturity

Those are important, but they are not needed to make Alpha credibly own the control plane first.

---

## Exit Criteria

Wave 1 is done when:

1. Alpha can handle core billing setup, pricing, subscriptions, invoices, and payments without sending users to Lago UI.
2. Alpha owns core browser user administration for organizations.
3. The main user flows are simple, role-aware, and non-intimidating.
4. Lago is functionally behind Alpha for those surfaces.

That is the standard.
