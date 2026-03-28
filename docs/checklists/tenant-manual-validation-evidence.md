# Tenant Manual Validation Evidence

This checklist is the manual proof record for tenant-scope product validation in Alpha.

Use it when you want evidence that core workspace journeys work in a real environment, not just that pages render.

## Purpose

This document exists to prove three things:

1. the tenant operator flows are understandable and executable end to end
2. Alpha surfaces the right product state without leaking internal engine concepts
3. a specific staging or release build has been manually verified with retained evidence

This is not a generic QA note. It is a release and signoff artifact.

---

## Evidence Standard

For every flow, capture enough detail that another operator can verify the result later without rerunning the whole journey.

Minimum evidence per flow:

- result: `pass`, `pass with polish`, or `fail`
- exact environment URL
- workspace id and workspace name
- actor email or operator identity used for the test
- absolute date and time of execution
- at least one concrete artifact:
  - screenshot
  - copied URL
  - exported CSV
  - API payload sample
  - object id such as customer id, subscription id, invoice id, payment id, or audit event id

If a flow fails, record:

- failing step
- exact visible message
- screenshot
- expected behavior

---

## Execution Record

Fill this section first before running any flow.

| Field | Value |
| --- | --- |
| Environment | |
| Alpha UI URL | |
| Alpha API URL | |
| Release date | |
| Commit SHA | |
| Image tag | |
| Helm revision | |
| Workspace name | |
| Workspace ID | |
| Operator account | |
| Secondary account used | |
| Provider account used | |
| Tested by | |
| Started at | |
| Finished at | |

---

## Result Scale

Use these values consistently:

- `pass`: journey works and messaging is acceptable
- `pass with polish`: journey works but wording, density, or minor UX issues remain
- `fail`: broken behavior, misleading state, contradictory action, or internal-system leakage

---

## Tenant Journey Set

These are the tenant-scope journeys that matter for manual signoff.

| ID | Journey | Why it matters |
| --- | --- | --- |
| T1 | Login, invite, and workspace session | proves operators and invited users can enter the right workspace cleanly |
| T2 | Pricing catalog setup | proves pricing is a usable commercial catalog, not just CRUD |
| T3 | Customer onboarding and billing profile | proves a customer can become commercially usable |
| T4 | Subscription lifecycle | proves plans can be attached to customers and managed coherently |
| T5 | Invoice and payment visibility | proves operators can understand invoice state, payment state, and next action |
| T6 | Workspace access and credential audit | proves member access, service-account posture, and credential evidence are understandable |
| T7 | Usage, recovery, dunning, and explainability | proves operator tooling is usable for investigation and remediation |

---

## T1. Login, Invite, and Workspace Session

### Goal

Prove that tenant entry works for both normal login and invite-based acceptance.

### Preconditions

- target workspace exists
- invited user email is known
- at least one admin workspace account exists

### Execute

1. sign in with a normal workspace account
2. verify the user lands in the expected workspace
3. if the test includes invite acceptance:
   - create an invite
   - open the invite URL in a fresh browser or incognito session
   - complete sign-in with the invited identity
   - verify the invite is accepted and the user lands inside the workspace
4. verify the product uses `Workspace` terminology in the visible session path

### Pass Criteria

- login succeeds without redirect loops
- invite acceptance succeeds without dead-end pages
- workspace selection behaves correctly when multiple workspaces exist
- no misleading `tenant` wording appears in product-facing labels where `workspace` should be used

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Normal login screenshot | |
| Invite URL used | |
| Invite acceptance screenshot | |
| Final landing URL | |
| Notes | |

---

## T2. Pricing Catalog Setup

### Goal

Prove that pricing setup reads like a commercial catalog and that the main tenant pricing flow works.

### Preconditions

- workspace session is active
- workspace has write access for pricing

### Execute

1. open `/pricing`
2. confirm the pricing home reads as one pricing catalog console
3. create one metric
4. create one tax rule
5. create one add-on
6. create one coupon
7. create one plan using the created metric and optional add-on or coupon
8. open the resulting detail pages and review catalog consistency

### Pass Criteria

- the pricing home screen is understandable without redundant repeated cards
- counts and inventory rows update correctly after creation
- the plan can reference the created commercial inputs
- detail pages expose business meaning, not backend terms

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Metric ID | |
| Tax ID | |
| Add-on ID | |
| Coupon ID | |
| Plan ID | |
| Pricing home screenshot | |
| Plan detail screenshot | |
| Notes | |

---

## T3. Customer Onboarding and Billing Profile

### Goal

Prove that a tenant operator can create a customer and understand billing readiness.

### Preconditions

- workspace session is active
- at least one usable pricing plan exists if subscription setup is part of the same pass

### Execute

1. open `/customer-onboarding`
2. create a customer record
3. complete billing profile fields that are required by the workspace flow
4. open customer detail
5. review payment setup state and billing readiness messaging

### Pass Criteria

- customer can be created without ambiguous blocking state
- billing profile requirements are understandable
- customer detail clearly distinguishes missing setup from ready state
- no internal engine wording leaks into the billing profile or readiness copy

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Customer external ID | |
| Customer detail URL | |
| Billing profile screenshot | |
| Payment setup state screenshot | |
| Notes | |

---

## T4. Subscription Lifecycle

### Goal

Prove that the workspace operator can create and inspect a subscription coherently.

### Preconditions

- one customer exists
- one pricing plan exists

### Execute

1. open `/subscriptions/new`
2. create a subscription for the target customer and plan
3. open the created subscription detail screen
4. if enabled in the environment, exercise change or cancel controls

### Pass Criteria

- subscription creation succeeds and routes to detail
- subscription detail shows commercial state clearly
- any change or cancellation action is understandable and reflected correctly
- the flow describes product behavior, not backend sync behavior

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Subscription ID | |
| Subscription detail screenshot | |
| Change or cancel evidence | |
| Notes | |

---

## T5. Invoice and Payment Visibility

### Goal

Prove that invoice and payment surfaces support operator review and action.

### Preconditions

- at least one customer and one subscription exist
- at least one invoice or payment record is available in the workspace

### Execute

1. open `/invoices`
2. open one invoice detail record
3. open `/payments`
4. open one payment detail record
5. open `/payment-operations` if populated
6. review recovery, diagnosis, and linked records

### Pass Criteria

- invoice and payment lists are readable and navigable
- detail screens show next action, posture, and evidence cleanly
- linked customer, invoice, and payment references are coherent
- no backend implementation wording leaks into primary operator copy

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Invoice ID | |
| Payment ID | |
| Invoice detail screenshot | |
| Payment detail screenshot | |
| Payment operations screenshot | |
| Notes | |

---

## T6. Workspace Access and Credential Audit

### Goal

Prove that workspace access control, service-account lifecycle, and credential audit behave correctly and read clearly.

### Preconditions

- workspace admin session is active

### Execute

1. open `/workspace-access`
2. create a service account if one does not already exist
3. issue a credential
4. rotate a credential
5. revoke a credential where safe to do so
6. click `Open audit`
7. confirm the audit section opens for the selected service account
8. click `Download audit CSV`
9. confirm the CSV downloads immediately
10. inspect one audit event in detail

### Pass Criteria

- service-account actions succeed without ambiguous state
- `Open audit` is a real navigation aid, not a silent selector change
- `Download audit CSV` produces a file immediately
- audit detail reads summary-first, with raw IDs secondary
- event language is understandable to an operator

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Service account ID | |
| Credential ID(s) | |
| Audit event ID | |
| Audit detail screenshot | |
| CSV filename or saved artifact path | |
| Notes | |

---

## T7. Usage, Recovery, Dunning, and Explainability

### Goal

Prove that the tenant-side operator tooling is usable for investigation and remediation.

### Preconditions

- tenant workspace has representative records for usage, collections, or billing visibility

### Execute

1. open `/usage-events`
2. open `/dunning`
3. open one dunning run detail if available
4. open `/replay-operations`
5. open `/invoice-explainability`
6. review whether each screen exposes operational state clearly

### Pass Criteria

- these pages read as operator tools, not raw technical consoles
- the user can understand what happened, what is blocked, and what to do next
- no internal billing-engine or storage terminology dominates the screen

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Usage screenshot | |
| Dunning screenshot | |
| Replay screenshot | |
| Explainability screenshot | |
| Notes | |

---

## Terminology and Abstraction Sweep

While executing all tenant journeys, explicitly look for product-language failures.

Fail the sweep if you find any of these in user-facing copy unless they are intentionally internal-only:

- `Lago`
- raw sync mechanics presented as product state
- `tenant` where `workspace` is the intended product term
- raw internal ids without business context
- actions that appear available but are guaranteed to fail

### Evidence

| Field | Value |
| --- | --- |
| Result | |
| Screens reviewed | |
| Leaks found | |
| Screenshots | |
| Notes | |

---

## Final Signoff

| Field | Value |
| --- | --- |
| Overall result | |
| Blocking failures | |
| Pass with polish items | |
| Recommended next fixes | |
| Signed off by | |
| Signed off at | |

## Operator Notes

Use this space for anything that does not fit the structured tables above.

- 
- 
- 
