# Alpha Provider-Connect Plan

This document defines the concrete implementation plan for moving billing-provider setup from a Lago-first operational workflow to an Alpha-owned product workflow.

Use this plan when building the first real provider-connect flow.

## Goal

Reach a model where:
- Alpha is the only normal entrypoint for billing-provider connection setup
- Alpha owns provider connection state, readiness, and operator UX
- Lago remains the billing execution backend
- operators and tenants do not need to understand Lago organization ids or provider codes during normal product use

Initial scope:
- Stripe-first
- platform/operator driven
- no self-serve OAuth in the first slice

## Current Problem

Today the product is not truly doing `Connect Stripe`.

Current flow:
- Stripe provider is configured directly in Lago
- Alpha tenant stores only:
  - `lago_organization_id`
  - `lago_billing_provider_code`
- workspace setup asks users for those operational values

This is workable for staging and operator-led onboarding, but it is not production-grade product behavior.

## Product Decision

The first real provider-connect workflow should be:
- Alpha-owned connection records
- secure secret storage outside tenant rows
- Alpha-to-Lago provider synchronization through adapters
- workspace/tenant selection of an Alpha billing connection

Do not build a generic provider marketplace first.
Do Stripe first and keep the model extensible.

## Phase 1 Scope

Deliver these capabilities:
- create a Stripe billing connection in Alpha
- store Stripe secret material securely through a secret reference
- sync or ensure the corresponding provider in Lago
- expose connection readiness and sync status in Alpha
- allow workspaces to reference a billing connection by Alpha id
- stop exposing raw Lago mapping fields in the primary UI

Do not deliver in Phase 1:
- tenant self-serve Stripe OAuth
- multi-provider abstraction beyond what is needed for Stripe-first
- direct secret entry in tenant-scoped workflows
- deleting old tenant mapping fields immediately

## Target Product Model

### New Alpha-owned record

Add `billing_provider_connections`.

Suggested fields:
- `id UUID PRIMARY KEY`
- `provider_type TEXT NOT NULL`
- `environment TEXT NOT NULL`
- `display_name TEXT NOT NULL`
- `scope TEXT NOT NULL`
- `status TEXT NOT NULL`
- `owner_tenant_id TEXT NULL`
- `lago_organization_id TEXT NULL`
- `lago_provider_code TEXT NULL`
- `secret_ref TEXT NULL`
- `last_synced_at TIMESTAMPTZ NULL`
- `last_sync_error TEXT NULL`
- `connected_at TIMESTAMPTZ NULL`
- `disabled_at TIMESTAMPTZ NULL`
- `created_by_type TEXT NOT NULL`
- `created_by_id TEXT NULL`
- `created_at TIMESTAMPTZ NOT NULL`
- `updated_at TIMESTAMPTZ NOT NULL`

Recommended enum values:
- `provider_type`: `stripe`
- `environment`: `test`, `live`
- `scope`: `platform`
- `status`: `pending`, `connected`, `sync_error`, `disabled`
- `created_by_type`: `platform_api_key`, `ui_session`

### Workspace linkage

Add `billing_provider_connection_id` to `tenants`.

Keep existing fields for migration and derived sync state:
- `lago_organization_id`
- `lago_billing_provider_code`

In the long-term model:
- `billing_provider_connection_id` is the product-level linkage
- raw Lago fields become internal implementation detail and backfill compatibility only

## Secret Handling

Do not store Stripe secret keys on:
- `tenants`
- `billing_provider_connections`
- JSON metadata blobs

Store secrets in a dedicated external secret system.

Recommended first implementation:
- AWS Secrets Manager in staging/prod
- local file or env-backed test double for unit/integration tests

Add a small Alpha abstraction, for example:
- `BillingSecretStore`

Methods:
- `PutStripeSecret(ctx, connectionID, secret) (secretRef string, error)`
- `GetStripeSecret(ctx, secretRef) (string, error)`
- `RotateStripeSecret(ctx, secretRef, secret) (newSecretRef string, error)`
- `DeleteSecret(ctx, secretRef) error`

Important rule:
- Alpha DB stores only `secret_ref`
- raw provider secret leaves the request boundary only long enough to write into the secret store and sync to Lago

## Backend Service Boundary

Add a dedicated service:
- `BillingProviderConnectionService`

Responsibilities:
- create and update Alpha billing connection records
- validate provider config input
- store and rotate secret material via `BillingSecretStore`
- ensure the provider exists in Lago via adapter
- persist sync status and error state
- attach connections to workspaces
- expose connection readiness for UI and onboarding

Do not fold this into `TenantService`.
Tenant service should consume the provider connection service, not own it.

## Lago Adapter Boundary

Add a dedicated adapter interface in Alpha:
- `BillingProviderAdapter`

Stripe-first methods:
- `EnsureStripeProvider(ctx, input) (ProviderSyncResult, error)`
- `GetProvider(ctx, organizationID, providerCode) (ProviderState, error)`
- `DisableProvider(ctx, organizationID, providerCode) error`

Suggested input shape:
- `organization_id`
- `provider_code`
- `display_name`
- `secret_key`
- `success_redirect_url`
- `webhook_url`
- provider capability flags as needed

Result should include:
- effective Lago organization id
- effective Lago provider code
- sync timestamp
- any normalized provider metadata worth projecting into Alpha

This keeps Lago-specific API calls out of product services.

## API Surface

Phase 1 should add platform-scoped APIs only.

Suggested endpoints:
- `POST /internal/billing-provider-connections`
- `GET /internal/billing-provider-connections`
- `GET /internal/billing-provider-connections/{id}`
- `PATCH /internal/billing-provider-connections/{id}`
- `POST /internal/billing-provider-connections/{id}/sync`
- `POST /internal/billing-provider-connections/{id}/rotate-secret`
- `POST /internal/billing-provider-connections/{id}/disable`

Workspace endpoints should evolve to accept:
- `billing_provider_connection_id`

Do not require normal callers to submit:
- `lago_organization_id`
- `lago_billing_provider_code`

## UI Surface

Add a platform-only area:
- `Billing Connections`
- `New Billing Connection`
- `Billing Connection Detail`

Suggested routes:
- `/billing-connections`
- `/billing-connections/new`
- `/billing-connections/[id]`

### Billing Connections list
Should show:
- display name
- provider type
- environment
- status
- linked workspaces count
- last sync time

### New Billing Connection
Should do one job:
- create a Stripe connection

Inputs:
- connection name
- environment
- Stripe secret key
- optional provider code override behind advanced section

Do not ask for raw Lago ids in the main form.

### Billing Connection Detail
Should show:
- connection status
- sync history summary
- last sync error
- linked workspaces
- rotate secret action
- resync action
- disable action
- advanced details with Lago provider code only if needed

### Workspace Setup change
Workspace setup should switch from:
- entering raw mapping values

to:
- selecting an existing billing connection
- or linking out to create one first

The main setup form should not pretend to be a Stripe-connect flow if it is still only mapping ids.

## Data Migration Plan

### Migration 1
Add:
- `billing_provider_connections`
- `tenants.billing_provider_connection_id`
- indexes and status constraints

### Migration 2
Backfill provider connections from existing tenant mappings where feasible.

Backfill rule:
- if multiple tenants share the same `(lago_organization_id, lago_billing_provider_code)` pair, create one platform-scoped provider connection and link all matching tenants to it
- if a tenant has incomplete mapping, leave `billing_provider_connection_id` null and keep it flagged for manual repair

### Migration 3
Update APIs and UI to prefer `billing_provider_connection_id`.

### Migration 4
Keep old tenant mapping fields readable internally, but remove them from primary setup UX.

Do not drop old fields immediately.
Use at least one release cycle of dual-read/dual-write if needed.

## Readiness Model Changes

Add a new readiness dimension for provider connections.

Connection readiness should cover:
- credentials stored
- synced to Lago
- provider reachable enough for Alpha workflow needs
- webhook/redirect prerequisites configured
- no outstanding sync error

Tenant/workspace readiness should depend on:
- workspace exists
- billing provider connection linked
- linked provider connection is healthy
- pricing exists
- first customer readiness as already modeled

This gives a more honest readiness model than today's raw mapping checks.

## Testing Strategy

### Unit tests
Add service tests for:
- secret write success/failure
- Lago sync success/failure
- status transitions
- workspace attach/detach behavior
- rotate secret behavior

### Integration tests
Add API coverage for:
- create connection
- list/detail connection
- sync failure -> `sync_error`
- rotate secret -> resync -> recovered
- create workspace using `billing_provider_connection_id`

### UI tests
Add Playwright/session coverage for:
- billing connections list
- new connection flow
- detail screen with sync error state
- workspace setup using connection selection instead of raw mapping entry

### Live/staging verification
Add a repeatable staging command that:
- mints a platform key
- creates or syncs a test Stripe connection in Alpha
- creates a workspace linked to that connection
- verifies payment/customer flows still work

## Infra Work

Needed infrastructure pieces:
- secret storage path for Stripe connection secrets
- IAM permissions for Alpha runtime to read/write those secrets
- staging bootstrap path for test Stripe connection sync
- optional one-shot admin job for seeding or rotating provider connections in-cluster

Do not rely on long-term manual Lago console setup.
That would preserve the wrong operational boundary.

## Rollout Order

Recommended execution order:
1. add schema and domain types
2. add `BillingSecretStore`
3. add `BillingProviderAdapter`
4. add `BillingProviderConnectionService`
5. add platform APIs
6. add Billing Connections UI
7. update Workspace Setup to select connections
8. backfill existing staging data
9. verify customer/payment flows against the new linkage
10. only then plan self-serve provider onboarding

## What To Avoid

Avoid these mistakes:
- storing Stripe secrets directly in Alpha tables
- making `TenantService` the owner of provider lifecycle
- exposing Lago ids in the primary product workflow
- building generic provider abstractions before Stripe-first works end to end
- pretending the product has self-serve connect when it still requires ops-only Lago setup

## Near-Term Repository Changes

The first repo slice should include:
- new migration(s) for provider connections and tenant linkage
- new domain/store types
- `BillingSecretStore` abstraction with test implementation
- `BillingProviderConnectionService`
- `LagoBillingProviderAdapter`
- internal API handlers and tests
- Billing Connections UI routes and screens
- Workspace Setup form update to remove raw Lago mapping from the primary path

## Bottom Line

If we build provider-connect now, the correct production-grade move is:
- Alpha owns provider connection state and UX
- secrets live in a real secret store
- Lago remains an adapter-backed execution dependency
- workspaces choose Alpha connections, not raw Lago mappings

That gives the product the right long-term boundary without turning Alpha into a second billing engine.
