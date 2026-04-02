# Docs

## Start Here

- [Architecture](./architecture.md) — system topology, workloads, Lago boundary, scaling
- [Engineering Standards](./standards/engineering-standards.md) — Go patterns, structure, what belongs where
- [Testing Strategy](./standards/testing-strategy.md) — test layers, when to use each

## Runbooks

Operational procedures for running and deploying the product.

- [Manual End-to-End Validation](./runbooks/manual-end-to-end-validation-runbook.md) — pre-release UI validation
- [End-to-End Product Journeys](./runbooks/end-to-end-product-journeys.md) — canonical automated flow set
- [Real Payment E2E](./runbooks/real-payment-e2e-runbook.md) — live payment flow validation
- [Replay Recovery](./runbooks/replay-recovery-live-runbook.md) — recovering from replay failures
- [Assisted Tenant Onboarding](./runbooks/assisted-tenant-onboarding-runbook.md) — onboarding a new tenant
- [Infra Rollout](./runbooks/infra-rollout-runbook.md) — Terraform + Helm deployment
- [Lago Staging Bootstrap](./runbooks/lago-staging-bootstrap.md) — first-time Lago staging setup
- [Temporal Staging Bootstrap](./runbooks/temporal-staging-bootstrap.md) — first-time Temporal setup
- [Cloudflare + Lago Admin Setup](./runbooks/cloudflare-lago-admin-setup.md) — DNS and admin bootstrap

## Checklists

- [Staging Go-Live Checklist](./checklists/staging-go-live-checklist.md) — release gate for staging

## Reference

- [Lago Baseline](./standards/lago-baseline.md) — which Lago version and images are in use
