# Alpha Workspace Access Spec

## Goal

Implement the product and backend slice that turns workspace creation into real tenant ownership.

After this slice:

- a platform admin can invite the initial workspace admin
- a workspace admin can invite additional users
- invited users can accept access through Alpha
- memberships become the durable access record
- login can route users into the correct workspace context

This is the implementation follow-up to:

- [Alpha Workspace Access Model](../models/alpha-workspace-access-model.md)

---

## Product Scope

### In scope

- workspace invitation model
- invitation create/list/revoke flows
- invitation acceptance flow
- workspace members list
- workspace role update and removal
- workspace-aware post-login routing
- role enforcement for platform admin vs workspace admin

### Out of scope

- full email delivery provider implementation
- self-signup marketing flow
- password reset lifecycle
- SCIM
- domain-based auto-provisioning
- full auth settings UI beyond current SSO foundation

---

## Product Rules

1. Workspace setup is not complete until access is delegated.
2. Invitations are onboarding objects, memberships are durable access objects.
3. Authentication and authorization stay separate:
   - auth proves identity
   - membership grants workspace access
4. Platform admins can create the initial workspace handoff.
5. Workspace admins own ongoing member management.

---

## User Stories

### Platform admin

- can invite the first workspace admin after workspace setup
- can see whether the workspace has members or pending invites
- can resend or revoke a pending invite

### Workspace admin

- can list members of their workspace
- can invite additional users to their workspace
- can change roles for existing members
- can remove members

### Invited user

- can open an invite link
- can sign in or create a browser identity
- can accept the invite
- lands in the assigned workspace

### Multi-workspace user

- can see multiple memberships
- can choose the workspace context after login

---

## Domain Model

### Existing foundation

- `users`
- `user_tenant_memberships`

### New table

`workspace_invitations`

Recommended columns:

- `id TEXT PRIMARY KEY`
- `workspace_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE`
- `email TEXT NOT NULL`
- `role TEXT NOT NULL`
- `status TEXT NOT NULL`
- `token_hash TEXT NOT NULL`
- `expires_at TIMESTAMPTZ NOT NULL`
- `accepted_at TIMESTAMPTZ`
- `accepted_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL`
- `invited_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL`
- `invited_by_platform_user BOOLEAN NOT NULL DEFAULT false`
- `revoked_at TIMESTAMPTZ`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Suggested constraints:

- `role IN ('reader', 'writer', 'admin')`
- `status IN ('pending', 'accepted', 'expired', 'revoked')`
- unique pending invite per `workspace_id + email`

Recommended indexes:

- `(workspace_id, status)`
- `(email, status)`
- `(token_hash)`

---

## Service Boundary

Introduce:

`WorkspaceAccessService`

Responsibilities:

- create invitation
- revoke invitation
- list workspace invitations
- accept invitation
- list workspace members
- update workspace member role
- remove workspace member
- resolve post-login workspace landing

This service should own all access lifecycle rules.

Do not spread invitation logic across auth handlers, tenant service, and UI handlers independently.

---

## API Shape

### Platform/workspace admin APIs

#### Invitations

- `GET /internal/tenants/{id}/invitations`
- `POST /internal/tenants/{id}/invitations`
- `POST /internal/tenants/{id}/invitations/{invite_id}/revoke`

Request for create:

```json
{
  "email": "ops@acme.com",
  "role": "admin"
}
```

Response shape:

```json
{
  "invitation": {
    "id": "wsi_123",
    "workspace_id": "acme",
    "email": "ops@acme.com",
    "role": "admin",
    "status": "pending",
    "expires_at": "2026-03-25T12:00:00Z",
    "created_at": "2026-03-18T12:00:00Z",
    "updated_at": "2026-03-18T12:00:00Z"
  }
}
```

#### Members

- `GET /internal/tenants/{id}/members`
- `PATCH /internal/tenants/{id}/members/{user_id}`
- `DELETE /internal/tenants/{id}/members/{user_id}`

PATCH request:

```json
{
  "role": "writer"
}
```

### User-facing invite APIs

- `GET /v1/ui/invitations/{token}`
- `POST /v1/ui/invitations/{token}/accept`

These should work with browser-user auth, not API keys.

Acceptance response should include:

- accepted membership
- resolved workspace id
- next landing route

### Session/workspace APIs

- `GET /v1/ui/workspaces`
- `POST /v1/ui/workspaces/select`

This becomes important for multi-workspace users.

---

## Permission Rules

### Platform admin

Can:

- manage workspace invitations for any workspace
- inspect members
- bootstrap the first workspace admin

### Workspace admin

Can:

- invite users into their own workspace
- change roles in their own workspace
- remove users from their own workspace

### Workspace writer and reader

Cannot:

- manage invites or membership

### Invite acceptance

An invite can be accepted only when:

- the token is valid
- status is `pending`
- token is not expired
- authenticated user email matches invite email

That last rule is important.

Do not allow arbitrary logged-in users to consume someone else’s invite token.

---

## Routing Rules

### After login

If user has:

1. platform role only
- route to platform home

2. one workspace membership
- route directly to that workspace default landing

3. multiple workspace memberships
- route to workspace chooser

4. platform role and workspace memberships
- route to last used context or chooser, depending on product choice

Recommended first version:

- platform-only -> platform home
- one workspace -> direct workspace landing
- multiple workspaces -> chooser

### After invite acceptance

- route directly into the invited workspace
- show a simple success state
- do not drop the user on a generic overview page

---

## UI Surfaces

### Platform

#### Workspace detail

Add:

- `Access` section
- members count
- pending invites count
- CTA:
  - `Invite workspace admin`

#### Invite flow

Simple form:

- email
- role

Avoid exposing internal auth details.

### Tenant

#### Settings > Members

Show:

- current members
- roles
- pending invites
- invite action

This should be a small, clear SaaS admin screen, not a dense access-control console.

### Auth

#### Invite acceptance screen

States:

- token valid, sign in required
- token valid, ready to accept
- accepted
- expired
- revoked
- wrong account/email mismatch

---

## Email and Delivery Model

First implementation can separate:

1. invitation lifecycle
2. email transport

That means:

- backend creates invite
- backend can return a generated acceptance URL
- UI can display/copy the link

This is acceptable as a first slice.

Then add:

- actual email delivery provider
- resend flow

Do not block the data model and access lifecycle on full email infrastructure.

---

## Backend Implementation Order

### Slice A

- migration for `workspace_invitations`
- domain/store/service support
- invitation create/list/revoke
- member list/update/remove

### Slice B

- UI invite acceptance endpoints
- token hashing and acceptance logic
- membership activation

### Slice C

- workspace-aware session routing
- workspace chooser when needed

### Slice D

- email send/resend integration
- polish and audit hooks

---

## Testing Requirements

### Backend

- create invite
- prevent duplicate pending invite
- revoke invite
- accept invite with matching email
- reject invite with wrong email
- reject expired invite
- create membership on acceptance
- update member role
- remove member
- post-login routing resolution

### UI

- platform admin can invite initial workspace admin
- workspace admin can invite additional users
- invite acceptance flow succeeds
- single-workspace user lands directly in workspace
- multi-workspace user sees chooser

---

## Exit Criteria

This slice is complete when:

1. A platform admin can invite the initial workspace admin from Alpha.
2. A workspace admin can manage members and invites from Alpha.
3. An invited user can accept access without backend-only pre-provisioning.
4. Login routing respects workspace memberships.
5. Workspace setup no longer depends on hidden manual access assignment to become truly usable.
