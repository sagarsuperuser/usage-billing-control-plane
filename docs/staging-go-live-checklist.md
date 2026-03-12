# Staging Go-Live Checklist

This checklist is the release gate for shipping `lago-usage-billing-alpha` to staging with real AWS infrastructure and real Lago integration.

## 1) Local Preflight

From repo root:

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

## 2) Infrastructure Apply (Staging)

```bash
make tf-plan-staging
make tf-apply-staging
```

Or in GitHub Actions:
- Run workflow: `Infra Deploy`
- inputs:
  - `environment=staging`
  - `action=apply`

## 3) Runtime Secret Wiring

Populate the AWS Secrets Manager secret referenced by Terraform output `runtime_secret_name`.

Required keys:
- `DATABASE_URL`
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`

Optional:
- `LAGO_ORG_TENANT_MAP`

## 4) Release Deploy (Staging)

Auto path:
- merge to `main` (release workflow deploys staging automatically)

Manual path:
- run workflow `Release`
- input `environment=staging`
- optional `image_tag=<existing_sha>` if reusing existing images

## 5) Post-Deploy Verification

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

## 6) Release Gate Decision

Promote only if all pass:
- `integration-real-env-smoke` CI status is green.
- `integration-temporal` CI status is green.
- Staging preflight has zero failures.
- Infra apply completed without drift surprises.
- Post-deploy functional checks pass.
- Rollback workflow has been validated for staging at least once.

## 7) Rollback (If Needed)

Workflow path:
- run `Rollback`
- `environment=staging`
- `revision=<previous_helm_revision>`

Local path:

```bash
make rollback-staging REVISION=<previous_helm_revision>
```
