# Lago Baseline

Alpha has one Lago source of truth:

- [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)

If someone asks "what Lago do we use?", answer from that file.

## Current Baseline

- supported Lago baseline:
  - `lago-fork-v1.44.0-alpha.1`
- staging backend image:
  - `139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/api:lago-fork-v1.44.0-alpha.1`
- CI integration backend image:
  - `139831607173.dkr.ecr.us-east-1.amazonaws.com/lago-alpha-staging/api:lago-fork-v1.44.0-alpha.1`
- CI integration compose stack:
  - [fixtures/lago-ci/docker-compose.yml](/Users/superuser/projects/golang/usage-billing-control-plane/fixtures/lago-ci/docker-compose.yml)

Deep CI and local composed integration use the Alpha-owned Lago CI compose stack in this repo.

## Rule

When the supported Lago line changes, update these together in the same change:

1. [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)
2. [ci-deep.yml](/Users/superuser/projects/golang/usage-billing-control-plane/.github/workflows/ci-deep.yml)
3. [Makefile](/Users/superuser/projects/golang/usage-billing-control-plane/Makefile)
4. [fixtures/lago-ci/docker-compose.yml](/Users/superuser/projects/golang/usage-billing-control-plane/fixtures/lago-ci/docker-compose.yml)
5. any staging deploy wiring using `LAGO_STAGING_BACKEND_IMAGE_OVERRIDE`

Do not let CI, staging, and repo docs drift silently.
