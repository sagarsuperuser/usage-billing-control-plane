# Lago Staging Bootstrap

This runbook deploys Lago into the same EKS cluster as `usage-billing-control-plane`, but keeps clean runtime boundaries.

## Topology

- Cluster: shared staging EKS
- Namespace: `lago`
- Helm release: `lago`
- Chart: official `lago/lago`
- Alpha namespace remains `lago-alpha`
- Lago API: restricted admin-access API
- Lago UI: restricted admin surface

## Exposure model

- Do not treat Lago as a customer-facing product API.
- `usage-billing-control-plane` can call Lago over cluster-internal connectivity or the restricted admin API URL.
- If humans use the Lago UI in a browser, the Lago `apiUrl` should be reachable from that same restricted admin path.
- Treat the current EKS ingress/load balancer as the origin only.
- Preferred long-term browser protection is an edge access layer such as Cloudflare Access or another SSO-aware proxy in front of the Lago hosts.
- Good options:
  - restricted admin ingress for both `apiUrl` and `frontUrl`
  - public DNS with Cloudflare Access / identity-aware proxy / IP allowlist
  - private ingress plus VPN/Tailscale/SSO access

Important:
- a browser-visible Lago UI cannot use a cluster-only API URL unless the front end proxies API calls server-side
- so "public" here should mean "browser-reachable for admins", not "open customer/public API surface"

## What is automated

- namespace creation
- sync of Lago DB/Redis credentials from AWS Secrets Manager into Kubernetes via `ExternalSecret`
- persistence of Lago encryption keys in AWS Secrets Manager and sync into Kubernetes via `ExternalSecret`
- persistence of Lago application secrets (`secretKeyBase`, `rsaPrivateKey`) in AWS Secrets Manager and bootstrap into Kubernetes before Helm deploy
- dedicated Lago service account with IRSA for S3-backed ActiveStorage
- Helm repo setup and chart deploy
- sequential safe rollout for Lago deployments that share the `lago-storage-data` `RWO` volume
- rollout verification
- API reachability verification
- bootstrap checklist printing

## What remains manual for first bring-up

- provision Lago Postgres
- provision Lago Redis
- provision Lago S3 bucket/credentials or equivalent object storage
- fill `deploy/lago/environments/staging-values.yaml`
- generate Lago API key
- configure Stripe test mode in Lago
- create initial staging org/customer as needed

## Required preconditions

- `kubectl` points to the staging cluster
- ingress controller exists in the cluster
- DNS hosts resolve if you are using admin ingress/public DNS
- cert-manager/TLS issuer exists if ingress/TLS is enabled at the origin
- managed Lago Postgres exists
- managed Lago Redis exists
- object storage for Lago exists

## Files to prepare

Start from:

- `deploy/lago/environments/staging-values.example.yaml`

Create your real file:

```bash
cp deploy/lago/environments/staging-values.example.yaml deploy/lago/environments/staging-values.yaml
```

Then update at minimum:

- `apiUrl`
- `frontUrl`
- `global.ingress.apiHostname`
- `global.ingress.frontHostname`
- `global.s3.bucket`

Practical guidance:

- `global.existingSecret` should stay set to `lago-credentials`.
- `global.serviceAccountName` should stay set to `lago-serviceaccount`.
- `encryption.existingSecret` should stay set to `lago-encryption`.
- `lago-credentials` is synced from AWS secret `lago/staging/backing-services`.
- `make lago-staging-sync-secrets` reconciles `databaseUrl` in `lago/staging/backing-services` from the live `lagostagingdb` RDS master secret before ExternalSecret sync, so stale DB passwords do not linger in staging.
- `lago-encryption` is synced from AWS secret `lago/staging/encryption`.
- `lago-secrets` is seeded from AWS secret `lago/staging/app-secrets`, then reused by the chart.
- `global.s3.bucket` is Lago's object storage bucket.
- `global.s3.enabled=true` is the preferred long-term mode.
- the deploy script now precreates `lago-serviceaccount` and can annotate it with the Lago IRSA role.
- because the upstream chart still injects static S3 credential env refs when `global.existingSecret` is set, the deploy script pulls the upstream chart locally and patches the S3 credential conditions so IRSA can be used cleanly under Helm 4.
- `apiUrl` and `frontUrl` should match the restricted admin endpoints you actually expose.
- if alpha talks to Lago over cluster-internal DNS, you may still use that internal URL for alpha config instead of the browser-admin URL.

If you want to sync secrets without deploying yet:

```bash
make lago-staging-sync-secrets
```

Important:
- if the shared staging DB password changes, update or verify both remote sources first:
  - Alpha runtime secret: `lago-alpha/staging/runtime`
  - Temporal source: the RDS master secret used by `scripts/sync_temporal_staging_secrets.sh`
- then run:

```bash
make lago-staging-sync-secrets
make temporal-staging-sync-secrets
```

- after the secret values have refreshed in Kubernetes, restart or redeploy the running consumers so they pick up the new env-backed credentials:
  - Alpha: API, replay worker, replay dispatcher
  - Temporal: frontend, history, matching, worker
- then verify:

```bash
make temporal-staging-verify
make verify-staging-runtime
```

## Long-Term Plan

The long-term fix is to stop sharing the Alpha staging DB credentials with Temporal.

Target end-state:
1. Alpha has its own Postgres/database credentials
2. Temporal has its own Postgres/database credentials
3. Alpha and Temporal use separate secret paths and separate secret rotation
4. deploy preflight verifies:
   - Alpha DB auth works
   - Temporal namespace lookup works
5. deploy verification fails if either subsystem cannot authenticate to its own database

Until that separation is implemented, treat the Alpha and Temporal secret update paths as a paired operation whenever the shared staging DB password changes.

## Upgrade behavior

- The upstream Lago chart hardcodes `RollingUpdate` for deployments that mount the shared `lago-storage-data` PVC.
- In this cluster that PVC is `ReadWriteOnce`, so naive rolling updates can deadlock on EBS multi-attach.
- `scripts/deploy_lago_staging.sh` now handles those deployments sequentially after Helm apply:
  - `lago-api`
  - `lago-billing-worker`
  - `lago-clock-worker`
  - `lago-payment-worker`
  - `lago-pdf-worker`
  - `lago-worker`
- This is controlled by `LAGO_SAFE_SHARED_PVC_ROLLOUT=1` and is enabled by default.

## TLS at the origin

There are two workable paths:

1. short-term HTTP-01 over the public origin
2. preferred long-term Cloudflare DNS-01

The preferred long-term path for Lago admin endpoints is Cloudflare DNS-01, because these hosts are meant to sit behind Cloudflare Access or another edge identity layer.

### Short-term HTTP-01 path

For the current `ingress-nginx` setup, the short-term path is:

Staging should keep `externalTrafficPolicy: Local` on the `ingress-nginx-controller` Service. The root cause was the worker node security group only allowing self-referenced TCP 1025-65535, which broke cross-node traffic to services listening on low ports like `80`. That SG rule has been fixed, but `Local` remains the safer edge setting for staging because it avoids unnecessary node hops through the NLB.

1. Install or reconcile ingress-nginx with the tracked staging values if needed:

```bash
make ingress-nginx-install-staging
```

2. Install cert-manager:

```bash
make cert-manager-install
```

3. Copy and fill one issuer template:

- `deploy/cert-manager/cluster-issuer-letsencrypt-staging.example.yaml`
- `deploy/cert-manager/cluster-issuer-letsencrypt-prod.example.yaml`

4. Apply it:

```bash
ISSUER_FILE=deploy/cert-manager/cluster-issuer-letsencrypt-prod.yaml \
make cert-manager-apply-issuer
```

5. Point your real DNS records to the ingress load balancer hostname.

6. Re-deploy Lago so the ingress annotations/hosts line up with the real DNS names.

This keeps the origin valid for HTTPS while still allowing you to put Cloudflare Access or another SSO-aware edge in front later.
For this repo, the preferred default is now Cloudflare DNS-01 instead of HTTP-01.

### Preferred Cloudflare DNS-01 path

1. Put the zone on Cloudflare.
2. Sync the Cloudflare API token into `cert-manager`:

```bash
CLOUDFLARE_API_TOKEN=replace-me \
make cloudflare-sync-dns-token
```

3. Copy and fill one issuer template:

- `deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-staging.example.yaml`
- `deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.example.yaml`

4. Apply it:

```bash
ISSUER_FILE=deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.yaml \
make cert-manager-apply-issuer
```

5. Update the Lago ingress issuer annotation to the Cloudflare issuer name and re-deploy.
   The staging values in this repo now default to `letsencrypt-cloudflare-prod`.

See:

- `docs/runbooks/cloudflare-lago-admin-setup.md`

## Deploy

```bash
make lago-staging-deploy
```

Optional overrides:

```bash
LAGO_NAMESPACE=lago \
LAGO_RELEASE_NAME=lago \
LAGO_VALUES_FILE=deploy/lago/environments/staging-values.yaml \
LAGO_SERVICE_ACCOUNT_ROLE_ARN=arn:aws:iam::123456789012:role/lago-alpha-staging-lago-irsa \
make lago-staging-deploy
```

If you need to validate a backend-only Lago fix before the upstream chart version changes, use a backend image override. The deploy script will apply that image to all Lago backend deployments after Helm so the override survives the deploy path instead of relying on a manual `kubectl set image`.

Current validated fork release for staging:

- `sagarsuperuser/lago-api` branch/tag: `release/v1.44.0-alpha.1` / `v1.44.0-alpha.1`
- `sagarsuperuser/lago` branch/tag: `release/v1.44.0-alpha.1` / `v1.44.0-alpha.1`

```bash
LAGO_BACKEND_IMAGE_OVERRIDE=139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/api:lago-fork-v1.44.0-alpha.1 \
make lago-staging-deploy
```

## Verify

```bash
LAGO_API_URL=https://lago-api-staging.sagarwaidande.org \
make lago-staging-verify
```

If Lago API is not browser-reachable from your laptop, verify in one of these ways instead:

```bash
kubectl -n lago port-forward svc/lago-api-svc 3000:3000
LAGO_API_URL=http://127.0.0.1:3000 make lago-staging-verify
```

or skip URL verification and only verify in-cluster readiness:

```bash
make lago-staging-verify
```

## Bootstrap outputs needed by alpha

After Lago is healthy, capture:

- `LAGO_API_URL`
- `LAGO_API_KEY`

Then write alpha runtime secret `lago-alpha/staging/runtime` with:

- `DATABASE_URL`
- `LAGO_API_URL`
- `LAGO_API_KEY`
- `TEMPORAL_ADDRESS`
- `AUDIT_EXPORT_S3_BUCKET`
- `RATE_LIMIT_REDIS_URL`

## Manual checklist after deploy

1. Create or retrieve a Lago API key.
2. Configure Stripe in test mode.
3. Verify Lago API returns `200` for authenticated requests.
4. Create the first test customer needed for alpha payment E2E.
5. Point alpha `LAGO_API_URL` and `LAGO_API_KEY` to this staging stack.
