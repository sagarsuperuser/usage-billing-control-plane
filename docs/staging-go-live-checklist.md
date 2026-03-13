# Staging Go-Live Checklist

This checklist is the release gate for shipping `lago-usage-billing-alpha` to staging with real AWS infrastructure and real Lago integration.

## 0) v1 Release Scope

The first customer/internal release is considered ready only when these flows pass in staging:

- auth (API key + UI session), authorization, and rate limiting
- usage/replay/reprocess with Temporal
- payment retry + payment success terminal state
- payment retry + payment failure terminal state
- payment lifecycle visibility (`/v1/invoice-payment-statuses` + events timeline)
- invoice explainability and reconciliation checks

## 1) P0 Release Board (Must Pass Before Release)

Use this as a tracked board. Do not promote while any row remains open.

| ID | Area | Execute | Pass Criteria | Evidence | Status |
| --- | --- | --- | --- | --- | --- |
| P0-1 | Build/Test Gate | `make preflight-staging`, `make test-real-env-smoke`, `make test-integration` | all commands exit `0`; CI jobs `test`, `migration-verify`, `integration-real-env-smoke`, `integration-temporal`, `web-e2e` are green | links to CI runs | [ ] |
| P0-2 | Real Payment E2E | run workflow `Real Payment E2E` twice: once with `expected_final_status=succeeded`, once with `expected_final_status=failed` | both runs pass; alpha projection converges to expected terminal status; webhook timeline exists | workflow URLs + invoice ids | [ ] |
| P0-3 | Payment Failure Visibility | run API checks in section 6 below and validate UI payment operations screen | failed invoice exposes `payment_status=failed`, non-empty `last_payment_error` (when present from Lago), and ordered event history | screenshot + API payload samples | [ ] |
| P0-4 | Runtime Security/Resilience | verify staging env values and limits from section 7 below | `UI_SESSION_COOKIE_SECURE=true`, `UI_SESSION_REQUIRE_ORIGIN=true`, `RATE_LIMIT_ENABLED=true`, `RATE_LIMIT_REDIS_URL` configured; 429 contains `Retry-After` + `X-RateLimit-*` | config diff + curl output | [ ] |
| P0-5 | Backup/Restore Drill | run snapshot + restore drill in section 8 below | snapshot and restore complete; restored DB is reachable; drill notes captured | AWS CLI output + runbook log | [ ] |
| P0-6 | Deploy/Rollback Safety | run release deploy then rollback then redeploy | release healthy, rollback healthy, redeploy healthy; no migration/data corruption | helm history + smoke logs | [ ] |

## 2) Local Preflight

```bash
make preflight-staging
```

Optional stricter preflight:

```bash
CHECK_GITHUB=1 \
GITHUB_REPOSITORY=<owner>/<repo> \
RUN_TERRAFORM_VALIDATE=1 \
make preflight-staging
```

What this checks:
- required tools are installed (`go`, `docker`, `terraform`, `helm`, `kubectl`, `aws`)
- required files exist (`infra/terraform/aws/environments/staging.tfvars`, `infra/terraform/aws/backends/staging.hcl`)
- Terraform formatting, Helm lint/template, migration integration test, governance check
- optional full Go test suite and GitHub variable/secret presence checks

## 3) Infrastructure Apply (Staging)

```bash
make tf-plan-staging
make tf-apply-staging
```

Or in GitHub Actions:
- Run workflow: `Infra Deploy`
- inputs:
  - `environment=staging`
  - `action=apply`

## 4) Runtime Secret Wiring

Populate the AWS Secrets Manager secret referenced by Terraform output `runtime_secret_name`.

Required keys:
- `DATABASE_URL`
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`
- `RATE_LIMIT_REDIS_URL`

Optional:
- `LAGO_ORG_TENANT_MAP`

## 5) Release Deploy (Staging)

Auto path:
- merge to `main` (release workflow deploys staging automatically)

Manual path:
- run workflow `Release`
- input `environment=staging`
- optional `image_tag=<existing_sha>` if reusing existing images

## 6) Post-Deploy Verification (Functional + Payment Visibility)

Run these checks against staging:

```bash
kubectl get pods -n lago-alpha
kubectl get externalsecret -n lago-alpha
kubectl logs -n lago-alpha deploy/lago-alpha-lago-alpha-replay-worker --tail=100
kubectl logs -n lago-alpha deploy/lago-alpha-lago-alpha-replay-dispatcher --tail=100
```

Functional checks:
- API health endpoint responds successfully.
- Create replay job via API; verify corresponding Temporal workflow execution.
- Payment operations UI loads and can list invoice payment statuses.
- Invoice explainability endpoint returns deterministic digest and line items.
- API logs include structured `event=http_request` entries and `X-Request-ID` correlation.
- Runtime auth hardening is active (`APP_ENV=staging`, `UI_SESSION_COOKIE_SECURE=true`, `UI_SESSION_REQUIRE_ORIGIN=true`).
- Runtime rate limiting is active (`RATE_LIMIT_ENABLED=true`, `RATE_LIMIT_REDIS_URL` configured) and 429 responses include `Retry-After` + `X-RateLimit-*`.
- Run `Real Payment E2E` workflow at least once with a Stripe test-mode invoice (`expected_final_status=succeeded`).

Automated verification shortcut:

```bash
ALPHA_API_BASE_URL='https://alpha-api.staging.example.com' \
ALPHA_READER_API_KEY='replace_me' \
INVOICE_ID='inv_xxx' \
make verify-staging-runtime
```

Payment visibility checks (replace env vars):

```bash
export ALPHA_API_BASE_URL='https://alpha-api.staging.example.com'
export ALPHA_READER_API_KEY='replace_me'
export INVOICE_ID='inv_xxx'

curl -sS -H "X-API-Key: ${ALPHA_READER_API_KEY}" \
  "${ALPHA_API_BASE_URL}/v1/invoice-payment-statuses/${INVOICE_ID}" | jq

curl -sS -H "X-API-Key: ${ALPHA_READER_API_KEY}" \
  "${ALPHA_API_BASE_URL}/v1/invoice-payment-statuses/${INVOICE_ID}/events?limit=200&order=desc" | jq
```

Pass criteria for visibility:
- invoice payment endpoint returns `200`
- status payload includes `invoice_id`, `payment_status`, `last_event_at`
- events endpoint returns at least one relevant payment event row
- for failed flows, `payment_status=failed` and `last_payment_error` is populated when provided by upstream Lago payload

## 7) Runtime Security and Rate-Limit Verification

Validate effective runtime config in staging:

```bash
kubectl -n lago-alpha get deploy lago-alpha-lago-alpha-api -o yaml | grep -E "APP_ENV|UI_SESSION_COOKIE_SECURE|UI_SESSION_REQUIRE_ORIGIN|RATE_LIMIT_ENABLED"
kubectl -n lago-alpha logs deploy/lago-alpha-lago-alpha-api --tail=300 | grep -E "event=rate_limiter_enabled.*backend=redis"
```

Rate-limit behavior check:

```bash
export ALPHA_API_BASE_URL='https://alpha-api.staging.example.com'
export BAD_KEY='invalid_key'

for i in $(seq 1 25); do
  curl -s -o /dev/null -D - -H "X-API-Key: ${BAD_KEY}" "${ALPHA_API_BASE_URL}/v1/meters" | tr -d '\r' | grep -E "HTTP/|Retry-After|X-RateLimit-"
done
```

Pass criteria:
- auth/secure cookie/origin hardening flags are present
- API startup logs confirm Redis limiter is enabled
- at least one request is throttled (`HTTP/1.1 429` or `HTTP/2 429`)
- throttled response includes `Retry-After`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## 8) Backup and Restore Drill (Staging)

Run at least once before first release window.

```bash
export AWS_REGION='us-east-1'
export RDS_INSTANCE_ID='<staging_rds_instance_id>'
export SNAPSHOT_ID="lago-alpha-staging-pre-release-$(date +%Y%m%d%H%M)"
export RESTORE_INSTANCE_ID="lago-alpha-staging-restore-$(date +%Y%m%d%H%M)"
export DB_SUBNET_GROUP='<staging_db_subnet_group>'
export VPC_SG_ID='<staging_db_security_group_id>'

aws rds create-db-snapshot \
  --region "${AWS_REGION}" \
  --db-instance-identifier "${RDS_INSTANCE_ID}" \
  --db-snapshot-identifier "${SNAPSHOT_ID}"

aws rds wait db-snapshot-available \
  --region "${AWS_REGION}" \
  --db-snapshot-identifier "${SNAPSHOT_ID}"

aws rds restore-db-instance-from-db-snapshot \
  --region "${AWS_REGION}" \
  --db-instance-identifier "${RESTORE_INSTANCE_ID}" \
  --db-snapshot-identifier "${SNAPSHOT_ID}" \
  --db-subnet-group-name "${DB_SUBNET_GROUP}" \
  --vpc-security-group-ids "${VPC_SG_ID}" \
  --publicly-accessible

aws rds wait db-instance-available \
  --region "${AWS_REGION}" \
  --db-instance-identifier "${RESTORE_INSTANCE_ID}"
```

Pass criteria:
- snapshot reaches `available`
- restored instance reaches `available`
- manual DB connection to restored endpoint succeeds
- cleanup plan documented (retain or delete restored instance)

## 9) Release Gate Decision

Promote only if all pass:
- `integration-real-env-smoke` CI status is green.
- `integration-temporal` CI status is green.
- Staging preflight has zero failures.
- Infra apply completed without drift surprises.
- Post-deploy functional checks pass.
- Real payment collection E2E gate is green for staging.
- Rollback workflow has been validated for staging at least once.
- all P0 rows in section 1 are checked complete.

## 10) Rollback (If Needed)

Workflow path:
- run `Rollback`
- `environment=staging`
- `revision=<previous_helm_revision>`

Local path:

```bash
make rollback-staging REVISION=<previous_helm_revision>
```
