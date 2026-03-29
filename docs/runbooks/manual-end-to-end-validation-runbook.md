# Manual End-to-End Validation Runbook

This runbook is the human-operated companion to Alpha's automated staging journeys.

Use it when you want to validate the product the way an operator actually experiences it:

- through the UI
- across role boundaries
- with real handoffs, refreshes, and downloads
- with evidence that the release is usable, not just technically green

This runbook is intentionally broader than the automated staging commands. The automated journeys prove core product state transitions. This runbook adds the operator-facing checks that automation still does not fully prove.

## Use With

- [End-to-End Product Journeys](./end-to-end-product-journeys.md)
- [Tenant Manual Validation Evidence](../checklists/tenant-manual-validation-evidence.md)

Use the journeys doc to understand the canonical automated flow set.

Use this runbook to execute the same flow set manually plus the missing manual-only checks.

Use the tenant evidence checklist to retain proof for the tenant-scope parts of the run.

## When To Run

Run this manual pass when:

1. preparing a release or staging signoff
2. validating a large UI or workflow refactor
3. proving that automated journeys still reflect real operator behavior
4. checking product language and navigability that automated tests do not fully cover

## Required Actors

Prepare these before you start:

- platform admin account
- tenant admin account
- tenant writer account
- invited user account for access flows
- billing provider test account if provider redirects are part of the pass
- one target workspace or a plan to create one

## Evidence Standard

For each journey retain:

- result: `pass`, `pass with polish`, or `fail`
- exact URL
- actor identity used
- absolute execution time
- at least one artifact:
  - screenshot
  - copied URL
  - exported CSV
  - downloaded document
  - concrete id such as workspace id, customer id, subscription id, invoice id, payment id, audit event id, or replay job id

If a journey fails, capture:

- failing step
- exact visible message
- screenshot
- expected behavior

## Canonical Manual Order

Run the journeys in this order so later flows reuse the state created by earlier ones.

| ID | Journey | Automated counterpart | Manual-only additions |
| --- | --- | --- | --- |
| M1 | Platform session and workspace entry | `make test-browser-staging-smoke`, `make test-staging-access-invite-journey` | verify workspace picker, logout/login continuity, and deep-link entry |
| M2 | Billing connection lifecycle | `make test-staging-billing-connection-lifecycle-journey` | verify operator wording, disabled actions, and refresh behavior |
| M3 | Pricing catalog setup | `make test-staging-pricing-journey` | verify list/detail readability, browser back path, and no redundant UI state |
| M4 | Customer onboarding | `make test-staging-customer-onboarding-journey` | verify guidance quality, readiness wording, and directory discoverability |
| M5 | Subscription creation and billing readiness | `make test-staging-subscription-journey` | verify detail readability, linked navigation, and explicit payment readiness |
| M6 | Usage to issued invoice | `make test-staging-usage-to-issued-invoice-journey` | verify invoice explainability entry and invoice detail coherence |
| M7 | Payment setup and payment recovery | `make test-staging-payment-setup-journey`, `make test-staging-browser-payment-setup-journey`, `make test-staging-payment-smoke` | verify UI handoffs, hosted return path, and operator retry flow |
| M8 | Replay, explainability, and recovery tooling | `make test-staging-replay-smoke`, `make test-browser-staging-smoke` | verify diagnostics readability, filter persistence, and deep links |
| M9 | Dunning and collections | `make test-staging-dunning-journey` | verify next action clarity and reminder evidence presentation |
| M10 | Workspace access and credential audit | partial automation only | verify service-account lifecycle, audit detail readability, and CSV download artifact |
| M11 | Subscription change and cancellation | `make test-staging-subscription-change-cancel-journey` | verify final commercial state and navigation consistency |
| M12 | Manual-only product checks | not fully automated | verify authorization guards, refresh resilience, exports, empty states, and terminology |

## M1. Platform Session and Workspace Entry

### Execute

1. sign in as platform admin
2. open the platform overview and one workspace detail
3. if multiple workspaces exist, verify workspace selection behavior
4. sign out and sign back in
5. open a deep link directly into a known tenant route
6. verify the session resolves into the expected workspace
7. if access testing is part of the run, execute the invite acceptance flow in an incognito window

### Pass Criteria

- login succeeds without redirect loops
- deep links land in the correct workspace context
- invite acceptance lands inside the workspace and not on a dead-end page
- session labels use product language consistently

## M2. Billing Connection Lifecycle

### Execute

1. open the platform billing connection list
2. create or select the target provider connection
3. verify status and metadata on the detail page
4. run connection refresh if applicable
5. verify the connection can be mapped to the workspace
6. verify workspace billing posture updates after assignment

### Pass Criteria

- creation, edit, refresh, and assignment are understandable
- disabled or unavailable actions do not mislead the operator
- the platform and workspace views agree on the resulting state

## M3. Pricing Catalog Setup

### Execute

1. open `/pricing`
2. create one metric
3. create one tax rule
4. create one add-on
5. create one coupon
6. create one plan using the created pricing inputs
7. open the created detail pages
8. use browser back and direct URL entry to confirm navigation coherence

### Pass Criteria

- the pricing catalog reads as one commercial control surface
- created records are discoverable in list and detail views
- browser navigation does not lose operator context

## M4. Customer Onboarding

### Execute

1. open `/customer-onboarding`
2. create one customer with usable billing profile values
3. verify onboarding completion state
4. open the resulting customer detail page
5. open the customer directory and rediscover the same record

### Pass Criteria

- onboarding guidance is understandable without backend knowledge
- the customer is discoverable from both completion and directory flows
- readiness and payment setup state are clearly separated

## M5. Subscription Creation and Billing Readiness

### Execute

1. open `/subscriptions/new`
2. create a subscription for the created customer and plan
3. open subscription detail
4. inspect linked customer and plan navigation
5. verify payment readiness or pending setup messaging

### Pass Criteria

- subscription creation routes cleanly to detail
- the commercial state is understandable
- linked navigation stays coherent across subscription, customer, and pricing

## M6. Usage to Issued Invoice

### Execute

1. run the usage-producing setup required by the workspace
2. open the invoices list
3. open the issued invoice detail
4. open invoice explainability for the same invoice
5. verify the invoice can be reasoned about from Alpha surfaces alone

### Pass Criteria

- issued invoice appears in the workspace inventory
- invoice detail, explainability, and linked references are coherent
- the operator can understand how usage became the invoice

## M7. Payment Setup and Payment Recovery

### Execute

1. start from a payment or invoice requiring `collect_payment`
2. follow the UI handoff into customer payment setup
3. send the payment setup request
4. if provider completion is part of the run, complete the hosted flow
5. return to Alpha
6. refresh readiness from the UI
7. retry payment from the UI
8. verify the resulting payment state

### Pass Criteria

- collect-payment handoff is usable and obvious
- hosted setup return path is not confusing
- retry action is only shown when it is actually the right next action
- payment detail, customer detail, and invoice state converge correctly

## M8. Replay, Explainability, and Recovery Tooling

### Execute

1. open replay operations
2. inspect a fresh or known replay job
3. open invoice explainability
4. inspect filters, drill-down, and detail presentation
5. hard refresh the page and repeat the same deep link

### Pass Criteria

- replay diagnostics are readable without internal jargon
- explainability is navigable and stable after refresh
- direct URLs remain usable

## M9. Dunning and Collections

### Execute

1. open `/dunning`
2. inspect at least one dunning run if available
3. open the run detail page
4. verify reminders, next action, and collected evidence

### Pass Criteria

- the operator can tell what happened and what remains to do
- reminder dispatch and collection posture are visible without backend decoding

## M10. Workspace Access and Credential Audit

### Execute

1. open `/workspace-access`
2. create a service account if needed
3. issue a credential
4. rotate a credential
5. revoke a credential where safe
6. use `Open audit`
7. inspect one audit event detail
8. use `Download audit CSV`
9. verify the file actually downloads and is readable

### Pass Criteria

- service-account lifecycle actions succeed and update the UI correctly
- audit detail reads summary-first
- export creates a real artifact, not just a silent state change

## M11. Subscription Change and Cancellation

### Execute

1. open an existing subscription
2. change it to a different plan
3. verify the changed plan across subscription and related commercial views
4. cancel the subscription
5. verify the terminal state and navigation behavior

### Pass Criteria

- change and cancellation paths are understandable
- final state is reflected consistently across the UI

## M12. Manual-Only Product Checks

These checks must be done even if every automated journey already passed.

### Authorization guards

Verify:

1. tenant writer cannot administer `/workspace-access`
2. reader-only identities cannot perform write actions
3. platform-only screens are not accessible from tenant roles

### Navigation and refresh resilience

Verify on the most important screens:

1. hard refresh keeps the operator in a valid state
2. browser back returns to a sensible inventory surface
3. direct deep links load without needing a hidden intermediate click

### Exports and downloads

Verify:

1. audit CSV download works
2. any invoice document or export action produces a real file if enabled
3. downloaded artifacts correspond to the selected record

### Empty and partial states

Verify:

1. no-data states are understandable
2. partial setup states are actionable
3. disabled actions explain themselves well enough

### Terminology and abstraction sweep

Explicitly fail the run if product-facing copy leaks:

- `Lago`
- raw sync mechanics as primary product state
- `tenant` where `workspace` is the correct operator term
- raw ids without business context
- actions that appear available but are guaranteed to fail

## Recording The Run

For tenant-scope evidence, fill:

- [Tenant Manual Validation Evidence](../checklists/tenant-manual-validation-evidence.md)

If platform flows are also in scope, extend the same evidence record with:

- platform admin actor used
- billing connection id
- workspace assignment evidence
- platform route screenshots

## Completion Rule

Treat the manual run as complete only when:

1. all canonical journeys were executed or explicitly marked not applicable
2. manual-only checks were executed
3. retained evidence exists for the run
4. blocking failures and polish items are separated clearly
