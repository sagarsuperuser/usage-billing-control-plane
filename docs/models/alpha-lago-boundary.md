# Alpha and Lago System Boundary

This document defines the intended long-term production architecture.

Use this as the decision guide for future onboarding, customer, billing, and payment work.

For a practical current-state ownership map, read [Alpha Capability Ownership Map](./alpha_capability_ownership_map.md).

## One-Line Model

- `Alpha` is the product surface, control plane, and orchestration layer.
- `Lago` is the billing execution engine behind the scenes.

People and automation should enter through Alpha.
Normal operator or tenant workflows should not require direct Lago setup or direct Lago API usage.

## Why This Split Exists

Alpha adds value in places Lago should not own:
- platform auth and tenant auth
- tenant lifecycle and onboarding
- control-plane UI and operator workflows
- replay, reconciliation, and explainability
- readiness checks and audit
- product-specific orchestration

Lago adds value in places Alpha should not reimplement:
- invoice generation mechanics
- payment execution mechanics
- billing-engine workflows
- provider-side billing execution details
- billing events used to project payment state back into Alpha

## Ownership Rules

For each domain, there must be one canonical owner.

### Alpha owns
- platform identities and operator access
- tenant identities and tenant lifecycle
- onboarding state and readiness state
- pricing intent and product-side configuration
- customer-facing and operator-facing workflows
- replay, reconciliation, and explainability workflows
- audit trails
- local projections needed for product UX and operations

### Lago owns
- billing execution
- invoice execution state
- payment execution state
- billing-engine internals
- provider-facing billing execution details

### Duplication That Is Allowed
- projections for fast reads in Alpha
- external mappings between Alpha and Lago records
- readiness and workflow state in Alpha
- execution copies of Alpha-authored config synced into Lago

### Duplication That Is Not Allowed
- two canonical sources of truth for invoice or payment execution state
- Alpha becoming a second billing engine
- operator workflows that require humans to understand Lago internals for normal product use

## Entry Points

### Alpha entry points
These are the long-term human and product entry points:
- platform/operator APIs under `/internal/...`
- tenant-facing product APIs under `/v1/...`
- browser UI backed by Alpha session and API flows
- later, self-signup or invite-based onboarding flows

### Lago entry points
These should be internal only:
- adapter calls from Alpha services
- webhook delivery into Alpha
- emergency/debug usage by the platform team only

## Service Boundary Inside Alpha

Keep Lago access behind adapters.
Do not let unrelated services call Lago directly.

Recommended internal boundary:
- `BillingAdapter`
- `LagoTenantAdapter`
- `LagoCustomerAdapter`
- `LagoPaymentAdapter`
- `LagoInvoiceAdapter`

This keeps the product model stable even if the billing backend evolves later.

## Onboarding Model

Onboarding should be layered.
Do not build one giant onboarding blob.

### Layer 1: Platform bootstrap
Goal:
- create the first `platform_admin`

Current shape:
- root bootstrap CLI for the first platform key
- after that, operator APIs are the normal path

### Layer 2: Tenant onboarding
Goal:
- create a tenant and make it structurally ready

Expected outcomes:
- tenant exists
- tenant is active
- billing integration mapping exists
- first tenant admin exists
- minimum pricing exists

### Layer 3: First-customer onboarding
Goal:
- make the tenant able to bill its first real customer through Alpha

Expected outcomes:
- customer exists in Alpha
- billing profile is complete enough for billing
- customer is synced to Lago through adapters
- customer readiness is visible in Alpha

### Layer 4: Payment readiness
Goal:
- make the first-customer billing path operational

Expected outcomes:
- payment setup is complete enough for the target flow
- Alpha can show readiness and failures clearly
- payment operations stay in Alpha while execution remains in Lago

## Data Model Guidance

Alpha should have first-class records for product and orchestration state.

Recommended Alpha-owned models:
- `tenants`
- `tenant_audit_events`
- `platform_api_keys`
- tenant API keys and API key audit
- `tenant_onboarding`
- `customers`
- `customer_billing_profiles`
- `customer_payment_setup`
- pricing configuration and templates
- operational projections for payment status and webhook processing

Important rule:
These Alpha records exist to support product flows, readiness, audit, and operator UX.
They should not silently become a second billing engine.

## Practical Rules For New Features

When adding a new billing-related model or API, ask these questions.

### Build it in Alpha when
- Alpha needs it for onboarding
- Alpha needs it for readiness
- Alpha needs it for audit or operator UX
- Alpha needs it for replay, reconciliation, or explainability
- Alpha needs a stable product model that should not expose Lago internals

### Do not build it as canonical Alpha truth when
- Lago already owns the execution state
- the model would duplicate invoice or payment execution truth
- the model would force Alpha to behave like a second billing engine

## Customer and Payment Caution

This is the main area where bad duplication can happen.

Good Alpha-side customer/payment data:
- onboarding state
- billing profile completeness
- payment setup readiness
- mappings to Lago-side records
- operator-visible status projections

Bad Alpha-side customer/payment ownership:
- full billing execution truth
- full payment execution truth
- invoice engine truth duplicated from Lago

If Alpha starts owning those, the architecture becomes confused and expensive.

## Self-Signup Later

This architecture is compatible with future self-signup.

The right sequence is:
1. operator-assisted onboarding through Alpha
2. stable onboarding orchestration inside Alpha
3. self-signup or invite flows added on top of the same services

Self-signup should reuse Alpha onboarding services.
It should not introduce a separate onboarding implementation.

## Recommended Delivery Order

1. keep the current platform and tenant auth split
2. stabilize the Lago adapter boundary inside Alpha
3. keep extending layered tenant onboarding
4. add first-class Alpha customer and billing-profile models
5. add first-customer and payment readiness in Alpha
6. keep Lago hidden behind adapters for execution
7. build self-signup later on top of the same onboarding services

## Bottom Line

The long-term production target is:
- `Alpha` as the only product and operator entrypoint
- `Lago` as the hidden billing execution engine

If a future change makes Alpha easier to use without making it a second billing engine, it is probably the right direction.
If a future change makes Alpha and Lago co-own the same billing execution truth, it is probably the wrong direction.
