# Infra Rollout Runbook

## 0. Run Staging/Prod Preflight

Before any Terraform/Helm apply:

```bash
make preflight-staging
# or for prod:
make preflight-prod
```

Optional stricter check (includes GitHub variable/secret validation):

```bash
CHECK_GITHUB=1 GITHUB_REPOSITORY=<owner>/<repo> RUN_TERRAFORM_VALIDATE=1 make preflight-staging
```

## 1. Provision Infrastructure

```bash
cp infra/terraform/aws/environments/staging.tfvars.example infra/terraform/aws/environments/staging.tfvars
cp infra/terraform/aws/backends/staging.hcl.example infra/terraform/aws/backends/staging.hcl

# update values in copied files first, then run:
make tf-plan-staging
make tf-apply-staging
```

For production use:

```bash
cp infra/terraform/aws/environments/prod.tfvars.example infra/terraform/aws/environments/prod.tfvars
cp infra/terraform/aws/backends/prod.hcl.example infra/terraform/aws/backends/prod.hcl

# update values in copied files first, then run:
make tf-plan-prod
make tf-apply-prod
```

Capture outputs:

- `eks_cluster_name`
- `rds_endpoint`
- `audit_exports_bucket`
- `api_ecr_repository_url`
- `web_ecr_repository_url`
- `api_irsa_role_arn`
- `external_secrets_irsa_role_arn`
- `runtime_secret_name`

## 2. Build and Push Images

```bash
docker build -t <api_ecr_repo>:<git_sha> .
docker push <api_ecr_repo>:<git_sha>

docker build -t <web_ecr_repo>:<git_sha> ./web
docker push <web_ecr_repo>:<git_sha>
```

## 3. Create Runtime Secret in AWS Secrets Manager

Populate `runtime_secret_name` output with JSON keys:

- `DATABASE_URL` (RDS endpoint)
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`
- `RATE_LIMIT_REDIS_URL`
- optional `LAGO_ORG_TENANT_MAP`

Then ensure External Secrets has access:

- annotate external-secrets operator service account with `external_secrets_irsa_role_arn`
- set Helm values:
  - `externalSecrets.runtimeSecretName=<runtime_secret_name>`
  - `secretEnv.secretRefName=<k8s_runtime_secret_name>`

## 4. Deploy Helm Chart

```bash
helm upgrade --install lago-alpha deploy/helm/lago-alpha \
  --namespace lago-alpha \
  --create-namespace \
  -f deploy/helm/lago-alpha/environments/staging-values.yaml \
  --set api.image.tag=<git_sha> \
  --set replayWorker.image.tag=<git_sha> \
  --set replayDispatcher.image.tag=<git_sha> \
  --set web.image.tag=<git_sha>
```

## 5. Post-Deploy Checks

- `kubectl get pods -n lago-alpha`
- API: `GET /health`
- Replay worker logs show Temporal poll
- Replay dispatcher logs show queue polling loop
- `kubectl get externalsecret -n lago-alpha`
- Create replay job and verify workflow execution

## 6. Scaling Guidelines

- API: scale by HPA (request latency / CPU / memory).
- Replay worker: increase replicas for high replay throughput.
- Replay dispatcher: keep at least 2 for HA; scale for queue surge fan-out.
- RDS: monitor connection saturation and I/O before vertical scaling.

## 7. Guardrails

- Keep `db_multi_az=true` for production.
- Keep `audit_exports_bucket_force_destroy=false` for production.
- Use Terraform remote state locking (S3 + DynamoDB) for all non-local runs.

Backup/restore drill (staging before first release window):

```bash
AWS_REGION='us-east-1' \
ENVIRONMENT='staging' \
RDS_INSTANCE_ID='<staging_rds_instance_id>' \
DB_SUBNET_GROUP='<staging_db_subnet_group>' \
VPC_SG_IDS='<sg-aaaa,sg-bbbb>' \
CONFIRM_BACKUP_RESTORE='YES_I_UNDERSTAND' \
PLAN_ONLY=1 \
make backup-restore-drill
```

## 8. GitHub Actions Infra Deploy Setup

Configure repository secrets used by `.github/workflows/infra-deploy.yml`:

- `AWS_TERRAFORM_ROLE_ARN_STAGING`
- `AWS_TERRAFORM_ROLE_ARN_PROD`
- `TFVARS_STAGING_B64` (`base64` of `infra/terraform/aws/environments/staging.tfvars`)
- `TF_BACKEND_STAGING_B64` (`base64` of `infra/terraform/aws/backends/staging.hcl`)
- `TFVARS_PROD_B64` (`base64` of `infra/terraform/aws/environments/prod.tfvars`)
- `TF_BACKEND_PROD_B64` (`base64` of `infra/terraform/aws/backends/prod.hcl`)

And repository variable:

- `AWS_REGION`

Latest proven staging drill (`2026-03-15`):
- source instance: `lagoalphastagingdb`
- snapshot: `lago-alpha-staging-drill-snap-20260315121714`
- restored instance: `lago-alpha-staging-drill-restore-20260315121714`
- restored endpoint reached `available` before cleanup
- cleanup completed with restored instance deleted
