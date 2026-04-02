# Manual End-to-End Validation Runbook

Human-operated companion to the automated staging journeys. Use this when you need to validate the product the way an operator experiences it: through the UI, across roles, with real handoffs and downloads.

Run when: preparing a release, validating a large refactor, or checking what automation does not fully prove (product language, refresh resilience, exports, authorization guards).

---

## Required Actors

- Platform admin
- Tenant admin, writer, and reader accounts
- Invited user (for access flows)
- Billing provider test account (if provider redirects are in scope)

---

## Evidence Standard

For each journey retain: result (`pass` / `pass with polish` / `fail`), exact URL, actor, timestamp, and one artifact (screenshot, copied URL, exported CSV, or concrete ID).

For failures: failing step, exact visible message, screenshot, expected behavior.

---

## Canonical Manual Order

| ID | Journey | Automated counterpart |
|----|---------|-----------------------|
| M1 | Platform session and navigation | `make test-browser-staging-smoke` |
| M1b | Workspace session entry and deep-link resolution | `make test-staging-access-invite-journey` |
| M2 | Billing connection lifecycle | `make test-staging-billing-connection-lifecycle-journey` |
| M3 | Pricing catalog setup | `make test-staging-pricing-journey` |
| M4 | Customer onboarding | `make test-staging-customer-onboarding-journey` |
| M5 | Subscription creation and billing readiness | `make test-staging-subscription-journey` |
| M6 | Usage to issued invoice | `make test-staging-usage-to-issued-invoice-journey` |
| M7 | Payment setup and recovery | `make test-staging-payment-setup-journey`, `make test-staging-browser-payment-setup-journey`, `make test-staging-payment-smoke` |
| M8 | Replay, explainability, and recovery tooling | `make test-staging-replay-smoke`, `make test-browser-staging-smoke` |
| M9 | Dunning and collections | `make test-staging-dunning-journey` |
| M10 | Workspace access and credential audit | partial automation only |
| M11 | Subscription change and cancellation | `make test-staging-subscription-change-cancel-journey` |
| M12 | Manual-only product checks | not automated |

---

## M1. Platform Session

1. Sign in as platform admin
2. Open platform overview and one workspace detail
3. Sign out and sign back in — confirm clean return

Pass: no redirect loops, platform routes stay in platform scope, session labels use product language.

## M1b. Workspace Session Entry

1. Sign in as a workspace-scoped user (or accept invite in fresh browser)
2. If multi-workspace, verify `/workspace-select` and choose workspace
3. Open a deep link into a known tenant route — verify it resolves correctly

Pass: tenant entry without loops, deep links land in correct context, invite acceptance lands inside workspace.

## M2. Billing Connection Lifecycle

1. Open platform billing connection list
2. Create or select a provider connection, verify detail
3. Run connection refresh if applicable
4. Map to workspace and verify workspace billing posture updates

Pass: create/edit/refresh/assignment are understandable; disabled actions do not mislead.

## M3. Pricing Catalog Setup

1. Open `/pricing`
2. Create one metric, one tax rule, one add-on, one coupon, one plan
3. Open created detail pages, use browser back and direct URL entry

Pass: catalog reads as one commercial surface; browser navigation does not lose context.

## M4. Customer Onboarding

1. Open `/customer-onboarding`
2. Create one customer with billing profile values
3. Open the resulting customer detail page and the customer directory

Pass: guidance is understandable without backend knowledge; readiness and payment setup state are clearly separated.

## M5. Subscription Creation

1. Open `/subscriptions/new`
2. Create a subscription for the created customer and plan
3. Open detail, inspect linked customer and plan navigation

Pass: routes cleanly to detail; commercial state is understandable; linked navigation stays coherent.

## M6. Usage to Issued Invoice

1. Produce usage, open invoices list, open issued invoice detail
2. Open invoice explainability for the same invoice

Pass: operator can understand how usage became the invoice from Alpha surfaces alone.

## M7. Payment Setup and Recovery

1. Start from a payment/invoice requiring `collect_payment`
2. Follow UI handoff into customer payment setup, send request
3. Complete hosted flow (if in scope), return to Alpha, refresh readiness, retry payment

Pass: collect-payment handoff is obvious; retry only shows when it is the right action; payment detail, customer detail, and invoice state converge.

## M8. Replay, Explainability, and Recovery

1. Open replay operations, inspect a replay job
2. Open invoice explainability, inspect filters and drill-down
3. Hard refresh and repeat the same deep link

Pass: diagnostics are readable without internal jargon; direct URLs remain usable after refresh.

## M9. Dunning and Collections

1. Open `/dunning`, inspect a dunning run if available
2. Verify reminders, next action, and collected evidence

Pass: operator can tell what happened and what remains to do.

## M10. Workspace Access and Credential Audit

1. Open `/workspace-access`
2. Create service account, issue/rotate/revoke a credential
3. Open audit, inspect one event detail, download CSV

Pass: lifecycle actions update UI correctly; CSV actually downloads and is readable.

## M11. Subscription Change and Cancellation

1. Open an existing subscription, change to a different plan
2. Verify changed plan across views, then cancel
3. Verify terminal state

Pass: change and cancellation are understandable; final state is consistent across the UI.

## M12. Manual-Only Product Checks

These must be done even if all automated journeys passed.

**Authorization guards**
- Tenant writer cannot administer `/workspace-access`
- Readers cannot perform write actions
- Platform-only screens inaccessible from tenant roles

**Navigation and refresh resilience**
- Hard refresh keeps operator in valid state
- Browser back returns to sensible inventory surface
- Direct deep links load without a hidden intermediate click

**Exports and downloads**
- Audit CSV download works
- Downloaded artifacts correspond to the selected record

**Empty and partial states**
- No-data states are understandable
- Partial setup states are actionable
- Disabled actions explain themselves

**Terminology sweep — fail the run if any surface leaks:**
- `Lago`
- Raw sync mechanics as primary product state
- `tenant` where `workspace` is the correct operator term
- Raw IDs without business context
- Actions that appear available but are guaranteed to fail

---

## Completion Rule

Manual run is complete only when:
1. All canonical journeys executed or explicitly marked N/A
2. Manual-only checks executed
3. Evidence retained for the run
4. Blocking failures and polish items separated clearly
