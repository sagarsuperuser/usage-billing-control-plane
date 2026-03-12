# Real Payment E2E Runbook (Stripe via Lago)

This runbook validates real payment collection end-to-end:
- retry-payment call through `lago-usage-billing-alpha`
- real Lago payment processing
- real Stripe test-mode charge outcome
- webhook ingestion to alpha payment projections

Use this in staging first, then production.

## 1) Prerequisites

- Lago organization has a Stripe payment provider configured (`secret_key`, redirect URL, `supports_3ds` as needed).
- Alpha has a working Lago integration (`LAGO_API_URL`, `LAGO_API_KEY`) and webhook ingestion endpoint.
- You have a finalized collectible invoice in Lago (`invoice_id`) bound to a customer with a Stripe payment method.

Recommended Stripe test cards:
- Success path: `4242 4242 4242 4242`
- Failure path: `4000 0000 0000 9995`

## 2) Required GitHub Environment Secrets

Define these in `staging` and `production` environments:
- `ALPHA_API_BASE_URL` (for example `https://alpha-api.staging.example.com`)
- `ALPHA_WRITER_API_KEY`
- `ALPHA_READER_API_KEY`
- `LAGO_API_URL` (for example `https://lago-api.staging.example.com`)
- `LAGO_API_KEY`

Set required reviewers on the GitHub environment for protected execution.

## 3) Run via GitHub Actions

Workflow: `Real Payment E2E`

Inputs:
- `environment`: `staging` or `prod`
- `invoice_id`: target Lago invoice id
- `expected_final_status`: `succeeded` or `failed`
- `timeout_sec` / `poll_interval_sec` (optional)

## 4) What the Gate Verifies

1. Confirms invoice exists in Lago and is `finalized`.
2. Calls alpha endpoint `POST /v1/invoices/{id}/retry-payment`.
3. Polls Lago invoice until terminal payment status matches expectation.
4. Polls alpha projection `GET /v1/invoice-payment-statuses/{id}` until it converges.
5. Verifies alpha webhook timeline exists via `GET /v1/invoice-payment-statuses/{id}/events`.

## 5) Local Manual Execution

```bash
ALPHA_API_BASE_URL='https://alpha-api.staging.example.com' \
ALPHA_WRITER_API_KEY='...' \
ALPHA_READER_API_KEY='...' \
LAGO_API_URL='https://lago-api.staging.example.com' \
LAGO_API_KEY='...' \
INVOICE_ID='inv_123' \
EXPECTED_FINAL_STATUS='succeeded' \
TIMEOUT_SEC='600' \
POLL_INTERVAL_SEC='5' \
bash ./scripts/test_real_payment_e2e.sh
```

## 6) Troubleshooting

- `invoice must be finalized`: finalize/regenerate target invoice first.
- `timeout waiting for Lago terminal status`: check Stripe provider config, customer payment method, and Lago worker logs.
- `timeout waiting for alpha projection convergence`: check Lago -> alpha webhook delivery/signature/tenant mapping.
- `expected webhook_type` mismatch: inspect `/v1/invoice-payment-statuses/{id}/events` payload and webhook routing.
