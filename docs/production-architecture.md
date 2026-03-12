# Production Architecture

## Control Plane Topology

- `alpha-api` (stateless HTTP service)
  - tenant auth/rbac
  - Lago proxy endpoints
  - replay/reconcile APIs
  - webhook ingestion
- `alpha-replay-worker` (Temporal worker)
  - runs replay job activities
  - horizontally scalable by queue throughput
- `alpha-replay-dispatcher` (Temporal starter)
  - polls queued replay jobs in Postgres
  - starts Temporal workflows
  - independently scalable for queue pressure
- `alpha-payment-reconcile` (Temporal worker + cron workflow, optional)
  - backfills stale failed/pending/overdue payment projections from Lago
  - keeps payment operations visibility resilient to missed/out-of-order webhook paths
- `alpha-web` (Next.js UI)
  - payment ops + invoice explainability + control plane UI

## Data and Dependencies

- Postgres (RDS): system of record for alpha metadata, replay jobs, api keys, audit metadata.
- Temporal: durable orchestration for replay/reprocess workflows.
- S3: audit export and replay artifact object storage.
- AWS Secrets Manager + External Secrets Operator: runtime config/secret distribution to workloads.
- Lago API: billing engine source-of-truth for invoice/payment primitives.

## Scaling Strategy

- Scale API via HPA on CPU/memory + request-rate.
- Scale replay worker on queue depth + workflow activity latency.
- Scale dispatcher for high queued-job fan-out; keep worker and dispatcher isolated to avoid API contention.
- Keep web independently deployable and autoscaled.

## Reliability Controls

- Pod disruption budgets (add per workload in cluster overlays).
- Multi-AZ EKS worker nodes.
- Multi-AZ RDS with automated backups.
- Idempotency keys on replay/event/billed-entry mutation paths.
- Temporal workflow IDs per replay job for deterministic dedupe.

## Recommended SLOs

- API p95 latency: < 300 ms (excluding Lago downstream failures)
- Replay queue pickup latency: < 30 s
- Replay workflow success rate: > 99.5%
- Financial mismatch false-positive rate: < 0.1%

## Deployment Flow

1. Terraform creates/updates AWS infrastructure.
2. CI builds and pushes API + web images to ECR.
3. Helm deploys chart with environment values and immutable image tags.
4. Post-deploy checks:
   - `/health` for API
   - replay dispatcher metrics non-erroring
   - Temporal worker task poll active
   - Lago webhook ingress acceptance path healthy
