# Architecture

## System Model

**Alpha** is the product surface, control plane, and orchestration layer.  
**Lago** is the billing execution engine behind the scenes.

Operators and customers enter through Alpha. Normal workflows never require direct Lago access.

```
Operators / Customers
        │
        ▼
  Alpha Control Plane  (auth, tenancy, replay, audit, orchestration)
        │  HTTP
        ▼
    Lago API  (invoice generation, payment execution, billing engine)
        │
   Lago Events Processor  (Kafka, high-throughput metering)
```

---

## Workloads

| Workload | Type | Responsibility |
|----------|------|----------------|
| `alpha-api` | Stateless HTTP | Tenant auth/RBAC, Lago proxy, replay/reconcile APIs, webhook ingestion |
| `alpha-replay-worker` | Temporal worker | Replay job activities — scales on queue throughput |
| `alpha-replay-dispatcher` | Temporal starter | Polls queued jobs in Postgres, starts Temporal workflows |
| `alpha-payment-reconcile` | Temporal cron (optional) | Backfills stale payment projections from Lago |
| `alpha-dunning-worker` | Temporal worker | Payment reminder campaigns |
| `alpha-web` | Next.js | Operator UI — payment ops, invoice explainability, control plane |

---

## Dependencies

| Dependency | Role |
|------------|------|
| Postgres (RDS) | System of record — tenants, customers, replay jobs, API keys, audit |
| Temporal | Durable orchestration for replay, dunning, payment reconciliation |
| S3 | Audit export and replay artifact storage |
| AWS Secrets Manager + ESO | Runtime config/secret distribution to workloads |
| Redis | Distributed rate limiting |
| Lago API | Billing engine — invoices, payments, subscriptions, billable metrics |

---

## Ownership Boundary

### Alpha owns
- Platform identities and operator access
- Tenant lifecycle and onboarding state
- Pricing intent and product-side configuration
- Customer onboarding and billing profile readiness
- Replay, reconciliation, and explainability workflows
- Audit trails
- Payment status projections (read model from Lago webhooks)

### Lago owns
- Invoice generation and execution state
- Payment execution state
- Provider-facing billing details (Stripe integration)
- Billing engine internals

### Allowed duplication
- Alpha projections for fast reads (payment status, invoice summaries)
- External ID mappings between Alpha and Lago records
- Alpha-authored config synced into Lago (meters, plans, customers)

### Not allowed
- Two canonical sources of truth for invoice or payment execution
- Alpha becoming a second billing engine
- Operator workflows that require understanding Lago internals

---

## Adapter Boundary

All Lago access goes through adapter interfaces in `internal/service/lago_client.go`. No service outside the adapter layer calls Lago directly.

Key adapters:
- `MeterSyncAdapter` — syncs meters to Lago billable metrics
- `PlanSyncAdapter` — syncs plans and charges
- `SubscriptionSyncAdapter` — creates/terminates Lago subscriptions
- `CustomerBillingAdapter` — upserts customers, verifies payment methods
- `InvoiceBillingAdapter` — passthrough for invoice/receipt/credit-note queries
- `BillingEntitySettingsSyncAdapter` — syncs workspace billing settings

This keeps the product model stable if the billing backend changes.

---

## Scaling

- API: HPA on CPU/memory + request rate (2–20 replicas)
- Replay worker: scale on queue depth + workflow activity latency
- Dispatcher: scale independently to avoid API contention on fan-out
- Web: independently deployable, autoscaled

## Reliability Controls

- Multi-AZ EKS + Multi-AZ RDS with automated backups
- Idempotency keys on replay/event/billed-entry mutation paths
- Temporal workflow IDs per replay job for deterministic deduplication
- Pod disruption budgets per workload

## Recommended SLOs

| Metric | Target |
|--------|--------|
| API p95 latency | < 300ms (excluding Lago downstream) |
| Replay queue pickup | < 30s |
| Replay workflow success rate | > 99.5% |
| Financial mismatch false-positive rate | < 0.1% |

---

## Deployment Flow

1. Terraform provisions/updates AWS infrastructure
2. CI builds and pushes API + web images to ECR
3. Helm deploys chart with environment values and immutable image tags
4. Post-deploy checks:
   - `/health` endpoint for API
   - Replay dispatcher metrics non-erroring
   - Temporal worker task poll active
   - Lago webhook ingress path healthy
