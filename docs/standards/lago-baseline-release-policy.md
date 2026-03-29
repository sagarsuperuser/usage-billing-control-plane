# Lago Baseline Release Policy

This document defines how Alpha changes its supported Lago baseline.

The goal is simple:

- staging/runtime, CI, and docs must move deliberately
- no hidden Lago upgrades through upstream head drift
- no mixed "supported" versions across deployment and validation paths without explicit documentation

## Policy

Alpha must always declare one authoritative Lago deployment baseline and, when needed, one explicitly temporary CI compatibility baseline.

These are not the same thing:

- `authoritative baseline`: the Lago artifact family Alpha actually supports in deployment
- `CI compatibility baseline`: the Lago source ref used by composed integration tests when CI cannot yet run the authoritative artifact directly

If the two differ, the mismatch must be:

- intentional
- documented
- time-bounded

It must never happen by accident.

## Source Of Truth

The canonical Alpha repo baseline file is:

- [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)

This file must define:

- `LAGO_AUTHORITATIVE_BASELINE`
- `LAGO_STAGING_BACKEND_IMAGE_OVERRIDE`
- `LAGO_RELEASE_LINE`
- `LAGO_CI_COMPAT_REF`
- `LAGO_COMPOSE_IMAGE_TAG`

The file answers:

- what Alpha actually supports in deployment
- what CI is validating against
- what local composed integration currently uses

## Current Model

Alpha currently uses:

- authoritative deployment baseline:
  - `lago-fork-v1.44.0-alpha.1`
- CI compatibility baseline:
  - `release/v1.44.0-alpha.1`
  - ref `be68660`
- local composed integration image tag:
  - `v1.43.0`

That mismatch is currently explicit, not accidental.

## Allowed Baseline States

### Preferred state

All three align:

1. deployment artifact line
2. CI integration baseline
3. local composed integration runtime line

This is the target state.

### Transitional state

Deployment and CI may differ only when:

- the authoritative deployment artifact is private or not directly reproducible in CI
- a compatible Lago source ref is pinned explicitly
- the mismatch is written into [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)
- release evidence records that CI is compatibility coverage, not full artifact parity

### Forbidden state

These are not allowed:

- CI cloning upstream head without a pinned ref
- staging moving to a new Lago line without updating the baseline file
- local integration silently using an older line without that being documented
- docs claiming one supported Lago version while CI or staging uses another

## Required Change Set When Bumping Lago

When Alpha intentionally changes the supported Lago baseline, the same PR or release train must update all of the following together.

### 1. Baseline declaration

Update:

- [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)

### 2. CI integration wiring

Update if needed:

- [ci-deep.yml](/Users/superuser/projects/golang/usage-billing-control-plane/.github/workflows/ci-deep.yml)
- [bootstrap_lago.sh](/Users/superuser/projects/golang/usage-billing-control-plane/scripts/bootstrap_lago.sh)
- [test_integration.sh](/Users/superuser/projects/golang/usage-billing-control-plane/scripts/test_integration.sh)

### 3. Deployment/runtime wiring

Update if needed:

- [Makefile](/Users/superuser/projects/golang/usage-billing-control-plane/Makefile)
- any staging deploy helpers using `LAGO_STAGING_BACKEND_IMAGE_OVERRIDE`
- any runbook or environment values that declare the Lago image line

### 4. Documentation

Update:

- this policy doc if the baseline model changes
- [docs/README.md](/Users/superuser/projects/golang/usage-billing-control-plane/docs/README.md) if a source-of-truth document changes
- any runbook or checklist that references a superseded Lago baseline

### 5. Evidence

Record the validation result for the new baseline in the release/change evidence for that PR or release.

## Validation Gates

A Lago baseline bump is not complete until the following are green against the intended baseline:

1. fast CI
- `go-test`
- `web-validate`
- `migration-verify`
- `infra-validate`

2. deep CI
- `web-ui-e2e`
- `integration-smoke`
- `integration-full`

3. staging/runtime verification
- staging deploy succeeds
- critical Alpha flows still work against the new Lago line

4. manual evidence
- use [tenant-manual-validation-evidence.md](/Users/superuser/projects/golang/usage-billing-control-plane/docs/checklists/tenant-manual-validation-evidence.md) for tenant-facing journey coverage when the Lago bump could affect product behavior

## Upgrade Procedure

When moving Alpha to a new Lago baseline:

1. choose the target deployment artifact line
2. decide whether CI can use the same artifact directly
3. if not, pin the closest compatible CI ref explicitly
4. update [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)
5. update CI and bootstrap wiring in the same change
6. run local smoke/integration validation if practical
7. merge only after CI and staging evidence are green
8. if CI compatibility is temporary, create follow-up work to remove the mismatch

## Rollback Rule

If a Lago bump breaks Alpha in CI or staging:

1. revert the baseline declaration first
2. revert CI pinning or bootstrap changes with it
3. revert deployment image overrides with it

Do not leave the repo declaring a baseline that staging is no longer on.

## Ownership

Any change to the Lago baseline must be reviewed as an architecture/runtime change, not treated as a routine dependency bump.

At minimum, the change owner must verify:

- Alpha billing flows still work
- CI is validating a deliberate baseline
- staging/runtime and docs are not lying

## Practical Rule

If someone asks "what Lago do we use?", the answer must come from:

- [config/lago-baseline.env](/Users/superuser/projects/golang/usage-billing-control-plane/config/lago-baseline.env)

If the repo cannot answer that question cleanly, the baseline policy has already failed.
