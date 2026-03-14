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

Live staging smoke for payment operations UI:

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='replace_me_writer_key' \
PLAYWRIGHT_LIVE_READER_API_KEY='replace_me_reader_key' \
npx -y pnpm@10.30.0 exec playwright test tests/e2e/payment-operations-live.spec.ts
```
