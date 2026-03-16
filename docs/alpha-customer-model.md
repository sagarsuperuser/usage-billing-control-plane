# Alpha Customer Model

This document defines the minimal Phase 2 customer model for Alpha.

It is designed to be production-usable without turning Alpha into a second billing engine.

## Goal

Add first-class customer records to Alpha so that Alpha can:
- represent customers as product objects
- track billing-profile completeness
- track payment setup readiness
- support onboarding and support workflows
- prepare for first-customer readiness checks

It must not:
- duplicate invoice execution truth from Lago
- duplicate payment execution truth from Lago
- force a rewrite of existing usage, replay, and reconciliation paths

## Current Constraint

Alpha already uses tenant-scoped `customer_id` strings across core flows:
- usage events
- billed entries
- replay jobs
- reconciliation filters and outputs

Lago-facing webhook and payment projections already carry:
- `customer_external_id`

So the correct Phase 2 move is not to invent a new public customer identifier.
The correct move is to make the existing tenant-scoped customer identifier a first-class Alpha concept.

## Key Decision

Treat the current tenant-scoped `customer_id` as the stable Alpha customer key.

In practice:
- `usage_events.customer_id`
- `billed_entries.customer_id`
- `replay_jobs.customer_id`
- reconciliation `customer_id`

all continue to mean:
- the Alpha customer external/business key within a tenant

This keeps current flows stable.

## Minimal Canonical Models

Phase 2 should add these Alpha-owned models.

### 1. customers
Purpose:
- first-class product/customer object in Alpha

Recommended fields:
- `id`
- `tenant_id`
- `external_id`
- `display_name`
- `email`
- `status`
- `lago_customer_id`
- `created_at`
- `updated_at`

Rules:
- `external_id` is the existing tenant-scoped customer key used by current runtime flows
- `id` is the internal Alpha row id
- unique constraint on `(tenant_id, external_id)`

Why both `id` and `external_id`:
- `id` gives Alpha a stable internal record handle
- `external_id` preserves compatibility with current usage/replay/reconciliation flows

### 2. customer_billing_profiles
Purpose:
- track billing-profile completeness in Alpha

Recommended fields:
- `customer_id`
- `tenant_id`
- `legal_name`
- `email`
- `phone`
- `billing_address_line1`
- `billing_address_line2`
- `billing_city`
- `billing_state`
- `billing_postal_code`
- `billing_country`
- `currency`
- `tax_identifier`
- `provider_code`
- `profile_status`
- `last_synced_at`
- `last_sync_error`
- `created_at`
- `updated_at`

This is not invoice truth.
It is onboarding and billing-readiness state.

### 3. customer_payment_setup
Purpose:
- track payment readiness and operator-visible payment setup state

Recommended fields:
- `customer_id`
- `tenant_id`
- `setup_status`
- `default_payment_method_present`
- `payment_method_type`
- `provider_customer_reference`
- `provider_payment_method_reference`
- `last_verified_at`
- `last_verification_result`
- `last_verification_error`
- `created_at`
- `updated_at`

This is not full provider vault truth.
It is Alpha-side readiness and support state.

## What Not To Add Yet

Do not add these in Phase 2:
- subscriptions table as billing-engine truth
- invoice table as canonical execution truth
- payment attempts ledger as canonical execution truth
- full provider payment-method objects in Alpha
- deep tax engine state

Those would push Alpha into billing-engine duplication.

## Identifier Strategy

Use this strategy consistently.

### External/business key
- `external_id`
- tenant-scoped
- corresponds to the current `customer_id` used in usage and replay flows

### Internal Alpha key
- `customers.id`
- used for Alpha-owned relational records such as billing profile and payment setup

### Lago mapping
- `lago_customer_id`
- stored on the Alpha customer record
- adapter-owned integration mapping

This gives Alpha:
- stable internal joins
- compatibility with current tenant usage flows
- a clean place to store Lago linkage

## Runtime Compatibility Plan

Do not rewrite existing usage/replay/reconciliation APIs in Phase 2.

Instead:
- keep `customer_id` in current runtime APIs
- interpret it as `customers.external_id`
- add customer creation and lookup APIs that manage the canonical customer record

This minimizes churn and avoids breaking existing tenant flows.

## Minimal Service Layer

Phase 2 should add these services.

### CustomerService
Responsibilities:
- create customer
- get customer by tenant and external id
- list customers
- update basic customer metadata
- attach or update Lago mapping

### CustomerBillingProfileService
Responsibilities:
- upsert billing profile
- compute billing profile completeness
- track sync status and errors

### CustomerPaymentSetupService
Responsibilities:
- upsert payment setup readiness state
- expose payment readiness summary
- track verification status and last errors

These are Alpha workflow services, not billing-engine services.

## Minimal API Surface

Do not overbuild this first slice.

Recommended first endpoints:
- `POST /v1/customers`
- `GET /v1/customers`
- `GET /v1/customers/{external_id}`
- `PATCH /v1/customers/{external_id}`
- `PUT /v1/customers/{external_id}/billing-profile`
- `GET /v1/customers/{external_id}/billing-profile`
- `PUT /v1/customers/{external_id}/payment-setup`
- `GET /v1/customers/{external_id}/payment-setup`
- `GET /v1/customers/{external_id}/readiness`

Keep the path identifier as `external_id` initially.
That matches current Alpha usage semantics and avoids leaking an internal row id too early.

## Readiness Model

Phase 2 should enable real customer readiness.

Recommended readiness sections:
- `customer_exists`
- `billing_profile_ready`
- `payment_setup_ready`
- `lago_mapping_ready`
- `status`
- `missing_steps`

This should feed Phase 3 onboarding work.

## Initial Status Enums

Keep status sets small.

### customers.status
- `active`
- `suspended`
- `archived`

### customer_billing_profiles.profile_status
- `missing`
- `incomplete`
- `ready`
- `sync_error`

### customer_payment_setup.setup_status
- `missing`
- `pending`
- `ready`
- `error`

These are enough for operational clarity without overdesign.

## How This Fits With Lago

Alpha owns:
- customer identity in the Alpha product
- billing-profile completeness
- payment readiness state
- support-facing and onboarding-facing status

Lago owns:
- billing execution behavior
- provider-side execution state
- invoice and payment execution results

The mapping point is:
- `customers.lago_customer_id`
- profile/provider metadata used by adapters

## Migration Strategy

Keep migration scope small.

### Phase 2A
Add tables only:
- `customers`
- `customer_billing_profiles`
- `customer_payment_setup`

No backfill required yet beyond optional inferred customer creation later.

### Phase 2B
Add services and CRUD APIs.

### Phase 2C
Add customer readiness endpoint and use it in onboarding.

Do not backfill every historic `customer_id` into `customers` immediately unless a real workflow needs it.
That is easy to overdo.

## Practical Near-Term Rule

Until Phase 2 is implemented:
- `customer_id` in current APIs remains a plain tenant-scoped business key

After Phase 2 is implemented:
- `customer_id` in current APIs should map to `customers.external_id`
- new customer APIs should make that relationship explicit

## Bottom Line

The minimal production-grade Phase 2 design is:
- keep current tenant-scoped `customer_id` semantics
- add canonical Alpha customer records around that key
- add billing-profile and payment-setup state in Alpha
- avoid duplicating billing execution truth from Lago

That gives Alpha real customer onboarding and readiness primitives without destabilizing existing flows.
