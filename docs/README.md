# Docs Index

This directory is the long-term documentation home for Alpha.

The goal is to keep the docs set:

- easy to discover
- explicit about source of truth
- stable as the product grows
- maintainable without constant renaming or rewriting

If you are starting cold, read in this order:

1. [Alpha Import Goal](./alpha_import_goal.md)
2. [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)
3. the relevant implementation spec for the slice you are changing
4. the relevant runbook only if you are deploying or operating that area

---

## Start Here

### Product direction

- [Alpha Import Goal](./alpha_import_goal.md)
- [Alpha Import Matrix](./alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](./alpha_wave1_roadmap.md)

### Core architecture

- [Production Architecture](./production-architecture.md)
- [Engineering Standards](./engineering-standards.md)
- [Alpha Lago Boundary](./alpha-lago-boundary.md)
- [Alpha Billing Execution Model](./alpha-billing-execution-model.md)
- [Alpha Notification Architecture](./alpha_notification_architecture.md)

---

## Architecture Models

Use these docs when shaping long-term boundaries and ownership.

- [Alpha Billing Execution Model](./alpha-billing-execution-model.md)
- [Alpha Customer Model](./alpha-customer-model.md)
- [Alpha Workspace Access Model](./alpha-workspace-access-model.md)
- [Alpha API Credentials Model](./alpha_api_credentials_model.md)
- [Alpha Notification Architecture](./alpha_notification_architecture.md)

---

## Implementation Specs

Use these when building or extending concrete product slices.

### Workspace and security

- [Alpha Workspace Access Spec](./alpha_workspace_access_spec.md)
- [Alpha Workspace Billing Binding Spec](./alpha_workspace_billing_binding_spec.md)
- [Alpha API Credentials Spec](./alpha_api_credentials_spec.md)

### Billing product slices

- [Slice 1 Spec: Billing Connections Hardening](./alpha_slice1_billing_connections_spec.md)
- [Slice 2 Spec: Pricing Foundation](./alpha_slice2_pricing_spec.md)
- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](./alpha_slice3_subscriptions_spec.md)
- [Slice 4 Spec: Invoices Visibility](./alpha_slice4_invoices_spec.md)

---

## Legacy Planning Docs

These still carry useful context, but should not be treated as the primary current source of truth if a newer model/spec exists.

- [Alpha Implementation Roadmap](./alpha-implementation-roadmap.md)
- [Alpha Lago Adapter Plan](./alpha-lago-adapter-plan.md)
- [Alpha Provider Connect Plan](./alpha-provider-connect-plan.md)

Rule:
- if a newer `model`, `spec`, or `wave1 roadmap` doc covers the same topic, prefer the newer document.

---

## UI Planning

- [UI Information Architecture Plan](./ui-information-architecture-plan.md)
- [UI Redesign Plan](./ui-redesign-plan.md)

---

## Runbooks and Operations

Use these only for environment setup, deployment, or operational execution.

- [Infra Rollout Runbook](./infra-rollout-runbook.md)
- [Staging Go Live Checklist](./staging-go-live-checklist.md)
- [Real Payment E2E Runbook](./real-payment-e2e-runbook.md)
- [Replay Recovery Live Runbook](./replay-recovery-live-runbook.md)
- [Assisted Tenant Onboarding Runbook](./assisted-tenant-onboarding-runbook.md)
- [Cloudflare Lago Admin Setup](./cloudflare-lago-admin-setup.md)
- [Lago Staging Bootstrap](./lago-staging-bootstrap.md)
- [Temporal Staging Bootstrap](./temporal-staging-bootstrap.md)

---

## Documentation Rules

Read before adding or changing docs:

- [Documentation Standards](./documentation-standards.md)

Short version:

1. Every new doc must have a clear type:
   - goal
   - model
   - spec
   - roadmap
   - runbook
   - checklist
2. Prefer adding a new focused doc over bloating an unrelated one.
3. Do not create duplicate source-of-truth docs for the same topic.
4. Link new docs from this index when they become durable references.
5. If a doc becomes obsolete, mark it as legacy or remove it intentionally.

---

## Naming Guidance

Current docs are mixed between:

- kebab-case
- underscore names
- older plan naming

Do not mass-rename existing files unless there is strong value, because it creates churn.

Going forward, prefer:

- kebab-case for new general docs
- `alpha_*_spec.md` only where that pattern is already established for slice/spec continuity
- explicit suffixes that show the document type:
  - `*-model.md`
  - `*-spec.md`
  - `*-roadmap.md`
  - `*-runbook.md`
  - `*-checklist.md`

---

## Maintaining This Index

Update this file when:

- a new long-lived architecture/model/spec doc is added
- a doc becomes legacy
- a newer source of truth replaces an older plan

Do not update this file for every temporary note or draft unless that note becomes part of the durable docs set.
