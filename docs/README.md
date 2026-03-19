# Docs Index

This directory is the long-term documentation home for Alpha.

The docs set is organized by document type so the directory itself stays navigable as the product grows.

If you are starting cold, read in this order:

1. [Alpha Import Goal](./goals/alpha_import_goal.md)
2. [Alpha Wave 1 Roadmap](./roadmaps/alpha_wave1_roadmap.md)
3. the relevant implementation spec in [`specs/`](./specs/)
4. the relevant runbook only if you are deploying or operating that area

---

## Directory Layout

- [`goals/`](./goals/): product intent and import direction
- [`models/`](./models/): durable architecture and ownership boundaries
- [`specs/`](./specs/): concrete implementation slices and subsystem contracts
- [`roadmaps/`](./roadmaps/): current sequencing and delivery order
- [`runbooks/`](./runbooks/): operational and deployment procedures
- [`checklists/`](./checklists/): finite verification lists
- [`standards/`](./standards/): documentation and engineering standards
- [`legacy/`](./legacy/): historical plans kept only for context

---

## Start Here

### Product direction

- [Alpha Import Goal](./goals/alpha_import_goal.md)
- [Alpha Import Matrix](./goals/alpha_import_matrix.md)
- [Alpha Wave 1 Roadmap](./roadmaps/alpha_wave1_roadmap.md)

### Core architecture

- [Production Architecture](./models/production-architecture.md)
- [Engineering Standards](./standards/engineering-standards.md)
- [Alpha Lago Boundary](./models/alpha-lago-boundary.md)
- [Alpha Billing Execution Model](./models/alpha-billing-execution-model.md)
- [Alpha Notification Architecture](./models/alpha_notification_architecture.md)

---

## Architecture Models

Use these docs when shaping long-term boundaries and ownership.

- [Alpha Billing Execution Model](./models/alpha-billing-execution-model.md)
- [Alpha Customer Model](./models/alpha-customer-model.md)
- [Alpha Workspace Access Model](./models/alpha-workspace-access-model.md)
- [Alpha API Credentials Model](./models/alpha_api_credentials_model.md)
- [Alpha Notification Architecture](./models/alpha_notification_architecture.md)

---

## Implementation Specs

Use these when building or extending concrete product slices.

### Workspace and security

- [Alpha Workspace Access Spec](./specs/alpha_workspace_access_spec.md)
- [Alpha Workspace Billing Binding Spec](./specs/alpha_workspace_billing_binding_spec.md)
- [Alpha API Credentials Spec](./specs/alpha_api_credentials_spec.md)

### Billing product slices

- [Slice 1 Spec: Billing Connections Hardening](./specs/alpha_slice1_billing_connections_spec.md)
- [Slice 2 Spec: Pricing Foundation](./specs/alpha_slice2_pricing_spec.md)
- [Slice 3 Spec: Subscriptions and Customer-Owned Payment Setup](./specs/alpha_slice3_subscriptions_spec.md)
- [Slice 4 Spec: Invoices Visibility](./specs/alpha_slice4_invoices_spec.md)
- [Slice 5 Spec: Payments Visibility](./specs/alpha_slice5_payments_spec.md)

---

## Active Roadmaps

- [Alpha Wave 1 Roadmap](./roadmaps/alpha_wave1_roadmap.md)

---

## Legacy Planning Docs

These still carry useful context, but should not be treated as the primary current source of truth if a newer model/spec exists.

- [Alpha Implementation Roadmap](./legacy/alpha-implementation-roadmap.md)
- [Alpha Lago Adapter Plan](./legacy/alpha-lago-adapter-plan.md)
- [Alpha Provider Connect Plan](./legacy/alpha-provider-connect-plan.md)
- [UI Information Architecture Plan](./legacy/ui-information-architecture-plan.md)
- [UI Redesign Plan](./legacy/ui-redesign-plan.md)

Rule:
- if a newer `model`, `spec`, or active roadmap covers the same topic, prefer the newer document.

---

## Runbooks and Operations

Use these only for environment setup, deployment, or operational execution.

- [Infra Rollout Runbook](./runbooks/infra-rollout-runbook.md)
- [Staging Go Live Checklist](./checklists/staging-go-live-checklist.md)
- [Real Payment E2E Runbook](./runbooks/real-payment-e2e-runbook.md)
- [Replay Recovery Live Runbook](./runbooks/replay-recovery-live-runbook.md)
- [Assisted Tenant Onboarding Runbook](./runbooks/assisted-tenant-onboarding-runbook.md)
- [Cloudflare Lago Admin Setup](./runbooks/cloudflare-lago-admin-setup.md)
- [Lago Staging Bootstrap](./runbooks/lago-staging-bootstrap.md)
- [Temporal Staging Bootstrap](./runbooks/temporal-staging-bootstrap.md)

---

## Standards

Read before adding or changing docs or core architecture:

- [Documentation Standards](./standards/documentation-standards.md)
- [Engineering Standards](./standards/engineering-standards.md)

---

## Maintaining This Index

Update this file when:

- a new long-lived architecture/model/spec doc is added
- a doc becomes legacy
- a newer source of truth replaces an older plan
- a document type folder gains a new durable reference

Do not update this file for temporary notes or drafts unless they become part of the durable docs set.
