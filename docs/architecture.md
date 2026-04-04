# Architecture

## System model

Alpha is a multi-tenant usage-based billing control plane. It owns the full billing lifecycle: pricing catalog, customer onboarding, subscription management, usage metering, invoice generation, payment execution via Stripe, dunning automation, and operator tooling.

```
Operators / Customers
        |
        v
  Alpha Control Plane
  (auth, tenancy, pricing, invoicing, dunning, audit)
        |
        v
  Stripe (payment execution only — PaymentIntents, Checkout Sessions, refunds)
```

Stripe handles payment processing (2.9% + 30c). Alpha owns everything else.

## Workloads

| Workload | Type | Responsibility |
|----------|------|----------------|
| `alpha-api` | Stateless HTTP (chi/v5) | Tenant auth/RBAC, all API endpoints, webhook ingestion |
| `alpha-replay-worker` | Temporal worker | Replay job activities — re-rates historical usage |
| `alpha-replay-dispatcher` | Temporal starter | Polls queued replay jobs, starts Temporal workflows |
| `alpha-billing-cycle-worker` | Temporal worker | Invoice generation + Stripe PaymentIntent creation |
| `alpha-billing-cycle-scheduler` | Temporal cron | Schedules billing cycle runs (@every 5m) |
| `alpha-payment-reconcile` | Temporal cron | Backfills stale payment status from Stripe |
| `alpha-dunning-worker` | Temporal worker | Payment retry + collection reminder campaigns |
| `alpha-dunning-scheduler` | Temporal cron | Schedules dunning batch runs (@every 2m) |
| `alpha-billing-check` | Temporal cron | Periodic Stripe connection health verification |
| `alpha-web` | Next.js | Operator UI |

## Dependencies

| Dependency | Role |
|------------|------|
| PostgreSQL (RDS) | System of record — all billing data, RLS per tenant |
| Temporal | Durable orchestration for billing cycle, replay, dunning |
| Stripe | Payment execution via PaymentIntents (no Stripe Billing) |
| S3 | Audit export storage, invoice PDF storage |
| AWS Secrets Manager | Stripe API keys per tenant (via External Secrets) |
| Redis | Distributed rate limiting |

## Ownership boundary

### Alpha owns
- Multi-tenant identity and workspace isolation (RLS)
- Pricing catalog (metrics, plans, add-ons, coupons, taxes)
- Customer onboarding and billing profile management
- Subscription lifecycle and billing cycle scheduling
- Usage event ingestion, aggregation, and rating (ComputeAmountCents)
- Invoice generation (line items, taxes, discounts, totals)
- Invoice finalization (Stripe PaymentIntent creation)
- Payment status tracking (from Stripe webhooks)
- Dunning automation (retry scheduling, payment reminders, escalation)
- Replay/re-rating pipeline
- Reconciliation engine (usage vs billed entries)
- Audit trails and credential management
- Operator UI

### Stripe owns
- Payment execution (charging cards, bank transfers)
- Payment method collection (Checkout Sessions)
- Refund execution
- Webhook delivery

## Adapter boundary

All Stripe access goes through `internal/service/stripe_client.go` — a thin wrapper around `stripe-go/v82` with per-request API keys for multi-tenant isolation.

Billing adapters (defined in `internal/service/adapter_interfaces.go`):
- `MeterSyncAdapter` — no-op (meters are local-only)
- `PlanSyncAdapter` — no-op (plans are local-only)
- `SubscriptionSyncAdapter` — initializes billing cycle fields
- `CustomerBillingAdapter` — Stripe Customer CRUD + Checkout Sessions
- `InvoiceBillingAdapter` — local invoice reads + PaymentIntent retries
- `BillingProviderAdapter` — Stripe key verification

## Scaling

- API: HPA on CPU/memory (2-20 replicas)
- Billing cycle worker: scale on subscription count
- Replay worker: scale on queue depth
- Web: independently deployable, autoscaled

## Reliability

- Multi-AZ EKS + Multi-AZ RDS with automated backups
- Idempotent billing cycles (UNIQUE constraint on subscription + period)
- Exactly-once payment execution (one PaymentIntent per invoice)
- Temporal workflow IDs for deterministic deduplication
- Row-level security per tenant in PostgreSQL

## Deployment flow

1. Push to `main` triggers CI (fast + deep) and staging deploy
2. CI Fast: Go build + lint + unit tests + E2E browser tests
3. CI Deep: integration tests against real Postgres + Temporal
4. Deploy: Docker build → ECR push → Helm upgrade with immutable tags
5. Post-deploy: `/health` endpoint, Temporal worker poll active
