# lago-alpha Helm Chart

Deploys independently scalable workloads:

- `api` (HTTP control plane)
- `replay-worker` (Temporal workflow activity worker)
- `replay-dispatcher` (queued replay job scheduler)
- `web` (Next.js UI)

## Render and lint

```bash
helm lint deploy/helm/lago-alpha
helm template lago-alpha deploy/helm/lago-alpha -f deploy/helm/lago-alpha/environments/staging-values.yaml
```

## Install example

```bash
helm upgrade --install lago-alpha deploy/helm/lago-alpha \
  --namespace lago-alpha \
  --create-namespace \
  -f deploy/helm/lago-alpha/environments/staging-values.yaml
```

## Database migrations

- This chart runs database migrations as a Helm `pre-install,pre-upgrade` Job.
- The Job uses the API image and the same runtime secret/config wiring as the API workload.
- This keeps schema rollout serialized at deploy time and avoids relying on `RUN_MIGRATIONS_ON_BOOT=true`.

## Runtime secrets (AWS Secrets Manager + External Secrets)

`ExternalSecret` syncs AWS Secrets Manager object `externalSecrets.runtimeSecretName`
into Kubernetes secret `secretEnv.secretRefName`.

Configure a `ClusterSecretStore` named by `externalSecrets.secretStoreRef.name`
(or set `externalSecrets.createClusterSecretStore=true` to let this chart create one).

The runtime secret JSON must include:

- `DATABASE_URL`
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`

Optional keys:

- `AUDIT_EXPORT_S3_ENDPOINT`
- `AUDIT_EXPORT_S3_ACCESS_KEY_ID`
- `AUDIT_EXPORT_S3_SECRET_ACCESS_KEY`
- `AUDIT_EXPORT_S3_SESSION_TOKEN`
- `LAGO_ORG_TENANT_MAP`
