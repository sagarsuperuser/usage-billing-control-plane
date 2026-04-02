# End-to-End Product Journeys

Canonical journey set for staging validation. All journeys are `implemented`.

---

## Journey Table

| # | Journey | Make Target |
|---|---------|-------------|
| 1 | Pricing configuration | `make test-staging-pricing-journey` |
| 2 | Subscription billing | `make test-staging-subscription-journey LAGO_API_KEY='...'` |
| 3 | Payment setup + collect-payment | `make test-staging-payment-setup-journey` |
| 4 | Payment retry and failure | `make test-staging-payment-smoke LAGO_API_KEY='...'` |
| 5 | Replay and recovery | `make test-staging-replay-smoke` |
| 6 | Browser operator | `make test-browser-staging-smoke` |
| 7 | Browser-led payment setup | `make test-staging-browser-payment-setup-journey LAGO_API_KEY='...'` |
| 8 | Access and invite membership | `make test-staging-access-invite-journey` |
| 9 | Customer onboarding | `make test-staging-customer-onboarding-journey` |
| 10 | Subscription change and cancellation | `make test-staging-subscription-change-cancel-journey LAGO_API_KEY='...'` |
| 11 | Usage to issued invoice | `make test-staging-usage-to-issued-invoice-journey` |
| 12 | Dunning and collections | `make test-staging-dunning-journey` |
| 13 | Billing connection lifecycle | `make test-staging-billing-connection-lifecycle-journey` |

---

## Release Confidence Set

Minimum (every release):

```bash
make verify-staging-runtime
make test-staging-payment-smoke LAGO_API_KEY='...'
make test-staging-replay-smoke
make test-browser-staging-smoke
```

Extended (significant releases, add):

```bash
make test-staging-pricing-journey
make test-staging-subscription-journey LAGO_API_KEY='...'
make test-staging-payment-setup-journey
```

---

## Data Rules

- Use per-run fixture IDs — never rely on fixed customer IDs like `cust_e2e_success`
- Never rely on stale tenant billing mapping being present
- Keep cleanup separate from bootstrap
- Shared staging mutation primitives live in `cmd/admin`; shell scripts are thin orchestration wrappers

---

## For Manual Passes

See [Manual End-to-End Validation Runbook](./manual-end-to-end-validation-runbook.md) for the operator-facing complement: cross-role checks, refresh resilience, exports, terminology review.
