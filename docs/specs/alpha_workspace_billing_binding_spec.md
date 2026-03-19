# Alpha Workspace Billing Binding Spec

This document defines the next architecture step after the current billing-connection transition work.

Read together with:

- [Alpha Billing Execution Model](../models/alpha-billing-execution-model.md)
- [Alpha Provider Connect Plan](../legacy/alpha-provider-connect-plan.md)
- [Alpha Wave 1 Roadmap](../roadmaps/alpha_wave1_roadmap.md)

---

## Objective

Introduce a first-class `workspace billing binding` model so Alpha can:

- keep billing credential ownership separate from billing execution boundaries
- support shared execution today
- support dedicated execution later
- adapt to stronger tenant-isolation and compliance requirements without rewriting the billing domain

This is the architecture slice that converts the current transition model into a durable domain model.

---

## Product intent

Normal users should think in:

- workspace
- billing connection
- billing status
- payment setup

Normal users should not think in:

- Lago organization id
- Lago provider code
- backend execution tenancy

The binding exists so Alpha can keep those backend concepts internal.

---

## New domain concept

### `WorkspaceBillingBinding`

A `WorkspaceBillingBinding` represents the billing execution context for one Alpha workspace.

It answers:

- which billing credential is attached to this workspace
- which backend execution organization currently owns this workspace in Lago
- whether the workspace is running in shared or dedicated isolation mode
- whether the execution context has been provisioned, verified, or needs repair

### Core rule

One workspace has at most one active billing binding.

A billing connection can be reused across many bindings.

That is the key separation:

- `Billing connection` = credential + provider metadata
- `Workspace billing binding` = workspace execution boundary

---

## Proposed schema

### `workspace_billing_bindings`

Suggested fields:

- `id`
- `workspace_id`
- `billing_provider_connection_id`
- `backend`
- `backend_organization_id`
- `backend_provider_code`
- `isolation_mode`
- `status`
- `provisioning_error`
- `last_verified_at`
- `connected_at`
- `disabled_at`
- `created_by_type`
- `created_by_id`
- `created_at`
- `updated_at`

### Suggested enums

`backend`
- `lago`

`isolation_mode`
- `shared`
- `dedicated`

`status`
- `pending`
- `provisioning`
- `connected`
- `verification_failed`
- `disabled`

### Constraints

- unique active binding per workspace
- foreign key to `tenants.id`
- foreign key to `billing_provider_connections.id`
- nullable `backend_organization_id` during early provisioning only

---

## Relationship to existing models

### Existing `Tenant`

Current tenant/workspace record still contains:

- `billing_provider_connection_id`
- `lago_organization_id`
- `lago_billing_provider_code`

Those fields should become compatibility/derived fields during migration.

Long-term target:

- workspace reads its effective billing execution state from `workspace_billing_bindings`
- tenant-level Lago mapping fields stop being the primary source of truth

### Existing `BillingProviderConnection`

Remains platform-owned and continues to store:

- secret reference
- provider type
- display name
- environment
- sync status
- provider-level metadata

It should not be treated as the per-workspace execution boundary.

---

## Service boundaries

### New service: `WorkspaceBillingBindingService`

Responsibilities:

- create or update a workspace billing binding
- resolve or provision backend execution organization
- verify that the binding is usable
- expose binding state for workspace detail, customer setup, subscription setup, and payment flows
- surface provisioning/verification failures in Alpha-safe language

Suggested methods:

- `EnsureWorkspaceBillingBinding(...)`
- `GetWorkspaceBillingBinding(workspaceID)`
- `ListWorkspaceBillingBindings(...)`
- `VerifyWorkspaceBillingBinding(workspaceID)`
- `DisableWorkspaceBillingBinding(workspaceID)`

### Existing `BillingProviderConnectionService`

Should stay responsible for:

- provider credential lifecycle
- provider-level sync
- provider-level status

It should not own workspace-level execution context.

### Existing `TenantService`

Should stop resolving effective Lago mapping directly from the billing connection over time.

Instead it should:

- delegate to `WorkspaceBillingBindingService`
- treat raw tenant Lago fields as transitional/derived values only

---

## Backend adapter responsibilities

### Alpha side

Alpha adapter layer should support both:

1. `EnsureBillingProviderConnection`
- provider credential sync

2. `EnsureWorkspaceBillingExecutionContext`
- workspace-specific execution organization resolution or provisioning

That second path is what enables:

- shared mode today
- dedicated mode later

### Lago side

Depending on the isolation mode:

#### Shared mode
- resolve the configured default or existing shared backend organization
- bind the workspace to that org internally in Alpha

#### Dedicated mode
- ensure a dedicated Lago organization exists for the workspace
- record the resulting `backend_organization_id`

If Lago cannot support automatic organization provisioning through the current public API, Alpha should still keep the service seam and use a temporary internal provisioning path behind it.

---

## Product behavior

### Workspace setup

Workspace setup should eventually ask for:

- billing connection
- optional isolation mode if exposed to operators

It should not ask for:

- raw Lago organization id
- raw provider code

### Workspace detail

Should show:

- billing connection
- billing binding status
- isolation mode
- readiness
- last verified time
- next actions if provisioning failed

Advanced section may show:

- backend organization id
- backend provider code

### Billing connection detail

Should show:

- linked workspace count
- count by status if cheap to compute later

It should not imply that the connection itself is the workspace billing boundary.

---

## Migration plan

### Phase 1
- add `workspace_billing_bindings` table
- keep tenant billing fields unchanged
- new service reads/writes binding table

### Phase 2
- workspace setup writes binding records
- tenant detail and onboarding read effective binding state from the new table
- tenant `lago_*` fields are backfilled from binding for compatibility

### Phase 3
- customer/subscription/payment flows read binding-derived execution context
- direct dependence on tenant `lago_*` fields is reduced

### Phase 4
- tenant `lago_*` fields become derived-only or are removed from primary product APIs

---

## Compatibility rules

During migration, the effective workspace billing context should resolve in this order:

1. explicit `workspace_billing_binding`
2. existing `billing_provider_connection_id` + tenant `lago_*` compatibility mapping
3. hard failure with operator-safe remediation

That keeps current environments functional while the new model is adopted.

---

## API surface

Suggested internal platform APIs:

- `GET /internal/workspace-billing-bindings`
- `GET /internal/workspace-billing-bindings/{workspaceId}`
- `POST /internal/workspace-billing-bindings`
- `PATCH /internal/workspace-billing-bindings/{workspaceId}`
- `POST /internal/workspace-billing-bindings/{workspaceId}/verify`
- `POST /internal/workspace-billing-bindings/{workspaceId}/disable`

These should remain platform-facing initially.

---

## Testing requirements

### Backend

- service tests for shared-mode binding
- service tests for dedicated-mode binding
- migration/backfill tests from tenant `lago_*` fields
- verification failure tests
- tenant/workspace read-path tests using the binding as source of truth

### UI

- workspace setup selects billing connection without raw Lago org input
- workspace detail shows binding status
- failure state guidance for missing or broken binding

---

## Out of scope for this slice

- full customer-facing isolation mode controls
- multi-backend breadth beyond Lago
- automatic legal-entity or regional routing
- full compliance policy framework

---

## Exit criteria

This slice is complete when:

1. Alpha has a first-class `workspace billing binding` model.
2. Workspace billing execution context is no longer conceptually stored only on the billing connection.
3. Alpha can support both shared and future dedicated execution without changing the product-facing nouns.
4. Raw Lago organization ids are no longer part of the normal workspace setup flow.
