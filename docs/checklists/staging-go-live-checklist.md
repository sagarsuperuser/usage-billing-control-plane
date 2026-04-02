# Staging Go-Live Checklist

Release gate for `usage-billing-control-plane` against staging.

---

## Current Proven State (2026-03-15)

- Alpha API: `https://api-staging.sagarwaidande.org`
- Alpha UI: `https://staging.sagarwaidande.org`
- Lago API: `https://lago-api-staging.sagarwaidande.org`
- Webhook endpoint: `https://api-staging.sagarwaidande.org/internal/lago/webhooks` (hmac)
- Payment E2E passed (success: `56251c97...`, failure: `baa27549...`)
- Replay smoke passed (job: `rpl_432a72de0e30cac9`)
- Backup/restore drill passed (snapshot: `lago-alpha-staging-drill-snap-20260315121714`)
- Rollback rehearsal passed (image: `staging-20260315-replay-ui`)

Note: Stripe account is India-based — success-path customers require a billing address synced to Stripe.

---

## P0 Board (All Must Pass Before Release)

| ID | Area | Command | Status |
|----|------|---------|--------|
| P0-1 | Build/Test Gate | `make preflight-staging && make test-real-env-smoke && make test-integration` | [ ] |
| P0-2 | Real Payment E2E | Run `Real Payment E2E` workflow twice (`succeeded` + `failed`) | [x] |
| P0-3 | Payment Failure Visibility | Verify `payment_status=failed`, `last_payment_error`, event history | [x] |
| P0-4 | Runtime Security | Verify secure cookie, rate limiting, 429 with `Retry-After` | [x] |
| P0-5 | Backup/Restore Drill | Snapshot + restore drill in staging | [x] |
| P0-6 | Deploy/Rollback Safety | Deploy → rollback → redeploy cycle | [x] |

P0-1 remains open (CI/preflight evidence not yet formally captured). All others proven on 2026-03-15.

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

Combined acceptance gate:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' ALPHA_READER_API_KEY='...' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='...' \
SUCCESS_INVOICE_ID='56251c97-597a-4cec-9a22-8106d746def8' \
FAILURE_INVOICE_ID='baa27549-32d4-47cd-9f14-d98b61c8b0fa' \
make verify-staging-acceptance
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

### 7. Backup/Restore Drill

```bash
AWS_REGION='us-east-1' ENVIRONMENT='staging' \
RDS_INSTANCE_ID='<staging_rds_instance_id>' \
DB_SUBNET_GROUP='<staging_db_subnet_group>' \
VPC_SG_IDS='<sg-aaaa,sg-bbbb>' \
CONFIRM_BACKUP_RESTORE='YES_I_UNDERSTAND' \
DELETE_RESTORE_ON_SUCCESS=1 WAIT_FOR_DELETE=1 \
make backup-restore-drill
```

### 8. Rollback (If Needed)

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
- Rollback validated at least once
- All P0 rows checked
