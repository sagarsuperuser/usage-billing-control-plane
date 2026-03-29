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
- CI integration ref for compose/scripts on the same line:
  - `release/v1.44.0-alpha.1`
  - ref `be68660`

Deep CI pulls the Lago backend image from ECR and still uses the pinned Lago repo ref for compose/scripts.

## Rule

When the supported Lago line changes, update these together in the same change:

1. [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)
2. [ci-deep.yml](/Users/superuser/projects/golang/usage-billing-control-plane/.github/workflows/ci-deep.yml)
3. [Makefile](/Users/superuser/projects/golang/usage-billing-control-plane/Makefile)
4. any staging deploy wiring using `LAGO_STAGING_BACKEND_IMAGE_OVERRIDE`

Do not let CI, staging, and repo docs drift silently.
