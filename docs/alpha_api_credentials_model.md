# Alpha API Credentials Model

## Goal

Define a production-grade API credential model for Alpha that cleanly separates:

- human access
- workspace machine access
- platform machine access
- bootstrap and recovery operations

This document answers the product question:

- should tenants generate their own API keys?
- should the platform also have that capability?

The short answer is:

- workspace admins should own normal workspace API key issuance
- platform admins should retain explicit bootstrap and break-glass capability
- human browser users should not use API keys as their primary login path

Read together with:

- [Alpha Workspace Access Model](./alpha-workspace-access-model.md)
- [Alpha Workspace Access Spec](./alpha_workspace_access_spec.md)
- [Alpha Billing Execution Model](./alpha-billing-execution-model.md)
- [Alpha Workspace Billing Binding Spec](./alpha_workspace_billing_binding_spec.md)

---

## Problem With The Current Model

Today Alpha has two API key families:

- tenant-scoped `api_keys`
- platform-scoped `platform_api_keys`

That foundation is useful, but the operating model is still transitional.

Current-state behavior mixes three concerns too closely:

1. human browser access
- older assisted flows treated API keys as a practical browser sign-in mechanism

2. machine credentials
- tenant API keys are also used for programmatic access and automation

3. bootstrap ownership
- platform admins bootstrap the first tenant admin key and therefore still sit too close to the tenant's steady-state credential model

That is acceptable for assisted bring-up.
It is not the long-term enterprise boundary.

---

## Long-Term Product Rule

Alpha should treat credentials in three distinct layers.

### 1. Human identity

Used for:
- browser login
- workspace membership
- workspace role enforcement
- approvals and audit attribution

Credential types:
- SSO
- password login
- future enterprise identity providers

Important rule:
- humans authenticate as users, not as API keys

### 2. Workspace machine credentials

Used for:
- tenant automation
- CI/CD
- backend jobs that act inside one workspace
- integrations that operate within one workspace boundary

Credential types:
- workspace API keys
- future service-account credentials

Important rule:
- these are scoped to one workspace
- these are owned by workspace admins in the normal operating model

### 3. Platform machine credentials

Used for:
- platform-level internal operations
- cross-workspace automation
- tenant bootstrap
- operator tooling
- emergency recovery

Credential types:
- platform API keys

Important rule:
- these are not normal tenant operating credentials
- these must remain rare, explicit, and auditable

---

## Recommended Ownership Model

### Workspace admins should generate workspace API keys

This should be the default production model.

Why:
- the workspace owns its own automation boundary
- the platform should not have to mint normal runtime credentials for every tenant workflow
- this reduces support coupling and improves tenant autonomy
- this matches enterprise expectations around delegated administration

Workspace admins should be able to:
- create workspace API keys
- rotate workspace API keys
- revoke workspace API keys
- view audit history for workspace API credentials
- set expiry, name, purpose, and future scopes

### Platform admins should still have capability

But only for clearly bounded cases.

Platform capability is still needed for:
- initial bootstrap when a workspace has no admin yet
- assisted onboarding
- break-glass recovery
- platform-owned background integrations
- migrations or forced revocation events

That means the answer is not:
- tenant only
nor:
- platform only

The correct enterprise answer is:
- tenant-owned by default
- platform-capable by exception

---

## Product Policy

### Default path

1. platform admin creates workspace
2. platform admin invites initial workspace admin
3. workspace admin accepts invite and gains browser access
4. workspace admin creates any needed workspace API keys for automation

### Assisted path

Allowed when:
- the customer is being onboarded white-glove
- the workspace admin is not ready yet
- a migration/bootstrap path still needs operator help

Flow:
1. platform admin creates workspace
2. platform admin optionally bootstraps one initial workspace admin machine credential
3. platform admin hands that credential to the authorized workspace owner through secure out-of-band delivery
4. workspace owner rotates or replaces it after first login

### Break-glass path

Allowed when:
- the workspace has lost admin access
- SSO or membership is broken
- incident recovery requires emergency access

Rules:
- platform-generated break-glass credentials must be time-boxed
- reason must be recorded
- audit trail must be explicit
- follow-up rotation/revocation must be mandatory

---

## Enterprise Design Principles

### 1. Human access and machine access must stay separate

Do not use tenant API keys as the normal browser-auth model.

Browser users should use:
- SSO
- password
- workspace membership

Machine actors should use:
- workspace API keys
- future service accounts

This is the most important long-term correction.

### 2. API keys should represent non-human principals

A key should not effectively mean:
- "this is Alice logging in"

It should mean:
- "this is the Acme CI deploy job"
- "this is the Acme ERP sync worker"
- "this is the platform migration runner"

That means key metadata should move toward:
- service account / credential owner name
- purpose
- environment
- expiry
- creator
- last used

### 3. Platform keys and workspace keys should never be ambiguous

Platform keys:
- can cross workspace boundaries when allowed
- can call `/internal/*`
- must be heavily restricted and audited

Workspace keys:
- can only act inside one workspace
- must never be silently promoted into platform scope

### 4. Bootstrap is not the steady-state model

The fact that platform can create the first key does not mean platform should keep owning normal workspace machine access.

Bootstrap is a setup and recovery capability.
Not the permanent operating model.

### 5. One-time secret reveal remains correct

Continue the current rule:
- raw secret returned only on create or rotate
- only hash stored at rest

That is already the right production behavior.

---

## Recommended Capability Matrix

### Platform admin

Can:
- create platform API keys
- revoke platform API keys
- list platform API keys
- create initial workspace bootstrap key
- force-revoke workspace keys in incident/recovery situations
- inspect workspace key inventory and audit when authorized by policy

Should not do routinely:
- create normal day-to-day tenant automation keys on behalf of healthy workspaces

### Workspace admin

Can:
- create workspace API keys for that workspace
- revoke and rotate those keys
- inspect audit history for that workspace
- manage future service accounts for that workspace

Cannot:
- create platform API keys
- mint keys for other workspaces

### Workspace writer / reader

Default recommendation:
- cannot create machine credentials

Rationale:
- credential issuance is an admin-level security action

### Platform automation

Can:
- use platform API keys or future workload identity
- act across workspaces only when explicitly required

### Workspace automation

Can:
- use workspace API keys or future service-account credentials
- act only within one workspace boundary

---

## Data Model Direction

### Keep

Existing:
- `api_keys`
- `platform_api_keys`
- API key audit tables

### Evolve

Current tenant `api_keys` should become conceptually:
- workspace machine credentials

Long-term recommended shape:
- keep the current table if needed for compatibility
- but treat rows as credentials for workspace-scoped machine principals
- add metadata so the key is attached to a non-human owner identity

Recommended future fields:
- `owner_type`
  - `service_account`
  - `bootstrap`
  - `break_glass`
- `owner_id`
- `purpose`
- `created_by_user_id`
- `created_by_platform_user`
- `last_rotated_at`
- `rotation_required_at`
- `revocation_reason`

### Stronger long-term model

Introduce first-class `service_accounts` later.

Recommended model:
- `service_accounts`
  - workspace-scoped by default
  - optional platform-scoped variant later if needed
- `service_account_credentials`
  - one or more secrets/keys for a service account
  - rotation history
  - independent revoke/expire states

Then Alpha can distinguish clearly between:
- human users
- workspace service accounts
- platform service accounts

That is stronger than treating every key as a loose standalone row forever.

---

## API Design Direction

### Workspace-facing API

Normal workspace machine credential lifecycle should be browser-admin driven.

Recommended product-facing paths:
- `GET /internal/tenants/{id}/api-credentials`
- `POST /internal/tenants/{id}/api-credentials`
- `POST /internal/tenants/{id}/api-credentials/{credential_id}/rotate`
- `POST /internal/tenants/{id}/api-credentials/{credential_id}/revoke`
- `GET /internal/tenants/{id}/api-credentials/audit`

Why not only keep `/v1/api-keys`?
- because Alpha is moving toward browser-user ownership and workspace admin UX
- browser-admin flows should not feel like raw machine-auth APIs
- a workspace access/security surface should own this explicitly

The existing `/v1/api-keys` endpoints can remain as compatibility/runtime APIs.
But the long-term product surface should be:
- `Workspace Access / API Credentials`

### Platform-facing API

Keep explicit platform-only surfaces, for example:
- `GET /internal/platform/api-credentials`
- `POST /internal/platform/api-credentials`
- `POST /internal/platform/api-credentials/{id}/revoke`

### Bootstrap and break-glass API

Keep separate from normal key issuance.

Recommended explicit paths:
- `POST /internal/tenants/{id}/bootstrap-admin-key`
- future:
  - `POST /internal/tenants/{id}/break-glass-key`

That distinction matters.
A bootstrap or emergency key should not look identical to a normal workspace automation credential in policy or audit.

---

## Audit And Governance Requirements

For enterprise use, API credentials need better policy than just create/revoke.

Minimum long-term requirements:
- creator identity recorded
- actor identity recorded for create/rotate/revoke
- one-time secret reveal only
- last-used timestamp
- expiry support
- audit export
- reason metadata for break-glass issuance and forced revocation

Recommended next governance features:
- mandatory expiry for platform keys
- optional mandatory expiry for workspace keys
- key naming policy
- environment labeling (`prod`, `staging`, `ci`, `local`)
- future IP allowlists or network policy hooks
- future scope narrowing beyond coarse `reader|writer|admin`

---

## UI / UX Rules

### Workspace UI

Workspace admins should see:
- API Credentials
- machine credential purpose
- status
- expiry
- last used
- create / rotate / revoke actions
- audit trail

They should not need to think in terms of:
- raw tenant bootstrap
- root platform operator mechanics

### Platform UI

Platform admins should see:
- platform credentials
- workspace bootstrap history
- break-glass actions
- revocation and audit controls

Platform UI should make it obvious when an action is exceptional.

### Browser sign-in

Do not present API keys as the preferred browser sign-in path for humans.

That model was acceptable during transition.
It should not remain the long-term default.

---

## Migration Plan

### Phase 1

Keep current tables and endpoints, but change policy and UX:
- browser users manage workspace API credentials through workspace access/security screens
- keep `/v1/api-keys` as runtime/API compatibility surface
- stop framing tenant API keys as the primary way users access the product

### Phase 2

Add richer metadata and explicit ownership classification:
- bootstrap
- service-account-like
- break-glass

### Phase 3

Introduce first-class service accounts and map API credentials to those principals.

### Phase 4

Tighten governance:
- expiry defaults
- narrower scopes
- optional network restrictions
- approval or dual-control for emergency credentials if needed

---

## Recommended Answer To The Product Question

### Should the tenant generate keys for themselves?

Yes.
That should be the normal long-term model.

More precisely:
- workspace admins should generate workspace-scoped machine credentials for their own workspace

### Should the platform also have that capability?

Yes.
But only as:
- bootstrap
- assisted onboarding
- emergency recovery
- platform-owned automation

Platform capability is necessary.
Platform primacy is not.

That is the enterprise balance.

---

## Bottom Line

The correct long-term Alpha model is:

- humans authenticate with browser identity and membership
- workspaces own their own machine credentials
- platform retains exceptional bootstrap and recovery power
- platform and workspace machine credentials stay separate
- API keys evolve toward service-account-backed credentials

If Alpha follows that model, the system stays:
- enterprise-credible
- auditable
- tenant-autonomous
- compatible with assisted onboarding and break-glass operations

If Alpha keeps using tenant API keys as the normal human access model, the product will keep mixing identity and automation in a way that does not scale cleanly.
