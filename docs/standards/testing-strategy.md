# Testing Strategy

This document defines Alpha's long-term testing layers.

The goal is not "more tests" in the abstract. The goal is to make it obvious:

- which tests are fast and mocked
- which tests use a real local stack
- which tests touch live staging or providers
- which commands should be used for each layer

If a test command does not clearly belong to one of these layers, the structure is wrong.

---

## Core Rule

Alpha should use a layered test model:

1. mocked and deterministic by default
2. real local integration for backend boundary confidence
3. narrow live staging smoke for release confidence

Do not use live staging or live provider flows as the default validation path for normal development.

---

## Layer 1: Mocked and Deterministic

Use this layer for most daily development and PR work.

### Scope

- unit tests
- service logic tests
- API contract tests that do not need live external systems
- browser workflow tests that mock UI session and API responses

### Purpose

- validate product behavior quickly
- keep feedback loops short
- make failures easy to diagnose

### Commands

```bash
make test-unit
make web-e2e
make test-browser-mocked
```

### Examples in this repo

- `internal/domain`
- many focused `internal/service` tests
- mocked/session Playwright specs under `web/tests/e2e/*-session.spec.ts`

### What does not belong here

- real Stripe payment collection
- live staging browser smoke
- cloud/runtime verification

---

## Layer 2: Local or Controlled Integration

Use this layer for backend workflow correctness with a real local stack.

### Scope

- real Postgres
- real Alpha API/service code
- real Lago and Temporal in local or controlled env
- deterministic fixtures

### Purpose

- validate orchestration and persistence boundaries
- catch schema, migration, projection, and workflow regressions
- avoid depending on live staging state

### Commands

```bash
make test-real-env-smoke
make test-integration
make test-smoke-local
make test-integration-local
```

### Examples in this repo

- replay and reconciliation correctness
- Lago-backed API visibility flows
- workflow convergence across Alpha + Lago + Temporal + Postgres

### What does not belong here

- browser auth against live staging
- release sign-off on live provider behavior

---

## Layer 3: Live Staging Smoke

Use this layer only for release readiness, operator verification, or controlled staging checks.

### Scope

- deployed staging runtime
- real browser sessions
- real staging data or prepared fixtures
- real provider-side behavior where needed

### Purpose

- validate environment wiring
- validate live browser surfaces
- validate real payment/replay/provider integration only where mocks are insufficient

### Commands

```bash
make test-browser-staging-smoke
make verify-staging-runtime
make test-staging-payment-smoke
make test-staging-replay-smoke
make test-staging-acceptance
```

### Current staging-specific helpers

- `make bootstrap-live-e2e-browser-users-cluster`
- `make bootstrap-staging-payment-fixtures`
- `make mint-live-e2e-keys-cluster`
- `make cleanup-staging-flow-data`

### Important boundary

- browser smoke should use browser users and UI sessions
- machine/API verification may still use API credentials where appropriate

Do not test browser login with API keys.

---

## Ownership by Layer

### Mocked

Use for:

- UI composition
- auth routing
- tenant/platform RBAC presentation
- payment/recovery guidance
- customer payment-setup flow UX

### Local integration

Use for:

- migrations
- RLS and persistence behavior
- replay workflow correctness
- Lago-backed API contracts
- Alpha orchestration over real local services

### Live staging smoke

Use for:

- deploy/runtime health
- live browser smoke on core surfaces
- real payment success/failure verification
- replay smoke against real staging runtime

---

## Naming Rules

Make targets and scripts should signal their layer clearly.

Preferred prefixes:

- `test-*` for mocked or local integration
- `verify-*` for environment/runtime checks
- `bootstrap-*` for fixture or identity preparation
- `cleanup-*` for explicit data reset tools

Required distinctions:

- `test-browser-mocked`
- `test-integration-local`
- `test-browser-staging-smoke`
- `test-staging-payment-smoke`

Avoid generic names that hide whether a flow is mocked or live.

---

## Staging Data Hygiene

Do not wipe staging blindly.

Staging cleanup must:

- default to dry-run
- target generated fixtures by reserved prefixes
- avoid deleting core tenant/workspace/product state unless explicitly intended

Current cleanup entrypoint:

```bash
make cleanup-staging-flow-data DATABASE_URL='...'
```

Apply only with:

```bash
APPLY=1 CONFIRM_STAGING_FLOW_CLEANUP=YES_I_UNDERSTAND
```

Payment fixture bootstrap is a separate first-class operation:

```bash
make bootstrap-staging-payment-fixtures
```

Do not hide fixture creation inside cleanup or vice versa.

---

## Long-Term Seams

These seams are intentionally first-class because they have been the recurring live-test failure points.

### Browser Auth Helper

- browser smoke should verify authenticated UI session state
- do not key success on one specific menu control being visible
- current source of truth is `/v1/ui/sessions/me`

### Payment Fixture Bootstrap

- payment fixture creation should run in a dedicated Lago bootstrap job
- do not `kubectl exec` a Rails runner in the live Lago pod as the normal path
- fixture ids must be per-run by default

### Cleanup

- cleanup must remain a separate command
- cleanup must target both current per-run fixtures and any known legacy fixed-id fixtures
- cleanup should never be the only way to understand whether bootstrap is healthy

---

## Current Source of Truth

Use this file as the durable policy for test-layer boundaries.

Operational details stay in:

- [Real Payment E2E Runbook](../runbooks/real-payment-e2e-runbook.md)
- [Replay Recovery Live Runbook](../runbooks/replay-recovery-live-runbook.md)
- [Staging Go Live Checklist](../checklists/staging-go-live-checklist.md)

The standards doc defines the layers.
The runbooks define how to execute a specific live path.
