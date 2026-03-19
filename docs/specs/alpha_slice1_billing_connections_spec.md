# Alpha Slice 1 Spec: Billing Connections Hardening

This document defines the first Wave 1 implementation slice for Alpha: hardening the Billing Connections surface so Alpha clearly owns billing connectivity.

Read together with:

- [Alpha Import Goal](../goals/alpha_import_goal.md)
- [Alpha Import Matrix](../goals/alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](../roadmaps/alpha_wave1_roadmap.md)

---

## Objective

Alpha must own billing connectivity as a first-class product capability.

For Wave 1, that means:

- a platform admin can create and inspect billing connections in Alpha
- workspace setup only references Alpha-owned billing connections
- connection status and sync health are visible in Alpha
- users do not need Lago UI to understand basic provider state

This slice is focused on the Stripe-first path.

---

## Product Scope

### In scope

- Billing Connections list
- Billing Connection create flow
- Billing Connection detail view
- sync status visibility
- last sync result visibility
- workspace assignment via `billing_provider_connection_id`
- failure-state visibility and recovery guidance

### Out of scope

- full multi-provider breadth
- secret rotation UX beyond clear limitation handling
- billing-entity-level assignment logic
- tenant-side billing connection management

---

## User Stories

1. A platform admin can create a Stripe billing connection in Alpha.
2. A platform admin can see whether the connection is `pending`, `connected`, `sync_error`, or `disabled`.
3. A platform admin can inspect the latest sync status without opening Lago.
4. A platform admin can assign a billing connection to a workspace during setup.
5. A platform admin can understand what failed and what to do next if sync fails.

---

## Product Rules

1. The UI must use Alpha language, not Lago plumbing language.
2. The primary concept is `Billing Connection`, not provider-code wiring.
3. Secret material must never be re-exposed after create.
4. Advanced engine details can exist on detail pages, but should remain secondary.
5. Workspace setup should select from Alpha-owned billing connections, not raw mapping fields.

---

## Target Product Surface

### Routes

- `/billing-connections`
- `/billing-connections/new`
- `/billing-connections/[id]`

### Navigation placement

- top-level platform nav item

### Primary actions

- create billing connection
- sync billing connection
- inspect billing connection detail

### Secondary actions

- disable billing connection
- inspect advanced connection metadata

---

## Backend Scope

### Domain model

Use the existing `billing_provider_connections` model and strengthen it.

The connection record should clearly represent:

- provider type
- display name
- environment if applicable
- lifecycle status
- secret reference
- linked execution identifiers
- latest sync timestamps/results
- latest sync error summary

### Required backend work

1. Finalize connection status model
- `pending`
- `connected`
- `sync_error`
- `disabled`

2. Normalize sync result capture
- store last sync timestamp
- store last sync outcome
- store last sync error summary in a user-safe form

3. Harden sync behavior
- explicit sync action
- clear service boundary around provider sync
- deterministic update of status after sync attempt

4. Clarify secret-handling boundary
- secret stored outside DB
- DB stores only secret reference
- API never re-returns the provider secret

5. Workspace assignment stability
- ensure workspace setup persists `billing_provider_connection_id`
- ensure workspace reads/detail views resolve and display the assigned connection correctly

### Suggested APIs

Keep or complete the following platform-only APIs:

- `POST /internal/billing-provider-connections`
- `GET /internal/billing-provider-connections`
- `GET /internal/billing-provider-connections/{id}`
- `PATCH /internal/billing-provider-connections/{id}`
- `POST /internal/billing-provider-connections/{id}/sync`
- `POST /internal/billing-provider-connections/{id}/disable`

### API response expectations

Billing connection responses should return:

- stable Alpha connection identifiers
- user-safe status
- provider type
- display name
- created/updated timestamps
- last sync timestamp
- last sync outcome summary
- last sync error summary if present
- linked workspace count if cheap to compute

Do not return:

- provider secret
- raw secret-store contents

---

## UI Scope

### Billing Connections list

Must show:

- connection name
- provider type
- status
- last sync state
- quick health/readiness signal

Should not show:

- dense engine metadata
- raw secret references

### Billing Connection create

Must support:

- provider type selection for supported providers
- display name
- required provider credential entry
- clear post-create state

Should explain:

- what the connection is for
- that Alpha will use it for workspace billing

### Billing Connection detail

Must show:

- status
- latest sync result
- latest sync time
- provider type
- display name
- linked usage in Alpha terms

Should include:

- `Sync now`
- `Disable`
- guidance when in `sync_error`

Advanced section may include:

- backend/provider identifiers
- internal metadata

---

## UX Notes

This surface should feel like a product setup flow, not a billing-engine admin page.

Use:

- clear status chips
- one primary action
- concise guidance
- non-threatening copy

Avoid:

- raw engine jargon
- too many inline actions
- dense technical cards above the fold

---

## Testing Requirements

### Backend

- service tests for create/sync/disable flows
- API tests for:
  - platform-only access
  - validation failures
  - sync success/failure transitions
  - secret non-exposure

### UI

- platform session route coverage
- list/create/detail flow coverage
- status rendering coverage
- sync-error empty-state/recovery coverage
- workspace assignment integration coverage

### Staging

- create a billing connection
- sync it
- assign it to a workspace
- verify workspace detail reflects the assignment

---

## Exit Criteria

This slice is complete when:

1. Alpha owns the end-to-end product flow for billing connection creation and inspection.
2. Workspace setup uses only Alpha billing connection concepts.
3. A failed sync can be understood from Alpha without falling back to Lago UI.
4. The surface feels simple, clear, and platform-admin focused.
