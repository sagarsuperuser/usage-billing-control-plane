# Alpha Notification Architecture

This document defines the long-term production-grade notification model for Alpha.

Read together with:

- [Alpha Import Goal](./alpha_import_goal.md)
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)
- [Alpha Workspace Access Spec](./alpha_workspace_access_spec.md)
- [Alpha API Credentials Spec](./alpha_api_credentials_spec.md)
- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](./alpha_slice3_subscriptions_spec.md)
- [Slice 4 Spec: Invoices Visibility](./alpha_slice4_invoices_spec.md)

---

## Objective

Alpha should own notification policy and product-level event routing.

That does **not** mean Alpha must execute every outbound email itself.

The correct long-term model is:

- Alpha owns notification intent, product copy, and routing rules
- Alpha uses its own delivery path for auth, workspace, security, and product notifications
- Alpha uses Lago selectively for billing-document and collections delivery where Lago is already the correct billing engine

This keeps product ownership clear without duplicating every billing mail capability in Alpha.

---

## Direct Decision

### Use Alpha delivery for

- workspace invitations
- password reset
- browser-auth account lifecycle
- workspace membership and access notifications
- service-account and security notifications
- platform/workspace onboarding and handoff notifications
- Alpha product lifecycle notifications that are not billing-document delivery

### Use Lago delivery for

- invoice finalized emails
- invoice resend flows
- payment receipt emails
- credit note emails
- dunning / overdue payment collection emails
- billing-entity scoped finance-document delivery settings

### Do not do this

- do not make Lago the general notification engine for Alpha
- do not route workspace/auth/security notifications through Lago
- do not make Alpha product flows depend on billing-entity mail semantics

---

## Why This Split Is Correct

### Lago is already strong at

- finance-document delivery
- billing-entity scoped email settings
- invoice / payment receipt / credit note delivery
- collections and dunning scenarios

### Alpha is now responsible for

- browser auth
- workspace invitations
- workspace membership
- workspace access handoff
- service accounts and API credentials
- platform vs tenant role-aware product workflows

Those are different notification domains.

If Alpha forces all notification traffic through Lago:

- product ownership becomes blurry
- auth and workspace flows become coupled to billing-engine concepts
- template and copy control splits between products
- observability and audit become harder to reason about
- future non-billing product notifications become awkward to implement

---

## Notification Domains

### 1. Auth and identity notifications

Owner:
- Alpha

Examples:
- password reset
- invite acceptance reminders
- browser account verification later if needed
- sign-in or recovery notifications later if needed

Execution path:
- Alpha mailer / Alpha delivery provider

### 2. Workspace and access notifications

Owner:
- Alpha

Examples:
- workspace invitation
- membership granted or changed
- access revoked
- initial workspace handoff
- service-account lifecycle alerts

Execution path:
- Alpha mailer / Alpha delivery provider

### 3. Product workflow notifications

Owner:
- Alpha

Examples:
- workspace billing connected
- setup requires attention
- customer readiness blocked
- payment setup requested or completed
- internal operator notifications later

Execution path:
- Alpha mailer / Alpha delivery provider

### 4. Billing-document notifications

Owner:
- Alpha product policy

Execution backend:
- Lago

Examples:
- invoice finalized email
- resend invoice email
- payment receipt email
- credit note email

Execution path:
- Alpha notification service routes to Lago-backed document delivery

### 5. Collections and dunning notifications

Owner:
- Alpha product policy

Execution backend:
- Lago initially

Examples:
- overdue payment reminder
- dunning sequence delivery
- payer collection follow-up tied to billing entity

Execution path:
- Alpha notification service routes to Lago-backed collections delivery

---

## Long-Term Architecture

### Alpha-owned boundary

Add or evolve a first-class Alpha service boundary such as:

- `NotificationService`

Responsibilities:

- accept product notification intents
- classify notification domain
- choose execution backend
- attach request/workspace/customer context
- enforce policy and feature flags
- log structured audit and delivery attempts

### Execution backends

The notification service should dispatch to one of two backends.

#### A. Alpha mail delivery backend

Use for:
- auth
- invites
- workspace access
- security
- product lifecycle notifications

Possible implementation:
- current SMTP-based mailers first
- provider-backed mail transport later
- templated delivery engine later

#### B. Lago billing notification adapter

Use for:
- invoice emails
- payment receipts
- credit note emails
- dunning / collection notifications

Possible implementation:
- reuse Lago invoice resend / receipt / dunning capabilities
- expose Alpha-owned APIs that delegate to Lago where needed

---

## Product Ownership Rule

Users should still experience Alpha as the owner of the workflow.

That means:

- Alpha UI triggers the action
- Alpha uses Alpha terminology
- Alpha decides when a notification should be sent
- Alpha should not expose Lago email settings as the normal product language unless required

Even when Lago executes delivery, Alpha remains the product owner.

---

## Notification Routing Rules

A simple long-term decision table:

1. Is this tied to browser identity, workspace access, or security?
- use Alpha delivery

2. Is this tied to Alpha product onboarding or workflow state outside billing documents?
- use Alpha delivery

3. Is this a finance-document delivery or resend flow?
- use Lago delivery

4. Is this collections/dunning behavior tied to billing entities and payment collections?
- use Lago delivery first

5. Is this a new notification class with unclear ownership?
- default to Alpha policy ownership, then choose backend explicitly

---

## Current Alpha State

Alpha already owns direct mail delivery for:

- workspace invitations
- password reset

That is correct and should stay Alpha-owned.

Alpha currently does **not** yet have a unified notification service. It has point mailers.

That is acceptable as an intermediate state, but not the final design.

---

## Migration Approach

### Phase 1. Keep current Alpha auth/access mailers

Keep:
- invitation mailer
- password reset mailer

Do not move these into Lago.

### Phase 2. Introduce Alpha notification boundary

Add:
- notification intent model
- notification classification
- structured delivery audit
- backend dispatch abstraction

### Phase 3. Route billing-document flows intentionally

For invoice/receipt/credit-note and dunning notifications:

- Alpha triggers the workflow
- Alpha delegates execution to Lago where Lago is already the right delivery engine
- Alpha records the product-level action and result

### Phase 4. Unify observability

Add:
- delivery attempt logs
- notification event audit
- request/workspace/customer correlation
- backend-specific failure classification

---

## API and UI Implications

### Alpha APIs should expose notification actions in Alpha language

Examples:
- `send payment setup reminder`
- `resend invoice email`
- `send overdue collection reminder`

The API handler may delegate to:
- Alpha mailer
- Lago adapter

But the outward product model stays Alpha-owned.

### UI should not expose backend distinction by default

Normal operators should see:
- notification sent
- failed to send
- needs attention
- resent successfully

Advanced/internal views may show:
- delivery backend = `alpha` or `lago`
- failure detail
- audit trace

---

## What Should Stay Out of Scope for Now

Do not overbuild:

- full cross-product notification center
- user notification preferences for every domain
- multi-channel delivery beyond email initially
- template studio for all notification classes

Wave 1 only needs a correct ownership model and a clean split.

---

## Recommended Next Implementation Order

1. Document the split and stop discussing notifications as one undifferentiated system.
2. Keep auth/access delivery in Alpha.
3. Build Alpha-owned resend/notification actions for billing workflows, delegating to Lago where appropriate.
4. Introduce `NotificationService` as a shared policy and routing boundary.
5. Add unified audit/observability around notification attempts.

---

## Short Decision

Best long-term production approach:

- Alpha owns notification intent and routing
- Alpha sends auth/workspace/security/product notifications itself
- Alpha uses Lago selectively for billing-document and collections delivery
- Lago is a specialized notification execution backend, not Alpha's general notification system
