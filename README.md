# lago-usage-billing-alpha

Production-oriented API foundation for:
- Deterministic rating rules (versioned)
- Meter registry
- Invoice preview integration (Lago-backed)
- Replay/reprocess tooling (idempotent + Temporal orchestration)
- Reconciliation reports (JSON/CSV)

Architecture split:
- `lago-usage-billing-alpha` is the control-plane layer (auth, tenancy, replay/reconcile tooling, audit).
- `../lago` remains the billing engine backend (metering/rating/invoice/payment primitives).
- Alpha integrates with Lago APIs; no local fallback billing engine is used.

## Requirements

- Go 1.25+
- Postgres 14+
- Node.js 20+ (for `web/` UI)

## Migration Model

- Uses **golang-migrate** as migration engine.
- Versioned SQL migrations live in `migrations/*.up.sql`.
- Migrations are applied by `cmd/migrate` for CI/CD-safe rollout.
- Server boot migrations are optional and disabled by default.
- Migration state is tracked in `schema_migrations` (golang-migrate table).
- Legacy custom `schema_migrations(version,name,applied_at)` is auto-renamed once to `schema_migrations_legacy_custom`.

## Run

Set database connection:

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:15432/lago_alpha?sslmode=disable'
```

Optional runtime tuning:

```bash
export DB_QUERY_TIMEOUT_MS=5000
export DB_MIGRATION_TIMEOUT_SEC=60
export RUN_MIGRATIONS_ON_BOOT=false
export RUN_API_SERVER=true
export RUN_REPLAY_WORKER=true
export RUN_REPLAY_DISPATCHER=true
export API_AUTH_ENABLED=true
# optional bootstrap on startup; creates missing keys in Postgres
export API_KEYS='reader_key:reader,writer_key:writer,admin_key:admin'
export UI_SESSION_LIFETIME_SEC=43200
export UI_SESSION_COOKIE_NAME=lago_alpha_ui_session
export UI_SESSION_COOKIE_SECURE=false
export UI_SESSION_COOKIE_SAMESITE=lax
export TEMPORAL_ADDRESS=localhost:7233
export TEMPORAL_NAMESPACE=default
export REPLAY_TEMPORAL_TASK_QUEUE=alpha-replay-jobs
export REPLAY_TEMPORAL_DISPATCH_POLL_MS=750
export REPLAY_TEMPORAL_DISPATCH_BATCH=25
export RUN_PAYMENT_RECONCILE_WORKER=false
export RUN_PAYMENT_RECONCILE_SCHEDULER=false
export PAYMENT_RECONCILE_TEMPORAL_TASK_QUEUE=alpha-payment-reconcile
export PAYMENT_RECONCILE_CRON_SCHEDULE='@every 2m'
export PAYMENT_RECONCILE_WORKFLOW_ID='payment-reconcile/cron'
export PAYMENT_RECONCILE_STALE_AFTER_SEC=300
export PAYMENT_RECONCILE_BATCH=100
export AUDIT_EXPORTS_ENABLED=true
export AUDIT_EXPORT_WORKER_POLL_MS=500
export AUDIT_EXPORT_DOWNLOAD_TTL_SEC=86400
export AUDIT_EXPORT_S3_REGION=us-east-1
export AUDIT_EXPORT_S3_BUCKET=lago-alpha-exports
export AUDIT_EXPORT_S3_ENDPOINT=http://localhost:19000
export AUDIT_EXPORT_S3_ACCESS_KEY_ID=minioadmin
export AUDIT_EXPORT_S3_SECRET_ACCESS_KEY=minioadmin123
export AUDIT_EXPORT_S3_FORCE_PATH_STYLE=true
export LAGO_API_URL=http://localhost:3000
export LAGO_API_KEY=your_lago_api_key
export LAGO_HTTP_TIMEOUT_MS=10000
export LAGO_WEBHOOK_PUBLIC_KEY_TTL_SEC=300
# optional mapping when one alpha control-plane serves multiple Lago organizations
# format: <lago_organization_id>:<tenant_id>,<lago_organization_id>:<tenant_id>
export LAGO_ORG_TENANT_MAP='org_1:tenant_a,org_2:tenant_b'
export LAGO_REPO_PATH=../lago
export VERIFY_LAGO_BACKEND_FOR_TESTS=0
export LAGO_VERIFY_COMPOSE_FILE=docker-compose.dev.yml
```

Make targets:

```bash
make help
make db-up
make migrate
make migrate-status
make migrate-verify
make verify-governance
make preflight-staging
make preflight-prod
make tf-plan-staging
make tf-apply-staging
make tf-plan-prod
make tf-apply-prod
make run
make test-real-env-smoke
make test-integration
make prepare-real-payment-fixture
make test-real-payment-e2e
make lago-verify
```

Run migrations:

```bash
go run ./cmd/migrate
```

Check migration status:

```bash
go run ./cmd/migrate status
```

Verify migration state (fails if dirty, unknown-current, or pending migrations exist):

```bash
go run ./cmd/migrate verify
```

If you need boot-time migrations in non-CI environments:

```bash
export RUN_MIGRATIONS_ON_BOOT=true
```

Start server:

```bash
go run ./cmd/server
```

Server starts on `:8080` by default.
With auth enabled (default):
- machine/API clients can authenticate with `X-API-Key`
- browser control-plane UI can authenticate with cookie session endpoints (`/v1/ui/sessions/login|me|logout`)
- unsafe session-authenticated writes require `X-CSRF-Token`
- `/internal/metrics` still requires an admin principal
API keys are validated against Postgres (`api_keys` table) using hashed key storage and revocation/expiration checks.
Each API key is tenant-scoped (`tenant_id`), and API reads/writes are isolated to that tenant.
`API_KEYS` bootstrap entries are created for tenant `default`.
Tenant tables also have Postgres RLS policies (`app.tenant_id`) for DB-side isolation.
Privileged worker/auth flows explicitly set `app.bypass_rls=on`; regular app paths always run with tenant-scoped sessions.
`LAGO_API_URL` and `LAGO_API_KEY` are required; backend overlap is delegated to Lago with no local fallback:
- meter create/update also sync to Lago billable metrics (`/api/v1/billable_metrics`)
- invoice preview proxies to Lago preview (`/api/v1/invoices/preview`) and returns Lago response/status
- invoice explainability fetches invoice details from Lago (`/api/v1/invoices/:id`) and returns deterministic line-item explainability contract


## Frontend Stack

`web/` is a production-grade SaaS UI foundation built with:
- Next.js (App Router) + TypeScript
- Tailwind CSS v4
- TanStack Query (server-state)
- Zustand (session/UI state)
- Zod + React Hook Form ready for typed form flows

Run UI locally:

```bash
cd web
npx -y pnpm@10.30.0 install
npx -y pnpm@10.30.0 dev
```

Run browser E2E tests:

```bash
cd web
npx -y pnpm@10.30.0 build
npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
npx -y pnpm@10.30.0 e2e
```

Optional API origin override for local split-host setups:

```bash
export NEXT_PUBLIC_API_BASE_URL='http://localhost:8080'
```

UI routes:
- `http://localhost:3000/control-plane`
- `http://localhost:3000/payment-operations`
- `http://localhost:3000/invoice-explainability`

## Endpoints

- `POST /v1/rating-rules`
- `GET /v1/rating-rules`
- `GET /v1/rating-rules/{id}`
- `POST /v1/ui/sessions/login`
- `GET /v1/ui/sessions/me`
- `POST /v1/ui/sessions/logout`
- `POST /v1/meters`
- `GET /v1/meters`
- `GET /v1/meters/{id}`
- `PUT /v1/meters/{id}`
- `POST /v1/invoices/preview`
- `POST /v1/invoices/{invoice_id}/retry-payment`
- `GET /v1/invoices/{invoice_id}/explainability`
- `POST /internal/lago/webhooks`
- `GET /v1/invoice-payment-statuses`
- `GET /v1/invoice-payment-statuses/summary`
- `GET /v1/invoice-payment-statuses/{invoice_id}`
- `GET /v1/invoice-payment-statuses/{invoice_id}/events`
- `POST /v1/usage-events`
- `GET /v1/usage-events`
- `POST /v1/billed-entries`
- `GET /v1/billed-entries`
- `POST /v1/replay-jobs`
- `GET /v1/replay-jobs`
- `GET /v1/replay-jobs/{id}`
- `GET /v1/replay-jobs/{id}/events`
- `POST /v1/replay-jobs/{id}/retry`
- `POST /v1/api-keys`
- `GET /v1/api-keys`
- `POST /v1/api-keys/{id}/revoke`
- `POST /v1/api-keys/{id}/rotate`
- `GET /v1/api-keys/audit`
- `POST /v1/api-keys/audit/exports`
- `GET /v1/api-keys/audit/exports`
- `GET /v1/api-keys/audit/exports/{id}`
- `GET /v1/reconciliation-report`
- `GET /v1/reconciliation-report?format=csv`
- `GET /internal/metrics`
- `GET /internal/ready`

Replay jobs process matching usage/billed data and create billed adjustment entries for deltas (`computed - billed`), including negative credits when needed. Billed entries carry provenance (`source=api|replay_adjustment`, `replay_job_id`).

`POST /v1/usage-events` accepts optional `idempotency_key`:
- first request with a key creates the event (`201`)
- retried request with same key and same payload returns the existing event (`200`)
- same key with a different payload is rejected (`409`)

`POST /v1/billed-entries` accepts optional `idempotency_key`:
- first request with a key creates the billed entry (`201`)
- retried request with same key and same payload returns the existing billed entry (`200`)
- same key with a different payload is rejected (`409`)

`POST /v1/replay-jobs` uses required `idempotency_key`:
- first request for a key creates the replay job (`201`)
- retried request with same key and same filter payload returns the existing job (`200`)
- same key with a different filter payload is rejected (`409`)

Rating-rule governance (`POST /v1/rating-rules`):
- `rule_key` identifies the logical rule family (for example `api_calls`)
- `version` is governed per `(tenant_id, rule_key)` and must be strictly increasing
- `lifecycle_state` supports `draft|active|archived` (default `active`)
- DB enforces unique `(tenant_id, rule_key, version)`
- DB enforces a single `active` version per `(tenant_id, rule_key)`
- creating a new `active` version auto-archives the previously active version for that rule key

`GET /v1/rating-rules` supports:
- `rule_key` (exact logical rule family filter)
- `lifecycle_state` (`draft|active|archived`)
- `latest_only` (`true|false`) to return latest version per `rule_key` in the filtered set

`GET /v1/reconciliation-report` supports:
- `customer_id`
- `from` / `to` (RFC3339 or unix seconds)
- `billed_source` (`api|replay_adjustment`)
- `billed_replay_job_id`
- `mismatch_only` (`true|false`)
- `abs_delta_gte` (integer cents; include rows where `abs(delta_cents) >= abs_delta_gte`)

`POST /v1/invoices/preview` is a Lago pass-through:
- request body must match Lago's `POST /api/v1/invoices/preview` payload
- response body/status are returned from Lago unchanged

`GET /v1/invoices/{invoice_id}/explainability`:
- fetches invoice + fees from Lago (`GET /api/v1/invoices/{id}`) and computes deterministic explainability output
- supports optional query params:
  - `fee_types` (comma-separated, for example `charge,subscription`)
  - `fee_type` (repeatable param alternative)
  - `line_item_sort` (`created_at_asc|created_at_desc|amount_cents_asc|amount_cents_desc`)
  - `page` / `limit`
- returns:
  - invoice metadata (`invoice_id`, `invoice_number`, `invoice_status`, `currency`)
  - deterministic digest (`explainability_version`, `explainability_digest`)
  - explainability line items (`line_items_count`, `line_items`)

`POST /internal/lago/webhooks` ingests Lago-signed webhook events (`X-Lago-Signature` + `X-Lago-Signature-Algorithm=jwt`) and updates tenant-scoped payment visibility projections.
- accepted event delivery returns `202`
- duplicate delivery by `X-Lago-Unique-Key` returns `200` with `idempotent=true`

`GET /v1/invoice-payment-statuses` supports:
- `payment_status` (for example `failed`, `pending`, `succeeded`)
- `invoice_status` (for example `finalized`, `voided`)
- `payment_overdue` (`true|false`)
- `limit` (default `50`, max `500`)
- `offset` (default `0`)

`GET /v1/invoice-payment-statuses/summary` supports:
- `organization_id` (optional filter)
- `stale_after_sec` (optional; if set, computes `stale_attention_required` for overdue/failed/pending invoices with `last_event_at < now-stale_after_sec`)

`GET /v1/invoice-payment-statuses/{invoice_id}/events` supports:
- `webhook_type` filter
- `limit` (default `50`, max `500`)
- `offset` (default `0`)

`GET /v1/billed-entries` supports:
- `customer_id`
- `meter_id`
- `from` / `to` (RFC3339 or unix seconds)
- `billed_source` (`api|replay_adjustment`)
- `billed_replay_job_id`
- `order` (`asc|desc`, default `asc`)
- `limit` (default `100`, max `1000`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)

`GET /v1/usage-events` supports:
- `customer_id`
- `meter_id`
- `from` / `to` (RFC3339 or unix seconds)
- `order` (`asc|desc`, default `asc`)
- `limit` (default `100`, max `1000`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)

`GET /v1/replay-jobs` supports:
- `customer_id`
- `meter_id`
- `status` (`queued|running|done|failed`)
- `limit` (default `20`, max `100`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)

`GET /v1/replay-jobs/{id}/events` returns a replay diagnostics snapshot with matched usage/billed counts and totals for that job filter.
`POST /v1/replay-jobs/{id}/retry` re-queues a failed replay job (`status=failed` only).

`GET /v1/api-keys` supports:
- `limit` (default `20`, max `100`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)
- `role` (`reader|writer|admin`)
- `state` (`active|revoked|expired`)
- `name_contains` (case-insensitive partial match)

`GET /v1/api-keys/audit` supports:
- `limit` (default `20`, max `100`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)
- `api_key_id`
- `actor_api_key_id`
- `action` (`created|revoked|rotated`)
- `format=csv` (exports filtered audit rows as CSV)

`POST /v1/api-keys/audit/exports` body:
- `idempotency_key` (required)
- `api_key_id` (optional filter)
- `actor_api_key_id` (optional filter)
- `action` (`created|revoked|rotated`, optional filter)

`GET /v1/api-keys/audit/exports` supports:
- `status` (`queued|running|done|failed`)
- `requested_by_api_key_id`
- `limit` (default `20`, max `100`)
- `offset` (default `0`)
- `cursor` (opaque token for keyset pagination; do not combine with `offset`)

`GET /v1/api-keys/audit/exports/{id}` returns export job status and `download_url` when complete.

`GET /internal/metrics` includes `http_requests_total` counters keyed by `METHOD ROUTE STATUS`.
Replay metrics include:
- `replay_execution_mode=temporal`
- `replay_temporal_dispatcher`

Payment visibility reconciliation (Temporal, optional):
- scans stale `failed|pending|overdue` invoice projections
- fetches latest invoice state from Lago (`GET /api/v1/invoices/{id}`)
- upserts deterministic `invoice.payment_status_reconciled` events into local projections
- requires `RUN_API_SERVER=true` in the current runtime split

## Local Infra

```bash
docker compose -f docker-compose.postgres.yml up -d
```

You can also use:

```bash
make db-up
make db-ps
make db-down
```

Start a Temporal cluster first (for example local dev at `localhost:7233`), then:

```bash
go run ./cmd/server
```

## Quick Demo

Create rating rule:

```bash
curl -s http://localhost:8080/v1/rating-rules \
  -H 'x-api-key: writer_key' \
  -H 'content-type: application/json' \
  -d '{
    "rule_key":"api_calls",
    "name":"API Calls v1",
    "version":1,
    "lifecycle_state":"active",
    "mode":"graduated",
    "currency":"USD",
    "graduated_tiers":[
      {"up_to":100,"unit_amount_cents":2},
      {"up_to":0,"unit_amount_cents":1}
    ]
  }'
```

Create meter (replace `<rule_id>`):

```bash
curl -s http://localhost:8080/v1/meters \
  -H 'x-api-key: writer_key' \
  -H 'content-type: application/json' \
  -d '{
    "key":"api_calls",
    "name":"API Calls",
    "unit":"call",
    "aggregation":"sum",
    "rating_rule_version_id":"<rule_id>"
  }'
```

Preview invoice (Lago-native payload, replace `<plan_code>`):

```bash
curl -s http://localhost:8080/v1/invoices/preview \
  -H 'x-api-key: reader_key' \
  -H 'content-type: application/json' \
  -d '{
    "customer": {
      "name": "Acme",
      "currency": "USD"
    },
    "plan_code": "<plan_code>",
    "billing_time": "anniversary"
  }'
```

Start replay job:

```bash
curl -s http://localhost:8080/v1/replay-jobs \
  -H 'x-api-key: writer_key' \
  -H 'content-type: application/json' \
  -d '{"idempotency_key":"idem_1","customer_id":"cust_1"}'
```

Reconciliation report:

```bash
curl -s 'http://localhost:8080/v1/reconciliation-report?customer_id=cust_1' \
  -H 'x-api-key: reader_key'
curl -s 'http://localhost:8080/v1/reconciliation-report?customer_id=cust_1&format=csv' \
  -H 'x-api-key: reader_key'
```

Metrics:

```bash
curl -s 'http://localhost:8080/internal/metrics' \
  -H 'x-api-key: admin_key'
```

## Tests

Unit tests:

```bash
go test ./internal/domain
```

Integration tests (real Postgres + Temporal + Lago):

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:15432/lago_alpha_test?sslmode=disable'
export TEST_TEMPORAL_ADDRESS='127.0.0.1:17233'
export TEST_TEMPORAL_NAMESPACE='default'
make lago-up
go test ./internal/api -run TestEndToEndPreviewReplayReconciliation -v
go test ./internal/api -run TestTenantIsolationAcrossAPIKeys -v
go test ./internal/api -run TestAPIKeyLifecycleEndpoints -v
go test ./internal/api -run TestAuditExportToS3 -v
go test ./internal/api -run TestLagoWebhookVisibilityFlow -v
go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v
```

`internal/api` tests are Lago-backed and do not use mock Lago servers.

Or run end-to-end integration workflow:

```bash
make test-integration
```

Run a larger replay dataset correctness check with explicit drift thresholds:

```bash
RUN_LARGE_REPLAY_DATASET=1 \
LARGE_REPLAY_EVENT_COUNT=2000 \
LARGE_REPLAY_MAX_MISMATCH_ROWS=0 \
LARGE_REPLAY_MAX_TOTAL_ABS_DELTA_CENTS=0 \
make test-integration
```

By default `make test-integration` bootstraps Lago from `../lago` and uses:
- `TEST_LAGO_API_KEY=lago_alpha_test_api_key`
- `TEST_TEMPORAL_ADDRESS=127.0.0.1:17233`
- `TEST_TEMPORAL_NAMESPACE=default`

On failure, integration script prints alpha + Lago compose diagnostics by default (`DEBUG_ON_FAILURE=1`).

`TEST_LAGO_API_URL` is auto-detected from `docker compose port api 3000` when omitted.

You can override these and disable auto-bootstrap if your Lago is already running:

```bash
BOOTSTRAP_LAGO_FOR_TESTS=0 \
TEST_LAGO_API_URL='http://localhost:3000' \
TEST_LAGO_API_KEY='your_lago_api_key' \
make test-integration
```

Optional: verify backend replay/correctness suites in the Lago repo before alpha integration tests:

```bash
VERIFY_LAGO_BACKEND_FOR_TESTS=1 \
LAGO_VERIFY_COMPOSE_FILE='docker-compose.dev.yml' \
make test-integration
```

Or run backend verification directly:

```bash
make lago-verify
```

## Production Infra

This repo includes a production baseline using popular tooling:

- Terraform (AWS): `infra/terraform/aws`
- Helm chart: `deploy/helm/lago-alpha`
- Infra CI checks: `.github/workflows/infra.yml`
- AWS Secrets Manager + External Secrets for runtime secret delivery
- Per-workload service accounts for least-privilege pod IAM

Terraform quickstart:

```bash
cp infra/terraform/aws/environments/staging.tfvars.example infra/terraform/aws/environments/staging.tfvars
cp infra/terraform/aws/backends/staging.hcl.example infra/terraform/aws/backends/staging.hcl
make preflight-staging
make tf-plan-staging
make tf-apply-staging
```

Helm quickstart:

```bash
helm lint deploy/helm/lago-alpha
helm template lago-alpha deploy/helm/lago-alpha -f deploy/helm/lago-alpha/environments/staging-values.yaml
```

Role-based scaling model (same backend image):

- API pods: `RUN_API_SERVER=true`, `RUN_REPLAY_WORKER=false`, `RUN_REPLAY_DISPATCHER=false`
- Replay worker pods: `RUN_API_SERVER=false`, `RUN_REPLAY_WORKER=true`, `RUN_REPLAY_DISPATCHER=false`
- Replay dispatcher pods: `RUN_API_SERVER=false`, `RUN_REPLAY_WORKER=false`, `RUN_REPLAY_DISPATCHER=true`

Architecture and rollout docs:
- `docs/production-architecture.md`
- `docs/infra-rollout-runbook.md`
- `docs/staging-go-live-checklist.md`
- `docs/real-payment-e2e-runbook.md`

## Release Pipeline

Workflows:

- `.github/workflows/release.yml`
  - builds and pushes API/web images to ECR
  - auto-deploys `main` to staging
  - supports manual deploy to staging/prod via `workflow_dispatch`
- `.github/workflows/rollback.yml`
  - manual Helm rollback for staging/prod to a chosen revision
- `.github/workflows/infra-deploy.yml`
  - manual Terraform plan/apply for staging/prod using env-scoped backend + tfvars files
- `.github/workflows/real-payment-e2e.yml`
  - manual protected gate for real Stripe test-mode payment collection via alpha + Lago
  - can auto-prepare fixture invoice (`prepare_fixture=true`)
  - validates retry-payment -> Lago terminal payment status -> alpha webhook projection convergence

Required repository variables:

- `AWS_REGION`
- `ECR_API_REPOSITORY`
- `ECR_WEB_REPOSITORY`
- `EKS_CLUSTER_NAME_STAGING`
- `EKS_CLUSTER_NAME_PROD`

Required repository secrets:

- `AWS_BUILD_ROLE_ARN`
- `AWS_DEPLOY_ROLE_ARN_STAGING`
- `AWS_DEPLOY_ROLE_ARN_PROD`
- `AWS_TERRAFORM_ROLE_ARN_STAGING`
- `AWS_TERRAFORM_ROLE_ARN_PROD`
- `TFVARS_STAGING_B64`
- `TF_BACKEND_STAGING_B64`
- `TFVARS_PROD_B64`
- `TF_BACKEND_PROD_B64`

Release preflight (recommended before each staging/prod rollout):

```bash
make preflight-staging
CHECK_GITHUB=1 GITHUB_REPOSITORY=<owner>/<repo> RUN_TERRAFORM_VALIDATE=1 make preflight-staging
```

Local deploy/rollback helpers:

```bash
make deploy-staging IMAGE_TAG=<sha> API_IMAGE_REPOSITORY=<ecr_api_repo> WEB_IMAGE_REPOSITORY=<ecr_web_repo>
make deploy-prod IMAGE_TAG=<sha> API_IMAGE_REPOSITORY=<ecr_api_repo> WEB_IMAGE_REPOSITORY=<ecr_web_repo>
make rollback-staging REVISION=<helm_revision>
make rollback-prod REVISION=<helm_revision>
```

## CI

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on `push` and `pull_request` with:

- `test`: full `make test` suite
- `web-e2e`: Next.js build + Playwright browser flow
- `migration-verify`: fresh Postgres service + `make migrate` + `make migrate-verify` + migration integration test
- `integration-real-env-smoke`: fast real stack gate via `make test-real-env-smoke` (real Postgres + real Temporal + real Lago; no local mocks)
- `integration-temporal`: real Temporal + real Lago + real Postgres/MinIO end-to-end suite via `make test-integration` (includes large replay dataset drift thresholds)

Nightly correctness workflow (`.github/workflows/correctness-nightly.yml`) runs scheduled and on-demand:

- executes the same real integration stack
- uses a larger replay dataset (`LARGE_REPLAY_EVENT_COUNT=20000`)
- fails on drift threshold breaches (`LARGE_REPLAY_MAX_MISMATCH_ROWS=0`, `LARGE_REPLAY_MAX_TOTAL_ABS_DELTA_CENTS=0`)

Infra validation workflow (`.github/workflows/infra.yml`) runs Terraform and Helm validation checks.

Set branch protection to require at least:
- `migration-verify`
- `integration-real-env-smoke`
- `integration-temporal`

Real payment collection gate:
- run `Real Payment E2E` workflow manually against `staging` (and `prod` for controlled checks) using environment-scoped secrets.
- required environment secrets:
  - `ALPHA_API_BASE_URL`
  - `ALPHA_WRITER_API_KEY`
  - `ALPHA_READER_API_KEY`
  - `LAGO_API_URL`
  - `LAGO_API_KEY`

Code ownership:
- `/.github/CODEOWNERS` defines high-risk path owners.
- Replace placeholder teams in `CODEOWNERS` with your real GitHub handles before enabling `Require review from Code Owners`.
- Validate CODEOWNERS format and placeholder readiness with `make verify-governance`.
