# Alpha Implementation Roadmap

This document translates the Alpha/Lago boundary into a practical delivery plan.

Use it when deciding what to build next without drifting into a second billing engine inside Alpha.

## Goal

Reach a long-term production model where:
- Alpha is the only normal entrypoint for operators and tenants
- Lago stays behind Alpha as the billing execution engine
- onboarding and payment operations are smooth in Alpha
- Alpha does not become a duplicate invoice or payment engine

## Guardrails

Keep these rules true in every phase:
- Alpha owns onboarding, readiness, audit, orchestration, and product workflows
- Lago owns billing execution
- duplication is allowed only for projections, mappings, and readiness state
- no normal operator workflow should require Lago-specific manual steps long-term

## Phase 0: Current Foundation

This is already in place.

Delivered foundations:
- platform and tenant auth split
- platform bootstrap path
- canonical tenant model
- tenant lifecycle audit
- internal tenant operator APIs
- tenant onboarding orchestration with layered readiness
- Lago webhook routing through tenant-backed mapping

What this phase gives us:
- a stable control-plane base
- a credible operator-led onboarding model
- the correct long-term auth and tenancy shape

## Phase 1: Stabilize The Alpha-Lago Adapter Boundary

Goal:
- remove scattered Lago coupling from unrelated services

Build:
- a clear internal adapter boundary for Lago calls
- thin adapter interfaces for:
  - tenant/billing organization setup
  - customer sync
  - invoice fetch/projection
  - payment operation calls
- service-level ownership rules so product services call adapters, not Lago directly

Do not do:
- user-visible API redesign just for adapter cleanup
- a large migration of every Lago call in one shot

Exit criteria:
- new Lago-dependent work goes through adapters
- ownership of Lago interaction becomes explicit in code

## Phase 2: Add First-Class Alpha Customer Models

Goal:
- make customer onboarding and readiness visible in Alpha

Build:
- `customers`
- `customer_billing_profiles`
- `customer_payment_setup`

These should hold product and orchestration state such as:
- tenant ownership
- customer display identity
- billing profile completeness
- payment readiness state
- mappings to Lago-side records
- operator-facing status and error fields

Do not do:
- full invoice engine state in Alpha
- full payment execution truth in Alpha
- provider-vault behavior in Alpha

Exit criteria:
- Alpha can represent a customer as a first-class product object
- Alpha can report whether that customer is billing-ready

## Phase 3: Extend Onboarding To First-Customer Readiness

Goal:
- move from tenant-ready to tenant-can-bill-a-customer

Build:
- first-customer readiness inside onboarding
- explicit readiness layers for:
  - tenant
  - billing integration
  - first customer
  - payment readiness
- operator-visible failure reasons and retry points

Checks should cover:
- customer exists in Alpha
- billing profile is complete
- customer is synced to Lago through adapters
- payment setup state is known
- first billing path is operationally understandable from Alpha

Do not do:
- a giant onboarding endpoint that hides partial states
- hard coupling of onboarding to one provider-specific happy path

Exit criteria:
- operators can see exactly why onboarding is blocked
- first-customer setup is no longer tribal knowledge

## Phase 4: Replace Lago-First Setup With Alpha-First Setup

Goal:
- make Alpha the operational entrypoint even for billing setup

Build:
- Alpha-native operator APIs for billing integration setup
- Alpha-driven creation or synchronization of tenant billing org state through adapters
- Alpha-driven customer billing setup flows

Long-term target:
- operators should not need the Lago console or ad hoc Lago scripts for normal onboarding
- Lago should remain available only for internal debugging or exceptional intervention

Do not do:
- expose raw Lago identifiers in primary operator workflows unless strictly necessary
- keep env-driven mapping as a long-term operational dependency

Exit criteria:
- normal onboarding can be completed from Alpha
- Lago setup becomes an implementation detail behind Alpha services

## Phase 5: Smooth Operator Onboarding

Goal:
- make first-customer onboarding reproducible and low-friction

Build:
- one operator onboarding flow in Alpha
- strong readiness reporting
- audit for onboarding transitions
- optional seeded pricing/customer templates where useful

This phase should cover:
- tenant creation
- billing integration setup
- first tenant admin bootstrap
- first customer creation
- billing profile completion
- payment readiness verification
- handoff evidence

Do not do:
- overbuild a UI before the backend orchestration and readiness model are stable

Exit criteria:
- operator-assisted onboarding is smooth and evidence-backed
- the onboarding runbook becomes shorter because Alpha absorbs the manual steps

## Phase 6: Self-Signup Or Invite-Based Onboarding

Goal:
- let tenants start onboarding without operator-led creation for every case

Build on top of existing Alpha onboarding services.
Do not build a second onboarding implementation.

Possible entrypoints:
- self-signup
- invite-based admin setup
- sales-assisted onboarding UI

These flows should reuse:
- tenant creation logic
- customer setup logic
- readiness checks
- billing adapter orchestration

Exit criteria:
- self-serve flows and operator flows share the same backend orchestration
- Alpha remains the only product-facing entrypoint

## What To Avoid Throughout

Avoid these traps:
- Alpha and Lago co-owning invoice or payment execution truth
- customers or operators needing Lago knowledge for normal work
- new features that bypass readiness and audit
- one-off scripts becoming permanent product dependencies
- building a polished UI over an unstable backend model

## Recommended Near-Term Next Steps

If we continue from the current state, the next sensible order is:
1. stabilize Lago adapter boundaries in code
2. design and add minimal Alpha customer models
3. extend onboarding readiness to first-customer and payment readiness
4. move billing setup flows behind Alpha APIs
5. then build smoother operator onboarding UX

Use [alpha-lago-adapter-plan.md](./alpha-lago-adapter-plan.md) as the concrete execution checklist for Phase 1.
Use [alpha-customer-model.md](./alpha-customer-model.md) as the concrete design for Phase 2 customer records and readiness.

## Bottom Line

The roadmap is intentionally staged.

First make Alpha the clear control plane.
Then make customer and payment onboarding visible in Alpha.
Then hide Lago operationally.
Then build self-serve entrypoints on top of the same orchestration.

That sequence is production-grade without overengineering.
