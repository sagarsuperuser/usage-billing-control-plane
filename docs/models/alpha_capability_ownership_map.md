# Alpha Capability Ownership Map

This document translates the Alpha/Lago boundary into a practical product ownership map.

Use it when deciding:
- what Alpha already owns today
- what still leaks Lago into the product or operations
- what to build next so Alpha becomes the only normal entrypoint

Read this together with [Alpha and Lago System Boundary](./alpha-lago-boundary.md).

## Product Goal

Alpha should be the only normal product surface for operators and end customers.

That means:
- no end user should need Lago UI
- no normal operator workflow should require Lago UI
- Lago should remain an internal execution backend until or unless Alpha replaces it

## Ownership Rule

For every billing capability, decide three things explicitly:
- who owns the product workflow
- who owns the execution truth
- whether Lago still leaks into normal operations

If Alpha owns the workflow but users still need Lago, the product boundary is incomplete.
If Alpha and Lago both try to own execution truth, the architecture is confused.

## Current Capability Map

| Capability | Alpha owns today | Lago still owns today | Current state | Lago leak today | Next build direction |
| --- | --- | --- | --- | --- | --- |
| Platform auth and admin bootstrap | platform users, platform keys, bootstrap, operator APIs | none | strong | no | keep Alpha-owned |
| Workspace access and invites | invite issue, acceptance, membership state, RBAC UX | none | strong | no | keep Alpha-owned |
| Tenant and workspace onboarding | onboarding workflow, readiness, workspace binding | none | strong | small | remove remaining manual repair paths |
| Billing connections | connection records, health/status surfaces, workspace binding | provider execution details synced behind the scenes | partial | some operator repair still depends on backend knowledge | complete Alpha-owned lifecycle and verification |
| Customer onboarding | customer create, billing profile, readiness, payment-setup status | billing-engine customer copy | strong | low | keep Alpha authoritative for workflow |
| Payment setup | setup request UX, readiness, refresh, retry handoff | checkout/payment-method execution backend | strong | low | keep Alpha-owned workflow, reduce backend-specific assumptions |
| Pricing authoring | metrics, rating rules, plans, product-side pricing model | executable billing-engine copy | strong | low | keep Alpha as pricing surface |
| Subscription orchestration | create/change/cancel workflow and UI | billing-engine subscription execution | strong | low | keep Alpha-owned workflow |
| Usage ingestion | Alpha API, persistence, replay-safe event model | billing-engine usage copy for rating/invoicing | partial | low | strengthen Alpha-native metering over time |
| Current billable-state proof | current-usage journey, customer/subscription linkage | current billed amount calculation | strong enough for staging | no user-facing leak | add issued-invoice proof next |
| Invoice visibility | invoice list/detail, Alpha-normalized invoice responses | invoice execution truth, document generation | partial but real | some backend execution dependence is acceptable | keep Alpha UI/API, continue normalization |
| Invoice explainability | explainability API/UI and digest | source invoice/fee payload | strong | low | keep Alpha-owned product feature |
| Payment recovery | lifecycle classification, retry/collect guidance, retry UI/API | actual collection attempt execution | partial | no user-facing leak, but no timed policy yet | build Alpha-owned dunning |
| Dunning | only lifecycle hints and manual actions | retry and overdue primitives exist in backend | weak | capability gap, not just leak | add Alpha policy, scheduler, notifications, operator controls |
| Replay and reconciliation | replay jobs, diagnostics, recovery UI | none | strong | no | keep Alpha-owned |
| Notifications for billing workflows | resend actions and notification boundary exist in Alpha | selected document/collections delivery execution | partial | delivery execution still delegated | keep Alpha-owned policy and routing |
| Webhook ingestion | HMAC verification, ingestion, tenant mapping, projections | webhook emission | strong | low | keep Alpha-owned ingestion boundary |
| Payment and invoice execution truth | projections only | canonical invoice/payment execution state | intentionally Lago-owned | acceptable internal dependency | keep hidden behind Alpha until replacement is intentional |

## What Alpha Already Owns Well

These areas are already product-grade or close enough that normal users should remain inside Alpha:

- platform auth and bootstrap
- workspace access and invites
- tenant and workspace onboarding
- customer onboarding and readiness
- payment setup request and refresh workflow
- pricing authoring
- subscription creation, change, and cancellation workflow
- payment visibility and operator guidance
- invoice explainability
- replay and reconciliation
- webhook ingestion and payment projections
- browser/operator journeys for the main tenant workflows

This is why Alpha is already a credible control plane.
It is not only a thin Lago skin anymore.

## What Still Leaks Lago Today

These are the main places where Lago still leaks into implementation or operations more than the target product boundary allows.

### 1. Dunning is not Alpha-owned yet

Today Alpha can say:
- `retry_payment`
- `collect_payment`
- `investigate`

But Alpha does not yet own:
- retry schedule
- reminder cadence
- escalation policy
- attempt history as a first-class workflow
- pause/resume/manual override of dunning state

This is the biggest capability gap.

### 2. Billing connection lifecycle still has backend-shaped seams

Alpha owns the connection records and much of the workflow, but the full lifecycle is not fully closed inside Alpha yet.

Remaining leak types:
- secret rotation and sync assumptions
- backend/provider verification repair paths
- provider-specific health and recovery knowledge

### 3. Invoice execution is still backend-shaped even though Alpha surfaces it well

Alpha already owns invoice browsing and explainability, but canonical invoice execution still lives in Lago.
That is acceptable for now, but it means:
- Alpha must keep a strong invoice adapter boundary
- Alpha must keep normalizing backend payloads into Alpha-native responses
- Alpha should keep adding operator workflows without exposing backend language

### 4. Payment setup and collection still rely on backend/provider internals behind the scenes

The user does not need Lago UI, which is good.
But the implementation still depends on:
- backend-side payment-method materialization
- backend payment execution semantics
- provider-specific collection behavior

That is acceptable short term, but it is where future Alpha-native backend decoupling will start.

### 5. Journey tooling still uses backend credentials in staging

The product boundary for real users is improving faster than the operational testing boundary.
Examples:
- staging journey scripts requiring `LAGO_API_KEY`
- deterministic fixture setup performed directly through backend APIs

That is fine for engineering validation, but it is not the desired long-term product ownership line.

## What To Build Next

If the product goal is that users never deal with Lago, the next build order should be:

### 1. Alpha-owned dunning and recovery policy

Build first-class Alpha dunning with:
- policy model
- per-invoice dunning state
- scheduled execution
- notification cadence
- operator controls

This closes the largest product capability gap.

### 2. Usage-to-issued-invoice journey

After current-usage proof, make Alpha prove:
- usage becomes an issued invoice
- the invoice is visible through Alpha
- explainability works on that exact invoice
- downstream payment flows start from that Alpha-visible invoice state

This strengthens invoice ownership at the product layer without changing execution truth yet.

### 3. Billing connection lifecycle journey

Complete the product boundary so operators can:
- create connection
- verify connection
- rotate or repair connection
- bind workspace billing
- confirm webhook health

All without backend/manual repair knowledge.

### 4. Alpha-owned notification and reminder policy for billing workflows

Keep delivery adapters if needed, but Alpha should own:
- when to send
- why to send
- to whom to send
- resend/escalation policy
- audit trail

### 5. Reduce backend-shaped staging fixtures over time

Long term, real journey setup should rely more on Alpha-owned test/bootstrap primitives and less on direct backend API mutation.
That does not need to happen before the product workflows are complete, but it should remain the direction.

## What Should Stay Backend-Owned For Now

Do not force Alpha to own these immediately:

- canonical invoice execution truth
- canonical payment execution truth
- provider-specific collections internals
- tax/compliance-heavy invoice mechanics
- the entire billing engine

Those can remain behind adapters until there is a deliberate backend replacement program.

## Good Long-Term Product Boundary

The clean long-term model is:

- Alpha owns workflow, policy, UI, notifications, readiness, audit, and operator controls
- Lago remains an internal billing execution backend for as long as it is useful
- users and normal operators stay entirely inside Alpha

This lets Alpha become the real product without prematurely turning Alpha into a second billing engine.

## Capability Decision Checklist

When adding or reviewing a billing capability, ask:

1. Can a user or operator complete this entirely in Alpha?
2. Does Alpha own the workflow and policy?
3. Is Lago only an execution backend here?
4. Are we exposing backend language that users should not need?
5. Are we duplicating execution truth instead of projecting it?

If the answer to 1 or 2 is no, the Alpha product boundary is still incomplete.
If the answer to 5 is yes, the architecture is moving in the wrong direction.
