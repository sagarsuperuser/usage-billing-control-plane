# Real Payment E2E Runbook (Stripe via Lago)

This runbook validates real payment collection end-to-end:
- retry-payment call through `usage-billing-control-plane`
- real Lago payment processing
- real Stripe test-mode charge outcome
- webhook ingestion to alpha payment projections

Use this in staging first, then production.

## 1) Prerequisites

- Lago organization has a Stripe payment provider configured (`secret_key`, redirect URL, `supports_3ds` as needed).
- Alpha has a working Lago integration (`LAGO_API_URL`, `LAGO_API_KEY`) and webhook ingestion endpoint.
- You have a finalized collectible invoice in Lago (`invoice_id`) bound to a customer with a Stripe payment method.
- If the Stripe account is India-based, the Lago customer must also have a complete billing address synced to Stripe. Without customer name/address, Stripe can reject export transactions even with a valid test card.

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
- `invoice_id`: target Lago invoice id (optional when `prepare_fixture=true`)
- `prepare_fixture`: auto-create one-off finalized fixture invoice
- `fixture_customer_external_id`: customer to use when fixture prep is enabled
- `fixture_add_on_code`: fixture add-on code (default `alpha-real-payment-fixture`)
- `fixture_unit_amount_cents`: fixture line-item cents (default `199`)
- `expected_final_status`: `succeeded` or `failed`
- `timeout_sec` / `poll_interval_sec` (optional)

## 4) What the Gate Verifies

1. Confirms invoice exists in Lago and is `finalized`.
2. Calls alpha endpoint `POST /v1/invoices/{id}/retry-payment` when the invoice has not already reached the expected terminal status.
3. Polls Lago invoice until terminal payment status matches expectation.
4. Polls alpha projection `GET /v1/invoice-payment-statuses/{id}` until it converges.
5. Verifies alpha webhook timeline exists via `GET /v1/invoice-payment-statuses/{id}/events`.
6. Verifies alpha lifecycle summary via `GET /v1/invoice-payment-statuses/{id}/lifecycle`.

Expected lifecycle assertions:
- success flow: `recommended_action=none`, `requires_action=false`, `retry_recommended=false`
- failure flow: `recommended_action=retry_payment`, `requires_action=true`, `retry_recommended=true`

Workflow evidence:
- uploads `fixture.json` and `result.json` as workflow artifacts
- writes invoice id, Lago status, alpha status, lifecycle action, and event count to the GitHub Actions step summary

## 5) Local Manual Execution

Bootstrap the staging Stripe/Lago fixtures first if needed. The recommended smoke path now uses fresh per-run customer ids; explicit ids are only needed for manual debugging:

```bash
STRIPE_SECRET_KEY='sk_test_replace_me_if_provider_missing' \
LAGO_WEBHOOK_URL='https://api-staging.sagarwaidande.org/internal/lago/webhooks' \
make lago-staging-bootstrap-payments
```

What this bootstrap does:
- ensures a Stripe provider exists in Lago with code `stripe_test`
- ensures the alpha webhook endpoint exists in Lago with `jwt` signing
- ensures a success fixture customer exists, has Stripe billing config, has a usable Stripe payment method, and has a billing address synced to Stripe
- ensures a failure fixture customer exists and intentionally has no default payment method so failed retry behavior remains deterministic

Notes:
- `STRIPE_SECRET_KEY` is only required if the `stripe_test` provider does not already exist or is missing a secret key
- for the default failure fixture, the customer is left without a payment method on purpose; this exercises the failed retry path without requiring an interactive Stripe Checkout session

Prepare fixture invoice first:

```bash
LAGO_API_URL='https://lago-api.staging.example.com' \
LAGO_API_KEY='...' \
CUSTOMER_EXTERNAL_ID='cust_e2e_001' \
ADD_ON_CODE='alpha-real-payment-fixture' \
UNIT_AMOUNT_CENTS='199' \
FINALIZE_INVOICE='1' \
REQUIRE_STRIPE_BILLING_CONFIG='1' \
bash ./scripts/prepare_real_payment_invoice_fixture.sh
```

Then run payment E2E:

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

To run the clean staging payment smoke in one command:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
PLATFORM_ADMIN_API_KEY='...' \
ALPHA_WRITER_API_KEY='...' \
ALPHA_READER_API_KEY='...' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='...' \
TARGET_TENANT_ID='default' \
bash ./scripts/run_clean_staging_payment_smoke.sh
```

Or use the self-contained staging wrapper that mints the required Alpha keys automatically:

```bash
LAGO_API_KEY='...' \
make test-staging-payment-smoke
```


The clean smoke now also patches the target Alpha tenant billing mapping before asserting webhook convergence:
- `lago_organization_id`
- `lago_billing_provider_code`

To run the full staging alpha acceptance gate in one command:

```bash
ALPHA_API_BASE_URL='https://api-staging.sagarwaidande.org' \
ALPHA_WRITER_API_KEY='...' \
ALPHA_READER_API_KEY='...' \
LAGO_API_URL='https://lago-api-staging.sagarwaidande.org' \
LAGO_API_KEY='...' \
SUCCESS_INVOICE_ID='56251c97-597a-4cec-9a22-8106d746def8' \
FAILURE_INVOICE_ID='baa27549-32d4-47cd-9f14-d98b61c8b0fa' \
bash ./scripts/verify_staging_acceptance.sh
```

## 6) Troubleshooting

- `invoice must be finalized`: finalize/regenerate target invoice first.
- `customer billing provider is not stripe`: ensure the customer has Stripe billing configuration in Lago.
- `timeout waiting for Lago terminal status`: check Stripe provider config, customer payment method, and Lago worker logs.
- `retry-payment failed: status=405 code=invalid_status`: the invoice already reached a terminal state before retry. This is valid for the success path if Lago auto-collected the payment first.
- `export transactions require a customer name and address`: for India-based Stripe accounts, add customer address fields in Lago and ensure they are synced to Stripe before retrying the payment.
- `payment_intent_unexpected_state`: the target customer has no usable Stripe payment method attached. This is expected for the failure smoke fixture customer and not expected for the success smoke fixture customer.
- `timeout waiting for alpha projection convergence`: check Lago -> alpha webhook delivery/signature/tenant mapping.
- `failed lifecycle expectation mismatch`: inspect `/v1/invoice-payment-statuses/{id}/lifecycle` and webhook ordering for the target invoice.
- `expected webhook_type` mismatch: inspect `/v1/invoice-payment-statuses/{id}/events` payload and webhook routing.
