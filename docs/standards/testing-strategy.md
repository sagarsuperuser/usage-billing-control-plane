# Testing Strategy

Three layers: fast mocked, real local, live staging.

## Layer 1 — Mocked and deterministic

Daily development and PR validation.

- Unit tests for service logic and domain rules
- API contract tests with real Postgres (gated by `TEST_DATABASE_URL`)
- Browser E2E tests with mocked sessions and API responses (Playwright)

```bash
make test-unit
make web-e2e
```

Does not include: real Stripe, live staging, cloud runtime.

## Layer 2 — Local integration

Backend correctness against a real local stack.

- Real Postgres, real Temporal, real Alpha API
- Validates: migrations, RLS, replay workflows, billing cycle, API contracts

```bash
make test-real-env-smoke
make test-integration
```

Does not include: browser auth against live staging, release sign-off.

## Layer 3 — Live staging smoke

Release readiness and operator verification.

- Deployed staging runtime, real browser sessions, real Stripe behavior

```bash
make verify-staging-runtime
make test-browser-staging-smoke
```

## Naming rules

Make targets signal their layer:

- `test-*` — mocked or local integration
- `verify-*` — environment/runtime checks
- `bootstrap-*` — fixture or identity preparation
- `cleanup-*` — explicit data reset

## Staging data hygiene

Never wipe staging blindly. Cleanup must:
- Default to dry-run
- Target only fixtures by reserved prefixes
- Never delete core tenant/workspace/product state without explicit intent

## Release confidence set

Minimum (unified pipeline — `.github/workflows/pipeline.yml`):
1. Stage 1: Go tests + web lint/build/typecheck + migration verify + Helm lint
2. Stage 2: Playwright E2E + integration smoke/full + Docker builds
3. Stage 3: Helm upgrade to staging (only after all tests + builds pass)
