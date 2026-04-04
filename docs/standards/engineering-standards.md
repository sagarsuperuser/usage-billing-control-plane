# Engineering Standards

Go-idiomatic, production-grade standard for this codebase.

---

## Layer Model

| Layer | Package | Owns |
|-------|---------|------|
| Domain | `internal/domain` | Stable product concepts: tenant, customer, subscription, invoice, etc. Not transport DTOs. |
| Service | `internal/service` | Business workflows, orchestration, external-system adapters |
| Store | `internal/store` | SQL, row mapping, transactions, persistence filters |
| Transport | `internal/api` | Routes, auth extraction, request/response DTOs, HTTP status mapping |

Rules:
- Handlers call services, not stores directly.
- Services may depend on stores and adapters.
- Services must not depend on HTTP concerns.
- SQL shape stays in the store layer.

---

## External Adapters

All external systems (Stripe, SMTP, OAuth) are wrapped behind adapter interfaces.

- Interface at the consumer boundary
- Adapter struct implements it
- Tenant-aware wrapper delegates with resolved transport

This keeps coupling explicit and the external provider replaceable.

---

## Interface Rules

Define interfaces at the consumer, not pre-emptively everywhere.

Good reasons: persistence boundary, mail sender, billing engine, test seam.

Bad reasons: one interface per struct by default, abstracting code with one obvious implementation.

---

## Error Handling

1. Use sentinel or typed errors where behavior differs (`ErrValidation`, `ErrNotFound`, etc.)
2. Map errors to HTTP behavior centrally, not per-handler.
3. Include `request_id` in error responses.
4. Never leak raw internal details to end users.
5. Wrap errors with enough context to identify the failing workflow stage.

---

## Observability

Every important workflow must support:
- Structured logs with `request_id` correlation
- Stage-aware error context for multi-step flows
- Metrics where failure rate matters (payment, replay, onboarding)

If a workflow can fail in production and create user-visible impact, its failure mode must be diagnosable without local reproduction.

---

## API Design

API responses must reflect Alpha concepts, not engine internals.

- Use explicit product DTOs
- Stable response envelopes
- Clear error codes
- `json:"-"` on any field that is provider-internal (Stripe account IDs, secret refs)

Never expose transport contracts that mirror storage layout or backend engine fields.

---

## Testing

- Focused unit tests for workflow rules
- Repository tests for persistence behavior
- API tests for request/response contracts and auth behavior
- Playwright for meaningful browser workflows

A meaningful workflow change ships with at least one of: service test, API contract test, or browser test.

---

## What To Avoid

- Fat handlers with business logic
- God services that own unrelated workflows
- Interface-per-struct abstraction noise
- Raw backend payload leakage into product APIs
- Stringly typed error control flow as a permanent pattern
- Silent fallback paths that hide the real source of truth
- Docs that compete with existing source-of-truth documents
