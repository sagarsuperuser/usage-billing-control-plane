# Staging Go-Live Checklist

Release gate for `usage-billing-control-plane` against staging.

---

## Staging Endpoints

- Alpha API: `https://api-staging.sagarwaidande.org`
- Alpha UI: `https://staging.sagarwaidande.org`
- Lago API: `https://lago-api-staging.sagarwaidande.org`
- Webhook endpoint: `https://api-staging.sagarwaidande.org/internal/lago/webhooks` (hmac)

Note: Stripe account is India-based — success-path customers require a billing address synced to Stripe.

---

## P0 Board (All Must Pass Before Release)

| ID | Area | Command | Status |
|----|------|---------|--------|
| P0-1 | Build/Test Gate | `make preflight-staging && make test-real-env-smoke && make test-integration` | [ ] |
| P0-2 | Real Payment E2E | `make test-staging-payment-smoke LAGO_API_KEY='...'` (success + failure) | [ ] |
| P0-3 | Payment Failure Visibility | Verify `payment_status=failed`, `last_payment_error`, event history | [ ] |
| P0-4 | Runtime Security | Verify secure cookie, rate limiting, 429 with `Retry-After` | [ ] |

---

## Release Steps

### 1. Preflight

```bash
make preflight-staging
```

### 2. Infra Apply

```bash
make tf-plan-staging
make tf-apply-staging
```

Or: run `Infra Deploy` workflow with `environment=staging, action=apply`.

### 3. Runtime Secret

Populate `runtime_secret_name` in AWS Secrets Manager:
- `DATABASE_URL`, `LAGO_API_URL`, `LAGO_API_KEY`, `TEMPORAL_ADDRESS`, `AUDIT_EXPORT_S3_BUCKET`, `RATE_LIMIT_REDIS_URL`

Multi-tenant Lago routing lives on the tenant record (`lago_organization_id`, `lago_billing_provider_code`) — no `LAGO_ORG_TENANT_MAP`.

### 4. Deploy

Auto: merge to `main`

Manual:
```bash
helm upgrade --install lago-alpha deploy/helm/lago-alpha \
  --namespace lago-alpha --create-namespace \
  -f deploy/helm/lago-alpha/environments/staging-values.yaml \
  --set api.image.tag=<git_sha> \
  --set replayWorker.image.tag=<git_sha> \
  --set replayDispatcher.image.tag=<git_sha> \
  --set web.image.tag=<git_sha>
```

### 5. Post-Deploy Checks

```bash
kubectl get pods -n lago-alpha
kubectl get externalsecret -n lago-alpha
kubectl logs -n lago-alpha deploy/lago-alpha-lago-alpha-replay-worker --tail=100
kubectl logs -n lago-alpha deploy/lago-alpha-lago-alpha-replay-dispatcher --tail=100
```

Runtime health check:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_READER_API_KEY='...' \
make verify-staging-runtime
```

Payment smoke (auto-mints keys):

```bash
LAGO_API_KEY='...' make test-staging-payment-smoke
```

Browser smoke:

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_PLATFORM_API_KEY='...' \
PLAYWRIGHT_LIVE_WRITER_API_KEY='...' \
PLAYWRIGHT_LIVE_READER_API_KEY='...' \
make web-e2e-live
```

### 6. Security Verification

```bash
kubectl -n lago-alpha get deploy lago-alpha-lago-alpha-api -o yaml \
  | grep -E "APP_ENV|UI_SESSION_COOKIE_SECURE|UI_SESSION_REQUIRE_ORIGIN|RATE_LIMIT_ENABLED"
```

Rate-limit check (expect 429 after ~20 bad-key requests):

```bash
for i in $(seq 1 25); do
  curl -s -o /dev/null -D - -H "X-API-Key: invalid_key" \
    "$ALPHA_API_BASE_URL/v1/meters" | tr -d '\r' | grep -E "HTTP/|Retry-After|X-RateLimit-"
done
```

### 7. Rollback (If Needed)

```bash
make rollback-staging REVISION=<previous_helm_revision>
```

---

## Release Gate Decision

Promote only when all pass:
- CI: `integration-real-env-smoke`, `integration-temporal` green
- Staging preflight zero failures
- Infra apply completed without drift
- Post-deploy functional checks pass
- Real payment E2E green (success + failure)
- All P0 rows checked
