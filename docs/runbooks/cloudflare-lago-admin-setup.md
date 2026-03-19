# Cloudflare DNS-01 And Access Setup

This is the preferred long-term setup for both:

- public alpha product origins
- restricted Lago admin origins

## Target model

- Cloudflare manages DNS for `sagarwaidande.org`
- cert-manager uses Cloudflare DNS-01 for origin certificates
- alpha product origins:
  - `staging.sagarwaidande.org`
  - `api-staging.sagarwaidande.org`
- Cloudflare Access protects:
  - `lago-staging.sagarwaidande.org`
  - `lago-api-staging.sagarwaidande.org`
- `ingress-nginx` remains the EKS origin only

## Why this is preferred

- avoids fragile HTTP-01 challenge routing once Cloudflare proxies the zone
- fits the future edge rate-limiting/WAF plan
- fits browser-reachable but restricted Lago UI/API
- keeps alpha product origins and Lago admin origins on one certificate strategy
- keeps origin simple and puts policy at the edge

## Cloudflare pieces

You need:

- Cloudflare zone for `sagarwaidande.org`
- DNS records for:
  - `staging.sagarwaidande.org`
  - `api-staging.sagarwaidande.org`
  - `lago-staging.sagarwaidande.org`
  - `lago-api-staging.sagarwaidande.org`
- API token with DNS edit permission for the zone
- Cloudflare Access application/policy for both Lago hosts

Recommended API token permissions:

- Zone:DNS:Edit
- Zone:Zone:Read

Scope the token to the single zone only.

## Repo-side steps

1. Sync the Cloudflare API token secret into `cert-manager`:

```bash
CLOUDFLARE_API_TOKEN=replace-me \
make cloudflare-sync-dns-token
```

2. Copy and fill the Cloudflare DNS-01 issuer:

- `deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-staging.example.yaml`
- `deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.example.yaml`

3. Apply the issuer:

```bash
ISSUER_FILE=deploy/cert-manager/cluster-issuer-letsencrypt-cloudflare-prod.yaml \
make cert-manager-apply-issuer
```

4. Update ingress issuer annotations to the Cloudflare issuer name:

- `letsencrypt-cloudflare-prod` for prod-like origin certs
- `letsencrypt-cloudflare-staging` for dry runs first

5. Re-deploy workloads that request certificates:

```bash
IMAGE_TAG=<current_alpha_tag> \
API_IMAGE_REPOSITORY=<alpha_api_repo> \
WEB_IMAGE_REPOSITORY=<alpha_web_repo> \
make deploy-staging
```

```bash
make lago-staging-deploy
```

Alpha and Lago can now both use the same DNS-01 ClusterIssuer.

```bash
make lago-staging-deploy
```

## Cloudflare Access

Protect the Lago hosts with an Access app/policy.

Recommended:

- allow only your admin emails/groups
- require identity provider login
- do not expose Lago as a public customer-facing surface

## After migration

Validate:

```bash
kubectl -n lago-alpha get certificate,secret,ingress
kubectl -n lago-alpha get order,challenge
kubectl -n lago get certificate,secret,ingress
kubectl -n lago get order,challenge
curl -I https://staging.sagarwaidande.org
curl -I https://api-staging.sagarwaidande.org
curl -I https://lago-staging.sagarwaidande.org
curl -I https://lago-api-staging.sagarwaidande.org
```

Then add Cloudflare-side:

- edge rate limiting for alpha public origins
- Access policies
- edge rate limiting for Lago admin origins
- optional WAF rules
