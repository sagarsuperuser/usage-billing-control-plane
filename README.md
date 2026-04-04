# Alpha — Usage Billing Control Plane

Multi-tenant usage-based billing platform with direct Stripe integration. Owns the full billing lifecycle: pricing catalog, customer onboarding, subscription management, usage metering, invoice generation, payment execution, dunning automation, and operator tooling.

## Architecture

- **Go API** (chi/v5) — 75+ routes, middleware groups, RLS per tenant
- **Vite + TanStack Router UI** — operator dashboard with sidebar navigation
- **Temporal** — billing cycle, replay, dunning, payment reconciliation
- **Stripe** — payment execution only (PaymentIntents, no Stripe Billing fees)
- **PostgreSQL** — system of record with row-level security
- **AWS** — EKS, Secrets Manager, S3, Terraform

See [docs/architecture.md](docs/architecture.md) for full system topology.

## Requirements

- Go 1.25+
- PostgreSQL 16+
- Node.js 20+
- Temporal server
- Redis (for rate limiting)

## Quick start

```bash
# Start local infrastructure
docker compose -f docker-compose.postgres.yml up -d

# Run migrations
DATABASE_URL='postgres://postgres:postgres@localhost:15432/lago_alpha?sslmode=disable' \
  go run ./cmd/migrate

# Start API server
DATABASE_URL='postgres://postgres:postgres@localhost:15432/lago_alpha?sslmode=disable' \
  go run ./cmd/server

# Start web UI
cd web && pnpm install && pnpm dev
```

## Project structure

```
cmd/
  server/          — API server entry point
  migrate/         — Database migration runner
  admin/           — Admin CLI tools
internal/
  api/             — HTTP handlers (chi/v5), middleware, error classification
  service/         — Business logic, billing engine, Stripe adapters
  store/           — PostgreSQL queries (per-entity files, RLS)
  domain/          — Domain model structs + pricing engine
  billingcycle/    — Billing cycle Temporal workflow
  dunningflow/     — Dunning Temporal workflow
  replay/          — Usage replay Temporal workflow
  paymentsync/     — Payment reconciliation Temporal workflow
  appconfig/       — Environment-based configuration
web/               — Vite + TanStack Router operator UI
deploy/
  helm/            — Helm chart + environment values
  terraform/       — AWS infrastructure (EKS, RDS, S3)
migrations/        — SQL migration files (48+)
```

## Testing

```bash
# Unit tests
go test ./... -short

# E2E browser tests
cd web && pnpm e2e

# Integration tests (requires Docker)
make test-real-env-smoke
```

## Deployment

Push to `main` triggers three GitHub Actions workflows:
1. **CI Fast** — Go build + lint + unit tests + Playwright E2E
2. **CI Deep** — integration tests against real Postgres + Temporal
3. **Deploy Staging** — Docker build → ECR → Helm upgrade to EKS

## Documentation

- [Architecture](docs/architecture.md)
- [Engineering standards](docs/standards/engineering-standards.md)
- [Testing strategy](docs/standards/testing-strategy.md)
- [Infra rollout runbook](docs/runbooks/infra-rollout-runbook.md)

## Key design decisions

- **No Stripe Billing** — own the billing engine, pay Stripe only for payment processing (2.9% + 30c). Same pattern as Orb/Metronome.
- **Row-level security** — every tenant query goes through PostgreSQL RLS. Tenant A cannot read tenant B's data even with application bugs.
- **Idempotent billing** — `UNIQUE (tenant_id, subscription_id, billing_period_start)` prevents double-billing.
- **Exactly-once payments** — one Stripe PaymentIntent per invoice, confirmed with `off_session=true`.
- **Structured error codes** — `DomainError` type with machine-readable codes. No string-matching in error classification.
- **chi/v5 middleware groups** — auth enforced structurally per route group, not per handler.
