# Testing Strategy

Three layers: fast mocked, real local, live staging.

---

## Layer 1 — Mocked and Deterministic

Daily development and PR validation.

- Unit tests, service logic, API contract tests
- Browser tests with mocked sessions and API responses

```bash
make test-unit
make web-e2e
make test-browser-mocked
```

Does not include: real Stripe, live staging browser, cloud runtime verification.

---

## Layer 2 — Local Integration

Backend correctness against a real local stack.

- Real Postgres, real Alpha API, real Lago and Temporal in local/controlled env
- Validates: migrations, RLS, replay workflows, Lago-backed API contracts

```bash
make test-real-env-smoke
make test-integration
make test-smoke-local
make test-integration-local
```

Does not include: browser auth against live staging, release sign-off.

---

## Layer 3 — Live Staging Smoke

Release readiness and operator verification only.

- Deployed staging runtime, real browser sessions, real provider-side behavior

```bash
make test-browser-staging-smoke
make verify-staging-runtime
make test-staging-payment-smoke
make test-staging-replay-smoke
make test-staging-acceptance
```

Staging helpers:
```bash
make bootstrap-live-e2e-browser-users-cluster
make bootstrap-staging-payment-fixtures
make mint-live-e2e-keys-cluster
make cleanup-staging-flow-data
```

Important: use browser sessions for browser smoke. Do not test browser login with API keys.

---

## Naming Rules

Make targets must signal their layer:

- `test-*` — mocked or local integration
- `verify-*` — environment/runtime checks
- `bootstrap-*` — fixture or identity preparation
- `cleanup-*` — explicit data reset

---

## Staging Data Hygiene

Never wipe staging blindly. Cleanup must:
- Default to dry-run
- Target only fixtures by reserved prefixes
- Never delete core tenant/workspace/product state without explicit intent

```bash
# dry-run
make cleanup-staging-flow-data DATABASE_URL='...'

# apply
APPLY=1 CONFIRM_STAGING_FLOW_CLEANUP=YES_I_UNDERSTAND \
make cleanup-staging-flow-data DATABASE_URL='...'
```

Payment fixture bootstrap is a separate operation:
```bash
make bootstrap-staging-payment-fixtures
```

---

## Release Confidence Set

Minimum:
1. `make verify-staging-runtime`
2. `make test-staging-payment-smoke LAGO_API_KEY='...'`
3. `make test-staging-replay-smoke`
4. `make test-browser-staging-smoke`

Extended (add for significant releases):
1. `make test-staging-pricing-journey`
2. `make test-staging-subscription-journey`
3. `make test-staging-payment-setup-journey`

See [End-to-End Product Journeys](../runbooks/end-to-end-product-journeys.md) for the full canonical journey set.
