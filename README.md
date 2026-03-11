# lago-usage-billing-alpha

Production-oriented API foundation for:
- Deterministic rating rules (versioned)
- Meter registry
- Invoice preview simulator
- Replay/reprocess tooling (idempotent + DB queue)
- Reconciliation reports (JSON/CSV)

## Requirements

- Go 1.25+
- Postgres 14+

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
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/lago_alpha?sslmode=disable'
```

Optional runtime tuning:

```bash
export DB_QUERY_TIMEOUT_MS=5000
export DB_MIGRATION_TIMEOUT_SEC=60
export RUN_MIGRATIONS_ON_BOOT=false
export REPLAY_WORKER_POLL_MS=500
export REPLAY_WORKER_ERR_BACKOFF_MIN_MS=250
export REPLAY_WORKER_ERR_BACKOFF_MAX_MS=5000
```

Make targets:

```bash
make help
make db-up
make migrate
make migrate-status
make migrate-verify
make run
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

## Endpoints

- `POST /v1/rating-rules`
- `GET /v1/rating-rules`
- `GET /v1/rating-rules/{id}`
- `POST /v1/meters`
- `GET /v1/meters`
- `GET /v1/meters/{id}`
- `PUT /v1/meters/{id}`
- `POST /v1/invoices/preview`
- `POST /v1/usage-events`
- `POST /v1/billed-entries`
- `POST /v1/replay-jobs`
- `GET /v1/replay-jobs/{id}`
- `GET /v1/reconciliation-report`
- `GET /v1/reconciliation-report?format=csv`
- `GET /internal/metrics`

## Local Postgres

```bash
docker compose -f docker-compose.postgres.yml up -d
```

You can also use:

```bash
make db-up
make db-ps
make db-down
```

## Quick Demo

Create rating rule:

```bash
curl -s http://localhost:8080/v1/rating-rules \
  -H 'content-type: application/json' \
  -d '{
    "name":"API Calls v1",
    "version":1,
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
  -H 'content-type: application/json' \
  -d '{
    "key":"api_calls",
    "name":"API Calls",
    "unit":"call",
    "aggregation":"sum",
    "rating_rule_version_id":"<rule_id>"
  }'
```

Preview invoice (replace `<meter_id>`):

```bash
curl -s http://localhost:8080/v1/invoices/preview \
  -H 'content-type: application/json' \
  -d '{
    "customer_id":"cust_1",
    "currency":"USD",
    "items":[{"meter_id":"<meter_id>","quantity":120}]
  }'
```

Start replay job:

```bash
curl -s http://localhost:8080/v1/replay-jobs \
  -H 'content-type: application/json' \
  -d '{"idempotency_key":"idem_1","customer_id":"cust_1"}'
```

Reconciliation report:

```bash
curl -s 'http://localhost:8080/v1/reconciliation-report?customer_id=cust_1'
curl -s 'http://localhost:8080/v1/reconciliation-report?customer_id=cust_1&format=csv'
```

Metrics:

```bash
curl -s 'http://localhost:8080/internal/metrics'
```

## Tests

Unit tests:

```bash
go test ./internal/domain
```

Integration tests (real Postgres):

```bash
export TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/lago_alpha_test?sslmode=disable'
go test ./internal/api -run TestEndToEndPreviewReplayReconciliation -v
go test ./migrations -run TestRunnerAppliesMigrationsIdempotently -v
```

Or run end-to-end integration workflow:

```bash
make test-integration
```

## CI

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on `push` and `pull_request` with:

- `test`: full `make test` suite
- `migration-verify`: fresh Postgres service + `make migrate` + `make migrate-verify` + migration integration test

Set branch protection to require at least `migration-verify` before merging.
