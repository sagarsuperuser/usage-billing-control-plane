# Engineering Standards

This document defines the default engineering standard for Alpha.

The goal is to keep the codebase production-grade, understandable, and maintainable as the product grows.

This is a Go-idiomatic standard.
It is not a textbook inheritance-heavy OOP standard.

---

## Purpose

Use this document when:

- shaping new packages or services
- refactoring product workflows
- deciding where logic should live
- adding new external integrations
- reviewing whether a change is maintainable long term

---

## Core Principles

### 1. Keep product concepts explicit

Do not overload one object with multiple responsibilities.

Prefer explicit domain separation, for example:

- billing connection vs workspace billing binding
- browser user vs workspace membership
- service account vs credential
- Alpha notification ownership vs Lago delivery execution

If two concepts have different ownership, lifecycle, or operational meaning, model them separately.

### 2. Prefer composition over inheritance

Alpha should follow normal Go design:

- small focused structs
- composition
- interfaces at real boundaries
- constructor-based dependency wiring

Do not introduce inheritance-style hierarchies or abstract base patterns just to simulate classical OOP.

### 3. Keep transport thin

HTTP handlers should primarily do four things:

- parse and validate request input
- resolve auth/session context
- call service-layer workflows
- map results and errors to API responses

Handlers should not become the main home for business logic.

### 4. Keep workflows in services

Business rules and multi-step orchestration belong in services.

Examples:

- onboarding
- workspace billing binding resolution
- invitation acceptance
- password reset
- service-account credential lifecycle

A service should own one coherent workflow boundary, not every operation in the system.

### 5. Keep persistence in store/repository code

Database-specific logic belongs in the store layer.

Examples:

- SQL shape
- scans and row mapping
- transaction boundaries where persistence needs them
- storage-specific filtering

Do not leak SQL concerns into handlers or unrelated services.

---

## Package and Layer Rules

Alpha should keep the following package posture unless there is a strong reason not to.

### `internal/domain`

Use for stable product concepts and domain structs.

Examples:

- tenant
- workspace billing binding
- service account
- invoice view

Do not fill `domain` with transport-only request/response structs.

### `internal/service`

Use for business workflows, orchestration, and external-system adapters.

Examples:

- `TenantOnboardingService`
- `WorkspaceAccessService`
- `ServiceAccountService`
- `PasswordResetService`

A service may depend on stores and external clients.
A service should not depend on HTTP response-writing concerns.

### `internal/store`

Use for persistence access.

Responsibilities:

- queries and commands
- persistence DTO mapping
- transactions
- storage-specific filters and constraints

The store layer should not encode product presentation logic.

### `internal/api`

Use for HTTP transport.

Responsibilities:

- routes
- auth/session extraction
- request/response DTOs
- HTTP status mapping
- structured request-scoped logging

Do not let `api` become a second service layer.

### External adapters

External systems should be wrapped behind focused service or adapter seams.

Examples:

- Lago
- SMTP
- OAuth / browser SSO
- future notification delivery backends

The goal is not abstraction for its own sake.
The goal is to keep external coupling explicit and replaceable.

---

## Interface Rules

Use interfaces when they create real value.

Good reasons:

- persistence boundary
- mail sender boundary
- billing engine adapter
- auth provider boundary
- test seam around an external dependency

Bad reasons:

- creating an interface for every struct by default
- abstracting code that has only one obvious implementation and no seam value

Rule:
- define interfaces at the consumer boundary where possible, not pre-emptively everywhere.

---

## Error Handling Rules

Errors are part of the design, not an afterthought.

### Requirements

1. Use explicit typed or sentinel errors where behavior differs.
2. Map domain/store/service errors centrally to HTTP behavior.
3. Include `request_id` in API error responses when available.
4. Do not leak raw internal details to end users by default.
5. Wrap internal errors with enough context to identify the failing workflow stage.

Examples of acceptable classification:

- validation
- not found
- already exists
- forbidden
- dependency failure
- internal unexpected error

Avoid string-matching control flow unless working through a temporary compatibility gap.

---

## Observability Rules

Operational visibility is required for critical workflows.

### Every important workflow should support

- structured logs
- request correlation via `request_id`
- stage-aware error context for multi-step flows
- metrics where failure rate matters

High-value workflows include:

- onboarding
- billing connection sync
- invite acceptance
- password reset
- service-account lifecycle
- invoice/payment operational actions

If a workflow can fail in production and create user-visible impact, its failure mode should be diagnosable without local reproduction.

---

## Testing Rules

Alpha should keep a layered test strategy.

### Service/store/API

Prefer:

- focused unit tests for workflow rules
- repository/store tests for persistence behavior
- API tests for request/response contracts and auth behavior

### Web

Prefer:

- typecheck and lint always
- Playwright for meaningful user workflows
- stable browser harnesses over brittle route-mock magic

### Change-level rule

A meaningful workflow change should usually ship with at least one of:

- new service test
- API contract test
- browser workflow test

---

## Refactoring and Migration Rules

Alpha is evolving while remaining deployable.

Rules:

1. Prefer migration-safe refactors over destructive rewrites.
2. Preserve compatibility intentionally when a runtime contract is still in use.
3. Move product/admin surfaces to the new model before deleting the old substrate.
4. Remove fallbacks once the new source of truth is real, not before.
5. Do not keep “temporary” compatibility paths alive indefinitely.

This is especially important for:

- billing bindings vs legacy tenant fields
- service accounts vs raw API-key admin posture
- Alpha-owned DTOs vs raw backend-engine payloads

---

## API Design Rules

Browser-facing and workspace-facing APIs should reflect Alpha concepts, not raw engine internals.

Prefer:

- explicit product DTOs
- stable response envelopes
- clear error codes
- subresources that match product ownership

Avoid:

- leaking backend implementation fields into normal product APIs
- forcing UI code to reconstruct product meaning from low-level fields
- exposing transport contracts that mirror storage layout unnecessarily

---

## UI and Product Boundary Rules

The UI should expose Alpha-native concepts.

Examples:

- billing connection
- workspace billing
- service account
- workspace access
- invoice

Do not expose raw backend vocabulary unless the screen is explicitly advanced/operator-facing.

A good product boundary hides engine complexity while preserving operational truth.

---

## Documentation Rules

Durable architecture and workflow decisions should be documented.

When landing a meaningful architecture or product-boundary change:

1. update or add the relevant model/spec doc
2. update [Docs Index](./README.md) if the doc is durable
3. update the roadmap if sequencing or priority changed
4. update runbooks if operator behavior changed

Do not rely on commit history as the only durable explanation.

---

## What To Avoid

Avoid these patterns unless there is a strong, explicit reason.

- inheritance-style architecture
- god services
- fat handlers
- interface-per-struct abstraction noise
- raw backend payload leakage into product UI
- stringly typed error behavior as a permanent design
- silent fallback paths that hide the real source of truth
- docs that compete with an existing source-of-truth document

---

## Decision Summary

Alpha should follow a production-grade, Go-idiomatic architecture standard:

- explicit domain boundaries
- thin transport
- real service workflows
- focused persistence layer
- interfaces only at real seams
- strong error handling and observability
- migration-safe refactoring
- Alpha-owned product contracts and terminology

That is the maintainable long-term standard for this codebase.
