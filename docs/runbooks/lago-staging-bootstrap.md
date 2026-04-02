# Lago Staging Bootstrap

Deploys Lago into the shared staging EKS cluster (`lago` namespace) alongside Alpha (`lago-alpha` namespace).

---

## Topology

- Cluster: shared staging EKS
- Namespace: `lago`
- Helm chart: official `lago/lago`
- Lago API: restricted admin-access only — not a public customer-facing surface

---

## Exposure Model

- Alpha calls Lago over cluster-internal connectivity or the restricted admin API URL
- Lago UI/API hosts sit behind Cloudflare Access (or another SSO proxy) — not open to the public
- `ingress-nginx` is the origin only

---

## What Is Automated

- Namespace creation
- Credential sync from AWS Secrets Manager via ExternalSecret (`lago-credentials`, `lago-encryption`, `lago-secrets`)
- Lago service account with IRSA for S3-backed ActiveStorage
- Helm repo setup and chart deploy
- Sequential safe rollout for deployments sharing the `lago-storage-data` RWO volume
- Rollout and API reachability verification

## What Remains Manual (First Bring-Up)

- Provision Lago Postgres, Redis, S3
- Fill `deploy/lago/environments/staging-values.yaml`
- Generate Lago API key
- Configure Stripe test mode
- Create initial staging org/customers

---

## Prepare Values File

```bash
cp deploy/lago/environments/staging-values.example.yaml \
   deploy/lago/environments/staging-values.yaml
```

Required fields to update:
- `apiUrl`, `frontUrl`
- `global.ingress.apiHostname`, `global.ingress.frontHostname`
- `global.s3.bucket`

Keep defaults:
- `global.existingSecret: lago-credentials`
- `global.serviceAccountName: lago-serviceaccount`
- `encryption.existingSecret: lago-encryption`

---

## Sync Secrets

```bash
make lago-staging-sync-secrets
```

If the shared staging DB password changes, update both sources first:
- Alpha runtime secret: `lago-alpha/staging/runtime`
- Temporal source: `scripts/sync_temporal_staging_secrets.sh`

Then:
```bash
make lago-staging-sync-secrets
make temporal-staging-sync-secrets
```

Restart Alpha and Temporal consumers after secret refresh.

---

## Deploy

```bash
make lago-staging-deploy
```

Current validated fork release: `sagarsuperuser/lago-api` `v1.44.0-alpha.1`

```bash
LAGO_BACKEND_IMAGE_OVERRIDE=139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/api:lago-fork-v1.44.0-alpha.1 \
make lago-staging-deploy
```

---

## Verify

```bash
LAGO_API_URL=https://lago-api-staging.sagarwaidande.org \
make lago-staging-verify

# If not browser-reachable from laptop:
kubectl -n lago port-forward svc/lago-api-svc 3000:3000
LAGO_API_URL=http://127.0.0.1:3000 make lago-staging-verify
```

---

## TLS — Cloudflare DNS-01 (Preferred)

```bash
CLOUDFLARE_API_TOKEN=replace-me \
make cloudflare-sync-dns-token

# Apply Cloudflare issuer
ISSUER_FILE=deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.yaml \
make cert-manager-apply-issuer
```

See [Cloudflare + Lago Admin Setup](./cloudflare-lago-admin-setup.md) for full details.

---

## Alpha Runtime Secret After Bootstrap

Write `lago-alpha/staging/runtime` with:
- `DATABASE_URL`
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`
- `RATE_LIMIT_REDIS_URL`

---

## Upgrade Notes

The `lago-storage-data` PVC is ReadWriteOnce. Naive rolling updates deadlock on EBS multi-attach.

`scripts/deploy_lago_staging.sh` handles affected deployments sequentially (`lago-api`, `lago-billing-worker`, `lago-clock-worker`, `lago-payment-worker`, `lago-pdf-worker`, `lago-worker`). Controlled by `LAGO_SAFE_SHARED_PVC_ROLLOUT=1` (default on).
