# Testing Strategy

Two goals: demo-ready + engineering interview confidence.

---

## Honest Baseline

**Solid today:**
- API key lifecycle, tenant isolation, rate limiting — Go integration tests
- Payment retry logic, replay idempotency — Go integration tests
- Rating rules engine — Go integration tests
- UI component rendering (mocked) — Playwright session specs

**Gaps that matter:**
- The core operator journey (customer → subscription → invoice → payment collection) has no end-to-end automated test
- Dunning/retry has backend logic but no integration test
- Subscription sync to Lago not tested in code
- Browser smoke exits 0 even when 4/5 specs skip (false confidence)

---

## Tier 1 — Manual Demo Run

**Do this before anything else.** If you can't walk it, you can't demo it.

Walk `docs/checklists/staging-go-live-checklist.md` in full, in order. All 10 steps. This also seeds staging data needed for automated gates.

After Step 8 (invoice + payment collection), save:
- The invoice ID — needed for browser smoke fixtures
- The subscription ID — needed for replay smoke

**Expected time:** 60–90 minutes. Surface and fix small bugs before the demo does.

---

## Tier 2 — Automated Gates

Run after the manual walkthrough (which seeds the data these depend on).

```bash
# Health + rate limiting
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_READER_API_KEY='...' \
make verify-staging-runtime

# Pricing journey
make test-staging-pricing-journey

# Subscription journey
LAGO_API_KEY='...' make test-staging-subscription-journey

# Payment smoke (Stripe success + failure)
LAGO_API_KEY='...' make test-staging-payment-smoke

# Replay smoke
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' \
ALPHA_READER_API_KEY='...' \
make test-staging-replay-smoke

# Browser smoke — pass fixture IDs from the manual walkthrough
PLAYWRIGHT_LIVE_PAYMENT_INVOICE_ID='<invoice from step 8>' \
PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID='<same invoice>' \
make test-browser-staging-smoke
```

All 6 must exit 0 before the demo.

> **Note on browser smoke:** Without fixture IDs, 4/5 specs silently skip and the suite still exits 0. Always pass real IDs from a completed walkthrough.

---

## Tier 3 — Gap Closure

Do these before interviews or prod promotion — not required for the demo.

| Gap | What to add | Why it matters |
|-----|-------------|----------------|
| Dunning/retry | Go integration test: overdue invoice → retry job runs → payment attempted → status updated | "What happens when payment fails?" is always asked |
| Invoice generation | Integration test: subscription active + usage events → invoice issued with correct amounts | Shows billing math is trusted |
| Subscription → Lago sync | Integration test: create subscription → verify Lago subscription created with correct external ID | Shows you understand the sync boundary |
| False-green browser smoke | Fail if fixture IDs are missing rather than skipping silently | Shows you care about test reliability |

**Skip for now:** webhook pipeline testing, multi-tenant edge cases beyond what exists, load testing.

---

## Interview Preparation

The Tier 3 gaps are the questions that come up. Have answers ready even before the tests are written:

| Question | Point to |
|----------|----------|
| "How do you test billing accuracy?" | Rating rules integration test + pricing journey script; name invoice generation test as next priority |
| "What happens when Stripe fails?" | Payment retry test + dunning architecture; honest that dunning integration test is next |
| "How do you prevent double-charging?" | Replay idempotency test — this is the strongest piece, leads well |
| "How do you test at scale?" | Replay smoke with 10k events |

---

## Sequencing

```
Now:                Walk the 10-step manual checklist on staging
                    Fix anything that breaks

Before demo:        Run all 6 automated gates with real fixture IDs
                    Verify all exit 0

Before interviews:  Add dunning integration test (highest-value gap)
                    Add invoice generation test
                    Fix false-green browser smoke
```
