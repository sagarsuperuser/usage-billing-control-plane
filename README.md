# lago-usage-billing-alpha

API-first alpha for:
- Deterministic rating rules (versioned)
- Meter registry
- Invoice preview simulator
- Replay/reprocess tooling (idempotent)
- Reconciliation reports (JSON/CSV)

## Run

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

Create meter:

```bash
curl -s http://localhost:8080/v1/meters \
  -H 'content-type: application/json' \
  -d '{
    "key":"api_calls",
    "name":"API Calls",
    "unit":"call",
    "aggregation":"sum",
    "rating_rule_version_id":"rrv_000001"
  }'
```

Preview invoice:

```bash
curl -s http://localhost:8080/v1/invoices/preview \
  -H 'content-type: application/json' \
  -d '{
    "customer_id":"cust_1",
    "currency":"USD",
    "items":[{"meter_id":"mtr_000001","quantity":120}]
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

## Tests

```bash
go test ./...
```
