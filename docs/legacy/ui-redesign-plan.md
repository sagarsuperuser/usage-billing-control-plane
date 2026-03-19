# UI Redesign Plan

## Goal

Turn the current operator-grade control plane into a UI that feels like a real SaaS admin product without changing the backend model.

The backend is already good enough. The problem is presentation:

- too many backend concepts are exposed directly
- happy-path onboarding and recovery tooling are mixed together
- readiness is technically correct but not human-friendly

This redesign keeps the current APIs and splits the UX into:

1. guided flows for normal users
2. advanced panels for operators and recovery actions

## Product Position

Alpha should look like:

- the product surface
- the control plane
- the guided onboarding experience

It should not look like:

- a thin API console over backend endpoints
- a UI that assumes users understand Lago concepts

## Design Rules

1. Default to guided workflows.
2. Hide raw backend identifiers unless the user asks for advanced detail.
3. Show next actions, not just raw status.
4. Keep recovery actions available, but secondary.
5. Translate readiness into business language.
6. Keep the existing dark control-plane visual direction, but reduce visual noise in forms.

## Navigation Model

Top-level UI should be reorganized mentally as:

1. `Overview`
2. `Onboarding`
3. `Customers`
4. `Payments`
5. `Recovery`
6. `Explainability`

Implementation note:

- existing routes can stay for now
- labels and grouping should change before more backend work

## Overview Redesign

Current issue:

- the overview reads like a toolbox of operations modules

Target:

- make it read like a product dashboard with primary journeys

### New overview sections

1. `Get started`
- primary CTA cards
- `Create tenant`
- `Onboard first customer`

2. `Operations`
- `Payments`
- `Replay recovery`
- `Invoice explainability`

3. `Needs attention`
- tenants missing pricing
- tenants missing first customer
- customers with payment setup pending
- customers with billing sync errors

### Keep

- strong visual cards
- bold dark atmosphere

### Change

- reduce backend-heavy descriptions
- make card text outcome-oriented

Example:

- instead of `Platform-admin workflow for tenant create/reconcile, bootstrap-admin, and layered readiness inspection`
- use `Create a tenant workspace, connect billing, and hand off admin access`

## Tenant Onboarding Redesign

Current issue:

- the page is powerful but too dense
- it feels like bootstrap tooling, not SaaS onboarding

### New structure

Split into two tabs or stacked sections:

1. `Guided setup`
2. `Advanced ops`

### Guided setup

The default tenant onboarding experience should be a 4-step flow:

#### Step 1: Create workspace

Visible fields:

- `Tenant name`
- `Workspace ID`

Hidden under advanced:

- raw backend naming help

#### Step 2: Connect billing

Visible content:

- billing status
- a simple `Connected / Needs setup` state

Hidden under advanced:

- `lago_organization_id`
- `lago_billing_provider_code`

If Alpha still requires those today, keep them in an expandable section called:

- `Advanced billing settings`

#### Step 3: Create admin access

Visible content:

- `Generate first admin credential`
- one clear explanation of what the generated secret is for

When created:

- show it in a dedicated handoff card
- emphasize that it is shown once

#### Step 4: Review readiness

Show a clear checklist:

- `Workspace created`
- `Billing connected`
- `Admin access ready`
- `Pricing not configured yet`
- `First customer not created yet`

Do not show raw readiness keys by default.

### Advanced ops

Move these here:

- `Allow existing active tenant keys`
- raw Lago IDs
- full readiness missing-step tokens
- detailed tenant inventory and status filtering

### Existing API mapping

Guided setup still uses:

- `POST /internal/onboarding/tenants`
- `GET /internal/onboarding/tenants/{id}`
- `GET /internal/tenants`

No backend redesign is required for this UI pass.

## Customer Onboarding Redesign

Current issue:

- onboarding, recovery, sync details, and status inspection all compete on one screen

### New structure

Split into:

1. `Guided customer setup`
2. `Advanced customer diagnostics`

### Guided customer setup

This should become a 4-step experience:

#### Step 1: Customer identity

Visible fields:

- `Customer name`
- `Billing email`

Optional advanced field:

- `Customer external ID`

The current external ID is important to the backend, but not all users should have to think about it first.

#### Step 2: Billing profile

Visible fields:

- legal name
- billing address
- currency

Hide provider-specific wording.

Instead of `Provider code`, use:

- `Billing connection`

If a raw provider code is still required, keep it behind an advanced section.

#### Step 3: Payment setup

Primary action:

- `Start payment setup`

Explain:

- what the customer will do next
- whether checkout will open immediately

#### Step 4: Readiness

Show plain-language states:

- `Customer created`
- `Billing profile synced`
- `Payment setup pending`
- `Payment method verified`

### Advanced customer diagnostics

Move these here:

- `Retry billing sync`
- `Refresh payment setup`
- `Lago customer ID`
- last sync error
- last sync time
- last verification time
- raw missing-step tokens

### Existing API mapping

Guided flow uses:

- `POST /v1/customer-onboarding`
- `GET /v1/customers/{id}/readiness`

Advanced recovery uses:

- `POST /v1/customers/{id}/billing-profile/retry-sync`
- `POST /v1/customers/{id}/payment-setup/refresh`

## Readiness Language Plan

Readiness must be translated before it is shown in the primary UI.

### Tenant examples

Map:

- `billing_integration.pricing`
to:
- `Pricing rules still need to be configured`

Map:

- `first_customer.customer_created`
to:
- `No billing-ready customer has been created yet`

### Customer examples

Map:

- `payment_setup_ready`
to:
- `Customer has not completed payment setup`

Map:

- `default_payment_method_verified`
to:
- `Default payment method has not been verified yet`

Raw keys may still be visible in advanced mode.

## Layout Rules

### Keep

- current dark gradients
- large rounded cards
- strong icons
- role-aware navigation

### Improve

- reduce the number of equal-weight panels on first load
- make the happy path the dominant visual block
- push diagnostics below the fold or behind disclosure
- add stronger `next action` messaging

## Implementation Order

### Phase 1

Redesign labels and hierarchy only:

- overview copy
- nav grouping/copy
- human-readable readiness labels

### Phase 2

Refactor tenant onboarding UI into:

- guided setup
- advanced ops

### Phase 3

Refactor customer onboarding UI into:

- guided setup
- advanced diagnostics

### Phase 4

Add `Needs attention` and progress-style summary widgets to overview

## What Not To Do

1. Do not redesign backend APIs for this pass.
2. Do not remove advanced operator controls.
3. Do not expose more Lago terminology in the primary flow.
4. Do not add marketing-site polish before the onboarding information architecture is fixed.

## Result

After this redesign, Alpha should feel like:

- a guided SaaS admin product on the surface
- an operator-capable control plane underneath

That is the right long-term UI shape for the current backend.
