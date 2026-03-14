# Staging Go-Live Checklist

This checklist is the release gate for shipping `usage-billing-control-plane` to staging with real AWS infrastructure and real Lago integration.

## Current Proven State

Validated on `2026-03-14` against the live staging stack:

- Alpha API:
  - `https://api-staging.sagarwaidande.org`
- Alpha UI:
  - `https://staging.sagarwaidande.org`
- Lago API:
  - `https://lago-api-staging.sagarwaidande.org`
- Lago UI:
  - `https://lago-staging.sagarwaidande.org`
- Lago webhook endpoint configured:
  - `https://api-staging.sagarwaidande.org/internal/lago/webhooks`
  - signature algorithm: `jwt`
- Runtime verification passed:
  - health endpoint
  - invoice payment status list + summary
  - login pre-auth rate limiting
- Real payment E2E passed for both terminal outcomes:
  - success invoice: `56251c97-597a-4cec-9a22-8106d746def8`
  - failure invoice: `baa27549-32d4-47cd-9f14-d98b61c8b0fa`
- Known staging test customers:
  - `cust_e2e_success`
  - `cust_e2e_failure`

Important staging nuance:
- the Stripe account used here is India-based
- success-path test customers must have a billing address synced to Stripe, or Stripe rejects export transactions even with a valid `4242` card
- the Stripe/Lago staging bootstrap is now automated via `make lago-staging-bootstrap-payments`

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
| P0-5 | Backup/Restore Drill | run snapshot + restore drill in section 8 below | snapshot and restore complete; restored instance endpoint/status captured; cleanup decision documented | AWS CLI output + runbook log | [ ] |
| P0-6 | Deploy/Rollback Safety | run release deploy then rollback then redeploy | release healthy, rollback healthy, redeploy healthy; no migration/data corruption | helm history + smoke logs | [ ] |

Status note as of `2026-03-14`:
- `P0-2` is proven manually in live staging, but the workflow evidence and runbook links still need to be recorded in this checklist.
- `P0-3` and `P0-4` are proven at the API/runtime level; UI screenshots and retained evidence still need to be captured.
- `P0-1`, `P0-5`, and `P0-6` still need formal execution/capture.

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

TLS / edge note:
- when the staging hosts are proxied by Cloudflare, use the Cloudflare DNS-01 ClusterIssuer (`letsencrypt-cloudflare-prod`) instead of origin HTTP-01
- sync the Cloudflare API token first with `make cloudflare-sync-dns-token`
- apply `deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.yaml`

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

Combined acceptance gate shortcut:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='replace_me' \
ALPHA_READER_API_KEY='replace_me' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='replace_me' \
SUCCESS_INVOICE_ID='56251c97-597a-4cec-9a22-8106d746def8' \
FAILURE_INVOICE_ID='baa27549-32d4-47cd-9f14-d98b61c8b0fa' \
make verify-staging-acceptance
```

What this acceptance gate does:
- runs `verify-staging-runtime`
- runs success-path payment E2E
- runs failure-path payment E2E
- emits one combined JSON result for the full alpha payment visibility gate

GitHub workflow shortcut:
- run workflow `Staging Runtime Verify`
- input `environment=staging`
- optionally pass `invoice_id=<known_invoice_id>` for lifecycle/timeline checks

Evidence to retain from `Real Payment E2E`:
- workflow step summary
- uploaded `fixture.json` and `result.json` artifacts
- invoice ids used for success and failure runs

Known good staging evidence from `2026-03-14`:
- success invoice: `56251c97-597a-4cec-9a22-8106d746def8`
- failure invoice: `baa27549-32d4-47cd-9f14-d98b61c8b0fa`
- failure path converged in alpha with:
  - `recommended_action=retry_payment`
  - `requires_action=true`
  - `retry_recommended=true`
- success path converged in alpha with:
  - `recommended_action=none`
  - `requires_action=false`
  - `retry_recommended=false`

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

Browser UI smoke for the payment operations console:

```bash
PLAYWRIGHT_LIVE_BASE_URL='https://staging.sagarwaidande.org' \
PLAYWRIGHT_LIVE_API_KEY='replace_me_writer_key' \
make web-e2e-live
```

Pass criteria:
- UI session login succeeds
- failed-payment list renders
- timeline drawer opens
- retry button submission is accepted from the browser UI

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
export ENVIRONMENT='staging'
export RDS_INSTANCE_ID='<staging_rds_instance_id>'
export DB_SUBNET_GROUP='<staging_db_subnet_group>'
export VPC_SG_IDS='<sg-aaaa,sg-bbbb>'
export CONFIRM_BACKUP_RESTORE='YES_I_UNDERSTAND'

# recommended first run (prints commands only)
PLAN_ONLY=1 make backup-restore-drill

# execute real drill and auto-cleanup restored instance
DELETE_RESTORE_ON_SUCCESS=1 WAIT_FOR_DELETE=1 make backup-restore-drill
```

Pass criteria:
- snapshot reaches `available`
- restored instance reaches `available`
- drill output includes restored endpoint and status
- cleanup policy is explicit (`DELETE_RESTORE_ON_SUCCESS=1` or manual cleanup ticket)

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

## Remaining Cleanup Items

These are not current blockers to the already-proven staging payment/runtime paths, but they should be cleaned up before treating staging as a durable pre-prod environment:

- capture and link formal CI/workflow evidence for the already-proven manual payment E2E runs
- automate Stripe/Lago staging bootstrap for:
  - `cust_e2e_success`
  - `cust_e2e_failure`
  - required Stripe payment method attachment
  - India-account customer address sync requirement
- decide whether to commit additional tracked ingress/cert-manager operational docs or automation still living only in live cluster state
- run and record a formal backup/restore drill
- run and record a formal deploy -> rollback -> redeploy rehearsal
- review remaining uncommitted repo drift and separate true changes from stale local noise

## 10) Rollback (If Needed)

Workflow path:
- run `Rollback`
- `environment=staging`
- `revision=<previous_helm_revision>`

Local path:

```bash
make rollback-staging REVISION=<previous_helm_revision>
```

Release rehearsal path (deploy -> rollback -> redeploy):

```bash
ENVIRONMENT='staging' \
RELEASE_NAME='lago-alpha' \
NAMESPACE='lago-alpha' \
IMAGE_TAG='<candidate_sha>' \
API_IMAGE_REPOSITORY='<staging_api_ecr_repo>' \
WEB_IMAGE_REPOSITORY='<staging_web_ecr_repo>' \
SMOKE_HEALTH_URL='https://alpha-api.staging.example.com/health' \
CONFIRM_RELEASE_REHEARSAL='YES_I_UNDERSTAND' \
PLAN_ONLY=1 \
make rehearse-release-rollback
```
