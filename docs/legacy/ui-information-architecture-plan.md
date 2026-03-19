# UI Information Architecture Plan

## Goal

Correct the product structure of the control plane UI.

The current problem is not only styling. The main issue is that onboarding screens are trying to do too many jobs at once:

- create
- browse inventory
- inspect readiness
- recover from failures
- expose advanced metadata

That structure is not production-grade long term. It creates layout instability, cognitive overload, and weak task hierarchy.

The long-term product shape should separate user intent into distinct surfaces.

## Product Rule

Do not mix these concerns on one page unless there is a strong reason:

1. create
2. browse
3. inspect
4. recover

Instead, use:

1. `List`
2. `Setup`
3. `Detail`

This is the baseline information architecture for the control plane.

## Recommended Navigation

### Platform session

Top-level navigation:

1. `Overview`
2. `Workspaces`
3. `Workspace Setup`

Platform users should not see tenant-only tools like payments, replay recovery, or invoice explainability in the primary nav.

### Tenant session

Top-level navigation:

1. `Overview`
2. `Customers`
3. `Customer Setup`
4. `Payments`
5. `Recovery`
6. `Explainability`

## Route Map

### Platform scope

- `/control-plane`
  - role-aware overview
- `/workspaces`
  - workspace list
- `/workspaces/new`
  - workspace setup
- `/workspaces/[id]`
  - workspace detail

### Tenant scope

- `/control-plane`
  - tenant overview
- `/customers`
  - customer list
- `/customers/new`
  - customer setup
- `/customers/[externalId]`
  - customer detail
- `/payment-operations`
  - tenant payment operations
- `/replay-operations`
  - tenant replay and recovery
- `/invoice-explainability`
  - tenant invoice explainability

## Page Responsibilities

### Overview

Purpose:

- home screen
- role-aware summary
- attention counts
- primary next action

Should contain:

- top metrics
- needs-attention section
- primary CTA
- links to the correct next workflow for the current scope

Should not contain:

- heavy editing
- detailed recovery controls
- dense object review panels

### Workspaces List

Purpose:

- browse all workspaces
- filter/search/status scan
- identify what needs action

Should contain:

- searchable list or table
- workspace status
- billing connected state
- pricing readiness
- first-customer readiness
- row click to workspace detail

Should not contain:

- tenant creation form
- bootstrap secret handoff
- advanced recovery

### Workspace Setup

Purpose:

- create or reconcile a workspace
- connect billing
- optionally create first admin credential

Should contain:

- guided setup form only
- clear form sections
- success state
- redirect to workspace detail after success

Should not contain:

- workspace inventory
- selected workspace review
- side-by-side readiness console
- advanced recovery blocks

### Workspace Detail

Purpose:

- review readiness
- inspect metadata
- understand next actions
- perform platform-scoped follow-up

Should contain:

- workspace summary header
- readiness sections
- next actions
- billing mapping summary
- created/updated metadata
- admin bootstrap handoff state when relevant
- optional advanced details section

This page is where the current right-side review panel should move.

### Customers List

Purpose:

- browse customers in a tenant
- filter/search by status and readiness

Should contain:

- customer list/table
- billing sync status
- payment readiness status
- row click to customer detail

Should not contain:

- customer creation form
- payment recovery controls inline with list browsing

### Customer Setup

Purpose:

- create customer
- apply billing profile
- optionally start payment setup

Should contain:

- guided form only
- success state
- redirect to customer detail after success

Should not contain:

- customer inventory
- selected customer diagnostics
- retry/refresh controls in the main form view

### Customer Detail

Purpose:

- inspect readiness
- inspect billing sync state
- inspect payment setup state
- perform recovery actions

Should contain:

- customer summary header
- readiness summary
- billing profile sync state
- payment setup state
- retry billing sync
- refresh payment setup
- raw IDs, timestamps, and advanced diagnostics in secondary sections

This page is where the current right-side customer review panel should move.

## What Moves Out Of The Current Screens

### Move out of `tenant-onboarding`

Remove from the setup page:

- workspace inventory list
- selected workspace panel
- readiness review cards
- next actions panel
- advanced workspace detail card

Keep on the setup page:

- workspace identity step
- billing connection step
- admin credential creation step
- one success handoff state

### Move out of `customer-onboarding`

Remove from the setup page:

- customer inventory list
- selected customer readiness panel
- billing sync retry controls
- payment refresh controls
- advanced diagnostics section

Keep on the setup page:

- customer identity step
- billing profile step
- payment setup step
- success handoff state

## Role Behavior

### Platform admin

Can access:

- overview
- workspaces list
- workspace setup
- workspace detail

Should not use tenant-only surfaces from the platform session.

### Tenant reader

Can access:

- overview
- customers list
- customer detail
- payment operations
- replay recovery
- explainability

Cannot run write actions.

### Tenant writer/admin

Can access:

- everything tenant reader can
- customer setup
- retry billing sync
- refresh payment setup
- payment write actions

## API Mapping

This IA change does not require a backend redesign.

### Workspace list/detail/setup

Use the existing APIs:

- `GET /internal/tenants`
- `POST /internal/onboarding/tenants`
- `GET /internal/onboarding/tenants/{id}`
- `POST /internal/tenants/{id}/bootstrap-admin-key` when needed

### Customer list/detail/setup

Use the existing APIs:

- `GET /v1/customers`
- `POST /v1/customer-onboarding`
- `GET /v1/customers/{external_id}`
- `GET /v1/customers/{external_id}/readiness`
- `POST /v1/customers/{external_id}/billing-profile/retry-sync`
- `POST /v1/customers/{external_id}/payment-setup/refresh`
- `POST /v1/customers/{external_id}/payment-setup/checkout-url`

## Implementation Order

### Phase 1: Navigation and route model

1. add `Workspaces` list route
2. add `Customers` list route
3. update nav labels and role-specific grouping
4. keep current onboarding routes temporarily

### Phase 2: Workspace detail

1. build `/workspaces/[id]`
2. move current readiness/review content there
3. keep `tenant-onboarding` focused on setup only
4. redirect successful setup to workspace detail

### Phase 3: Customer detail

1. build `/customers/[externalId]`
2. move current readiness/diagnostics/recovery content there
3. keep `customer-onboarding` focused on setup only
4. redirect successful setup to customer detail

### Phase 4: Lists

1. build `/workspaces`
2. build `/customers`
3. move inventory/filtering out of setup pages

### Phase 5: Rename old routes cleanly

1. migrate nav to `/workspaces/new`
2. migrate nav to `/customers/new`
3. keep temporary redirects from old onboarding routes if useful during transition

## Acceptance Criteria

The IA is correct when:

1. setup pages do one job only
- create/reconcile through a guided flow

2. list pages do one job only
- browse and filter records

3. detail pages do one job only
- inspect readiness, diagnostics, and recovery

4. no critical content depends on a narrow side panel surviving desktop compression

5. platform and tenant scopes are obvious from navigation alone

6. users do not discover role boundaries by hitting random `403` responses

## Immediate Next Step

Do not keep patching the combined onboarding pages as the long-term answer.

Build this sequence next:

1. `/workspaces`
2. `/workspaces/[id]`
3. simplify `tenant-onboarding`
4. `/customers`
5. `/customers/[externalId]`
6. simplify `customer-onboarding`

That is the correct production-grade direction.
