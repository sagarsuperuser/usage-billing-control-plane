# Temporal Staging Bootstrap

Staging Temporal is deployed in-cluster using the official `temporal/temporal` Helm chart.

## Why this shape

- official upstream chart, no custom Temporal manifests
- internal-only service for alpha workers/API
- Postgres-backed persistence using the live staging RDS master user
- no public ingress for Temporal in staging

## Runtime endpoint

Alpha should use:

- `TEMPORAL_ADDRESS=temporal-frontend.temporal.svc.cluster.local:7233`
- `TEMPORAL_NAMESPACE=default`

## Deployment

```bash
make temporal-staging-deploy
make temporal-staging-verify
```

The deploy script:

- discovers the staging RDS endpoint and master username
- reads the live RDS-managed master secret from AWS Secrets Manager
- syncs Temporal ExternalSecret resources into the `temporal` namespace
- currently prepares `temporal-sql`, which is shared by both default and visibility SQL persistence
- deploys the official chart with SQL persistence and schema jobs enabled

## Notes

- Temporal is intentionally internal-only in staging.
- This staging shape is compatible with a later move to Temporal Cloud or a dedicated Temporal data plane.
- The official chart creates the `temporal` and `temporal_visibility` databases during schema bootstrap.
