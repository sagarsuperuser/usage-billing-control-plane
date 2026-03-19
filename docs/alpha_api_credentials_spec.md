# Alpha API Credentials Spec

## Goal

Implement the first production-grade API credential slice for Alpha.

This spec turns the policy from:

- [Alpha API Credentials Model](./alpha_api_credentials_model.md)

into an implementation plan for:

- schema
- service boundaries
- API shape
- UI surface
- migration from the current `/v1/api-keys` model

The goal is not to replace the current key model in one shot.
The goal is to move Alpha to the right enterprise boundary without breaking existing integrations.

---

## Product Outcome

After this slice:

- browser users remain the primary human access model
- workspace admins can manage workspace machine credentials from Alpha UI
- platform admins retain explicit platform/bootstrap/break-glass credential capability
- the existing `/v1/api-keys` API remains compatible for machine clients
- Alpha starts treating API keys as machine credentials, not normal human login credentials

This slice should be implemented as a security and ownership improvement, not just a rename.

---

## Scope

### In scope

- workspace-facing API credential inventory and lifecycle
- platform-facing API credential inventory and lifecycle
- explicit credential classification metadata
- bootstrap and break-glass distinction in product and audit
- compatibility mapping from existing `api_keys` records
- UI integration into workspace access/security

### Out of scope

- full service-account model
- SCIM
- workload identity federation
- IP allowlists
- fine-grained per-endpoint scopes beyond current role model
- customer-managed key encryption/HSM features

---

## Product Rules

1. Human users authenticate with browser identity, not API keys.
2. Workspace API credentials are machine credentials scoped to one workspace.
3. Platform API credentials are machine credentials for platform operations only.
4. Platform bootstrap and break-glass credential issuance must be explicit and exceptional.
5. Existing `/v1/api-keys` clients must keep working during migration.
6. Workspace admins own normal workspace credential lifecycle.

---

## Domain Model

### Current foundation

Existing:
- `api_keys`
- `platform_api_keys`
- `api_key_audit_events`
- `api_key_audit_export_jobs`

This spec keeps those tables initially.

### New metadata on workspace API keys

Recommended new columns on `api_keys`:

- `owner_type TEXT NOT NULL DEFAULT 'workspace_credential'`
- `owner_id TEXT NOT NULL DEFAULT ''`
- `purpose TEXT NOT NULL DEFAULT ''`
- `environment TEXT NOT NULL DEFAULT ''`
- `created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL`
- `created_by_platform_user BOOLEAN NOT NULL DEFAULT false`
- `last_rotated_at TIMESTAMPTZ`
- `rotation_required_at TIMESTAMPTZ`
- `revocation_reason TEXT NOT NULL DEFAULT ''`

Suggested `owner_type` values:
- `workspace_credential`
- `bootstrap`
- `break_glass`
- `service_account`

Notes:
- `service_account` is reserved for the later model but worth allowing now
- `owner_id` can be empty during compatibility migration
- `purpose` and `environment` should be optional at first but encouraged in UI

### New metadata on platform API keys

Recommended new columns on `platform_api_keys`:

- `owner_type TEXT NOT NULL DEFAULT 'platform_credential'`
- `owner_id TEXT NOT NULL DEFAULT ''`
- `purpose TEXT NOT NULL DEFAULT ''`
- `environment TEXT NOT NULL DEFAULT ''`
- `created_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL`
- `last_rotated_at TIMESTAMPTZ`
- `rotation_required_at TIMESTAMPTZ`
- `revocation_reason TEXT NOT NULL DEFAULT ''`

Suggested `owner_type` values:
- `platform_credential`
- `bootstrap`
- `break_glass`
- `platform_service_account`

### Audit expansion

Keep the current audit tables, but expand metadata expectations.

Required audit metadata moving forward:
- `owner_type`
- `owner_id`
- `purpose`
- `environment`
- `created_by_user_id`
- `created_by_platform_user`
- `revocation_reason`
- `break_glass_reason`
- `bootstrap_source`

The actor columns should continue to record machine actor IDs where applicable.
If a browser user initiated the action, user attribution should be added to metadata and later formalized if needed.

---

## Service Boundary

Introduce:

`APICredentialService`

Responsibilities:
- create workspace credential
- list workspace credentials
- rotate workspace credential
- revoke workspace credential
- list platform credentials
- create platform credential
- revoke platform credential
- issue bootstrap workspace credential
- issue break-glass workspace credential
- translate compatibility responses from existing key models

This service should sit above:
- `APIKeyService`
- `PlatformAPIKeyService`

Do not keep pushing browser-admin credential policy directly into raw `/v1/api-keys` handlers.

### Suggested structure

- `WorkspaceCredentialService`
- `PlatformCredentialService`

or a single:
- `APICredentialService`

Either is acceptable.
The important point is:
- browser-admin credential lifecycle should not be encoded directly in low-level key CRUD handlers

---

## API Shape

### Workspace-facing browser-admin API

These belong under workspace access/security.

#### List workspace credentials

- `GET /internal/tenants/{id}/api-credentials`

Response:

```json
{
  "items": [
    {
      "id": "key_123",
      "workspace_id": "acme",
      "name": "Acme CI",
      "role": "writer",
      "state": "active",
      "owner_type": "workspace_credential",
      "purpose": "CI deploy pipeline",
      "environment": "prod",
      "last_used_at": "2026-03-19T10:00:00Z",
      "expires_at": "2026-06-01T00:00:00Z",
      "created_at": "2026-03-19T09:00:00Z",
      "updated_at": "2026-03-19T09:00:00Z"
    }
  ]
}
```

#### Create workspace credential

- `POST /internal/tenants/{id}/api-credentials`

Request:

```json
{
  "name": "Acme CI",
  "role": "writer",
  "purpose": "CI deploy pipeline",
  "environment": "prod",
  "expires_at": "2026-06-01T00:00:00Z"
}
```

Response:

```json
{
  "api_credential": {
    "id": "key_123",
    "workspace_id": "acme",
    "name": "Acme CI",
    "role": "writer",
    "state": "active",
    "owner_type": "workspace_credential",
    "purpose": "CI deploy pipeline",
    "environment": "prod",
    "created_at": "2026-03-19T09:00:00Z",
    "updated_at": "2026-03-19T09:00:00Z"
  },
  "secret": "alpha_live_..."
}
```

#### Rotate workspace credential

- `POST /internal/tenants/{id}/api-credentials/{credential_id}/rotate`

Response includes one-time `secret`.

#### Revoke workspace credential

- `POST /internal/tenants/{id}/api-credentials/{credential_id}/revoke`

Request:

```json
{
  "reason": "pipeline migrated to new credential"
}
```

#### Audit

- `GET /internal/tenants/{id}/api-credentials/audit`

This can proxy or adapt the existing audit event model.

### Platform-facing browser-admin API

#### List platform credentials
- `GET /internal/platform/api-credentials`

#### Create platform credential
- `POST /internal/platform/api-credentials`

#### Revoke platform credential
- `POST /internal/platform/api-credentials/{id}/revoke`

### Exceptional flows

#### Bootstrap workspace admin credential

Keep:
- `POST /internal/tenants/{id}/bootstrap-admin-key`

But change its product semantics:
- explicit bootstrap path
- explicit `owner_type=bootstrap`
- explicit audit metadata

#### Break-glass credential

Add:
- `POST /internal/tenants/{id}/break-glass-key`

Request:

```json
{
  "reason": "workspace admin lost access during incident",
  "expires_at": "2026-03-20T00:00:00Z"
}
```

Rules:
- platform admin only
- expiry required
- reason required
- audit required
- clearly marked in UI/API responses

### Existing runtime API compatibility

Keep:
- `POST /v1/api-keys`
- `GET /v1/api-keys`
- `POST /v1/api-keys/{id}/rotate`
- `POST /v1/api-keys/{id}/revoke`
- audit/export endpoints

But reposition them as:
- compatibility/runtime API for machine clients
- not the preferred browser-admin product surface

---

## Permission Rules

### Platform admin browser user

Can:
- manage platform credentials
- bootstrap workspace admin credential
- issue break-glass workspace credential
- inspect workspace credential inventory
- revoke workspace credentials

Should not routinely:
- create normal workspace automation credentials for healthy workspaces

### Workspace admin browser user

Can:
- list workspace credentials
- create workspace credentials
- rotate workspace credentials
- revoke workspace credentials
- inspect workspace credential audit

Cannot:
- manage platform credentials
- issue break-glass credentials
- create credentials for other workspaces

### Workspace writer / reader browser user

Cannot:
- create or revoke credentials
- rotate credentials
- access credential audit by default

### Machine credentials

Workspace credential:
- tenant/workspace scope only
- no `/internal/*`
- no cross-workspace operations

Platform credential:
- platform scope only
- may call `/internal/*` when allowed by role
- cross-workspace actions are allowed only through explicit platform permissions

---

## UI Surface

### Workspace access/security

Extend the current workspace access page rather than creating a separate top-level product area.

Current surface:
- [tenant-workspace-access-screen.tsx](../web/src/components/workspaces/tenant-workspace-access-screen.tsx)

Add a second section below members/invitations:
- `API Credentials`

Recommended blocks:
- credential summary
- create credential form
- credentials table/list
- audit entry point

Fields in create form:
- name
- role
- purpose
- environment
- expiry

Credential list should show:
- name
- role
- owner type
- state
- last used
- expires at
- created at
- actions: rotate / revoke

Important UX rule:
- secret reveal must happen only once in a dedicated post-create state
- copy affordance should be explicit
- subsequent views must never reveal the raw secret again

### Platform UI

Add platform credential management under:
- `Team & Security`

Subsections:
- `Platform Credentials`
- `Workspace Bootstrap`
- `Break-glass Actions`

Use clear warning styling for:
- bootstrap credential issuance
- break-glass issuance
- forced workspace credential revocation

### Copy rules

Prefer:
- `API Credentials`
- `Machine credentials`
- `Workspace automation`
- `Bootstrap`
- `Break-glass`

Avoid using:
- `tenant API key` as the primary user-facing concept long term

---

## Migration Plan

### Phase 1: Policy and UI shift

- browser-admin users manage workspace credentials through workspace access/security
- `/v1/api-keys` remains compatible
- docs and UI stop framing API keys as normal human access

Data behavior:
- existing `api_keys` rows are treated as `owner_type=workspace_credential` unless explicitly bootstrap-created

### Phase 2: Metadata migration

Add columns listed above.

Backfill rules:
- existing tenant keys -> `owner_type=workspace_credential`
- keys created by `/internal/tenants/{id}/bootstrap-admin-key` -> `owner_type=bootstrap`
- existing platform keys -> `owner_type=platform_credential`
- `purpose`, `environment`, `owner_id`, `revocation_reason` default to empty string

### Phase 3: New browser-admin APIs

Ship:
- `/internal/tenants/{id}/api-credentials`
- `/internal/platform/api-credentials`
- `/internal/tenants/{id}/break-glass-key`

Internally these can call the existing key services at first.

### Phase 4: Service-account model

Introduce:
- `service_accounts`
- `service_account_credentials`

Then new workspace credential creation should target service accounts rather than loose keys.

### Phase 5: Runtime compatibility narrowing

After service-account-backed credentials are established:
- keep `/v1/api-keys` as compatibility alias if needed
- or progressively migrate clients to explicit credential/service-account APIs

Do not remove compatibility until live integrations have a safe migration path.

---

## Implementation Order

### Slice A: Metadata and service seam

- add schema columns
- add `APICredentialService`
- classify bootstrap-created keys
- expand audit metadata

### Slice B: Workspace browser-admin API and UI

- workspace credential list/create/rotate/revoke
- workspace access page integration
- audit entry point

### Slice C: Platform credential admin

- platform credential list/create/revoke
- bootstrap history visibility
- break-glass issuance API and UI

### Slice D: Governance hardening

- mandatory expiry for platform credentials
- optional expiry defaults for workspace credentials
- environment labeling and stricter validation

### Slice E: Service accounts

- first-class non-human principals
- credentials attached to service accounts

---

## Testing Requirements

### Backend

Add coverage for:
- workspace admin can create workspace credential through browser-admin path
- writer/reader cannot create workspace credential
- platform admin can bootstrap workspace key
- platform admin can issue break-glass key only with reason + expiry
- workspace credential remains tenant-scoped
- platform credential remains platform-scoped
- one-time secret reveal behavior still holds
- audit metadata includes owner classification and browser actor context

### Web

Add E2E/session coverage for:
- workspace admin creates credential and copies one-time secret
- workspace admin rotates credential
- workspace admin revokes credential
- platform admin issues bootstrap credential
- platform admin issues break-glass credential with warnings

---

## Recommended Current Answer

Implement this as:
- browser-user-owned workspace credential management
- compatibility-preserving runtime API
- explicit bootstrap and break-glass exceptions

Do not implement this as:
- removing platform capability entirely
- keeping tenant API keys as the primary human auth model
- inventing a brand new auth system before using the current workspace access foundation

That would be the wrong order.

---

## Bottom Line

The next correct implementation step is not to delete `/v1/api-keys`.
It is to put a proper enterprise ownership layer on top of it.

That means:
- workspace admins own normal workspace machine credentials
- platform retains explicit exceptional powers
- browser access remains membership-based
- API keys begin evolving toward service-account-backed credentials

This gives Alpha the right long-term enterprise posture without breaking the current machine integration surface.

## Migration note: service_accounts

`0032_service_accounts` introduces the first-class machine identity layer.
It does not replace `api_keys` yet.

Current migration intent:
- create a durable workspace-scoped machine identity record in `service_accounts`
- keep credential secrets in the existing `api_keys` table
- bind credentials to machine identities with:
  - `owner_type=service_account`
  - `owner_id=<service_account_id>`

This is intentionally incremental:
- runtime API authentication continues to validate `api_keys`
- browser admins now manage machine identity separately from individual secret rows
- a later slice can enforce service-account status in runtime auth once service-account lifecycle controls are expanded
