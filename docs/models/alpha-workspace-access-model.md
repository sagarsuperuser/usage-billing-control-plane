# Alpha Workspace Access Model

## Goal

After a platform admin creates a workspace, Alpha must provide a clean handoff into tenant ownership.

That means:

- the platform admin should not remain the long-term operator for tenant billing work
- a tenant user should be explicitly connected to the workspace through Alpha
- that connection should happen through user identity and membership, not raw tenant ids or internal pre-provisioning forever

This document defines the long-term product model for that handoff.

---

## Core Product Rule

Workspace setup is not complete when:

- the workspace exists
- billing is attached

Workspace setup is complete when:

- the workspace exists
- billing is attached
- at least one tenant admin or operator has access to the workspace

If Alpha stops before that, the platform admin owns too much of the tenant workflow and the product stays operator-heavy.

---

## Product Concepts

Use four different concepts:

### 1. User

A human browser identity in Alpha.

Examples:

- `sagar10018233@gmail.com`
- `superuser0x777@gmail.com`

This is the identity that signs in with:

- password
- SSO

### 2. Workspace

The tenant operating boundary inside Alpha.

This is where users later manage:

- customers
- pricing
- subscriptions
- invoices
- payments

### 3. Workspace Membership

The durable link between:

- user
- workspace
- role

This is the real access control record.

Examples:

- `user_123` is `admin` of workspace `acme`
- `user_456` is `writer` of workspace `globex`

### 4. Workspace Invitation

The onboarding and delegation record.

This exists so a platform admin or workspace admin can say:

- invite this email to this workspace
- with this role

The invitation should resolve into a membership once accepted.

---

## Correct End-to-End Flow

### Platform setup flow

1. Platform admin signs into Alpha
2. Platform admin creates or verifies a billing connection
3. Platform admin creates a workspace
4. Platform admin attaches one active billing connection to that workspace
5. Platform admin invites a workspace admin or operator to that workspace

At that point the platform handoff is complete.

### Tenant ownership flow

1. Invited user receives invite
2. User signs in or creates an Alpha browser identity
3. Alpha verifies the invite
4. Alpha activates a workspace membership
5. Alpha lands the user inside the workspace tenant surface
6. The tenant operator creates the first customer, pricing, subscriptions, and later billing operations

That is the correct product ownership model.

---

## What Alpha Should Own

Alpha should own the full workspace access lifecycle:

- invite creation
- invite acceptance
- membership creation
- membership listing
- role changes
- membership removal
- workspace-aware post-login routing

Lago should not own any part of this flow.

---

## Access Rules

### Platform admin

Can:

- create workspaces
- attach billing
- invite the initial workspace admin
- inspect cross-workspace health

Should not be the normal owner of:

- customer creation
- pricing maintenance
- subscription operations

except during assisted onboarding.

### Workspace admin

Can:

- manage workspace users
- invite additional users
- change workspace roles
- manage tenant billing workflows

### Workspace writer

Can:

- manage operational billing work inside the workspace

Cannot:

- manage workspace members unless explicitly elevated

### Workspace reader

Can:

- view the workspace

Cannot:

- change billing configuration or operational data

---

## Product UX Rules

### After workspace creation

Alpha should present a clear next step:

- `Invite workspace admin`

not:

- `Copy tenant id`
- `Use this API key`
- `Ask engineering to provision access`

### Workspace detail should answer

- Is billing connected?
- Does this workspace have an owner?
- How many members have access?
- Is an invite pending?

### Login routing should answer

After login:

- platform-only user lands on platform home
- single-workspace tenant user lands directly in that workspace
- multi-workspace user lands on a workspace chooser or recent workspace

That removes ambiguity and keeps the UI simple.

---

## Recommended Long-Term Data Model

### Existing

- `users`
- `user_tenant_memberships`

These are the correct foundation.

### Add

- `workspace_invitations`

Recommended shape:

- `id`
- `email`
- `workspace_id`
- `role`
- `status`
- `invited_by_user_id`
- `invited_by_platform_user`
- `token_hash`
- `expires_at`
- `accepted_at`
- `accepted_by_user_id`
- `created_at`
- `updated_at`

Status values:

- `pending`
- `accepted`
- `expired`
- `revoked`

The invitation is a lifecycle object.
The membership is the durable access object.

Do not collapse those into one table.

---

## Auth and SSO Interaction

Workspace access should work with both:

- password auth
- SSO

The rule is:

- authentication proves who the user is
- membership determines what workspace they can access

SSO should not directly imply workspace access by itself unless:

- Alpha has an approved auto-provisioning or domain policy

Default long-term rule:

- invite or approved provisioning creates membership
- login resolves into existing memberships

That is safer and easier to reason about.

---

## Transitional Rules

Today Alpha still supports pre-provisioned users and membership creation through backend paths.

That is acceptable as a transition.

It should not remain the main product path.

The target path is:

- invite user
- user signs in
- membership activates

So pre-provisioning should become:

- internal admin fallback
- staging/bootstrap path

not the primary tenant access workflow.

---

## API Shape To Aim For

### Platform-side

- `POST /internal/workspaces/{id}/invitations`
- `GET /internal/workspaces/{id}/members`
- `PATCH /internal/workspaces/{id}/members/{user_id}`
- `DELETE /internal/workspaces/{id}/members/{user_id}`

### User-side

- `GET /v1/ui/invitations/{token}`
- `POST /v1/ui/invitations/{token}/accept`
- `GET /v1/ui/workspaces`

### Session resolution

- `GET /v1/ui/sessions/me`

should eventually include:

- user identity
- memberships or selected workspace
- active workspace context when applicable

---

## UI Surfaces To Aim For

### Platform

- `Workspaces`
- `Workspace detail`
  - `Billing`
  - `Access`
  - `Members`
  - `Pending invites`

### Tenant

- `Settings`
  - `Members`
  - `Roles`

### Auth

- invite acceptance
- workspace chooser when needed

These should stay simple and role-aware.

---

## Decision

The long-term solution for connecting a tenant to a workspace is:

- browser user identity
- workspace membership
- invitation-driven handoff

not:

- platform admin doing tenant work forever
- raw tenant ids
- API key handoff
- permanent backend-only pre-provisioning

---

## Recommended Next Slice

The next access-control slice after current workspace billing work should be:

1. `workspace invitations`
2. `workspace members` list and role management
3. `workspace-aware post-login routing`

That is the missing bridge between:

- platform setup
- tenant ownership
