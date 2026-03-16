# Web Control Plane

UI for `usage-billing-control-plane` control plane.

## Run

```bash
npx -y pnpm@10.30.0 install
npx -y pnpm@10.30.0 dev
```

Open:
- `http://localhost:3000/control-plane`
- `http://localhost:3000/payment-operations`
- `http://localhost:3000/replay-operations`
- `http://localhost:3000/invoice-explainability`

Optional API base override:

```bash
export NEXT_PUBLIC_API_BASE_URL='http://localhost:8080'
```

## Auth Model

- Browser UI uses cookie-backed sessions (`/v1/ui/sessions/login|me|logout`).
- Unsafe writes from session auth include `X-CSRF-Token`.
- API clients can still use `X-API-Key`.

## Quality

```bash
npx -y pnpm@10.30.0 lint
npx -y pnpm@10.30.0 typecheck
```

## E2E Tests

```bash
npx -y pnpm@10.30.0 build
npx -y pnpm@10.30.0 exec playwright install --with-deps chromium
npx -y pnpm@10.30.0 e2e
```

Live staging smoke for overview, payment operations, replay operations, and invoice explainability:

The staging runtime verifier no longer hammers `/v1/ui/sessions/login` directly. It now probes `/v1/ui/sessions/rate-limit-probe`, which shares the login throttling policy but uses its own route-scoped bucket, so it is safe to run `make verify-staging-runtime` before `make web-e2e-live`.

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_PLATFORM_API_KEY='replace_me_platform_key' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me_writer_key' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me_reader_key' \
make web-e2e-live

ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='replace_me_writer_key' \
ALPHA_READER_API_KEY='replace_me_reader_key' \
OUTPUT_FILE='/tmp/replay-smoke.json' \
make verify-replay-smoke-staging

PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_PLATFORM_API_KEY='replace_me_platform_key' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me_writer_key' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me_reader_key' \
PLAYWRIGHT_LIVE_REPLAY_JOB_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_job_id' /tmp/replay-smoke.json)" \
PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_customer_id' /tmp/replay-smoke.json)" \
PLAYWRIGHT_LIVE_REPLAY_METER_ID="$(jq -r '.live_browser_smoke.playwright_live_replay_meter_id' /tmp/replay-smoke.json)" \
npx -y pnpm@10.30.0 exec playwright test tests/e2e/replay-operations-live.spec.ts

PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me_reader_key' \
PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID='replace_me_invoice_id' \
npx -y pnpm@10.30.0 exec playwright test tests/e2e/invoice-explainability-live.spec.ts
```
