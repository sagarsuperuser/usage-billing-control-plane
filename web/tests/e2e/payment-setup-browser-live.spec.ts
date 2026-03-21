import { execFileSync } from "node:child_process";
import path from "node:path";

import { expect, test, type Page } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const livePaymentSetupInvoiceID = process.env.PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID || "";
const livePaymentSetupCustomerExternalID = process.env.PLAYWRIGHT_LIVE_PAYMENT_SETUP_CUSTOMER_EXTERNAL_ID || "";
const livePaymentSetupLagoOrgID = process.env.PLAYWRIGHT_LIVE_PAYMENT_SETUP_LAGO_ORG_ID || "";
const livePaymentSetupStripeProviderCode = process.env.PLAYWRIGHT_LIVE_PAYMENT_SETUP_STRIPE_PROVIDER_CODE || "";
const lagoAPIKey = process.env.LAGO_API_KEY || "";
const paymentMethodFixture = process.env.PAYMENT_METHOD_FIXTURE || "pm_card_visa";
const repoRoot = path.resolve(process.cwd(), "..");

async function resolveAPIBaseURL(page: Page): Promise<string> {
  const runtimeConfigURL = new URL("/runtime-config", page.url()).toString();
  const runtimeResponse = await page.context().request.get(runtimeConfigURL, {
    failOnStatusCode: true,
  });
  const runtimePayload = (await runtimeResponse.json()) as { apiBaseURL?: string } | null;
  return (runtimePayload?.apiBaseURL ?? new URL(page.url()).origin).replace(/\/+$/, "");
}

async function waitForCustomerReadiness(page: Page, customerExternalID: string, expectedStatus: string) {
  const apiBaseURL = await resolveAPIBaseURL(page);
  await expect
    .poll(
      async () => {
        const response = await page.context().request.get(
          `${apiBaseURL}/v1/customers/${encodeURIComponent(customerExternalID)}/readiness`,
          {
            failOnStatusCode: false,
          },
        );
        if (!response.ok()) {
          return { ok: false, status: response.status() };
        }
        const payload = (await response.json()) as { status?: string; payment_setup_status?: string; default_payment_method_verified?: boolean };
        return {
          ok: true,
          status: payload.status ?? null,
          payment_setup_status: payload.payment_setup_status ?? null,
          default_payment_method_verified: payload.default_payment_method_verified ?? false,
        };
      },
      { timeout: 120000, intervals: [1000, 2000, 3000, 5000] },
    )
    .toMatchObject({ ok: true, status: expectedStatus, payment_setup_status: expectedStatus, default_payment_method_verified: true });
}

async function waitForPaymentStatus(page: Page, invoiceID: string, expectedStatus: string, expectedAction: string) {
  const apiBaseURL = await resolveAPIBaseURL(page);
  await expect
    .poll(
      async () => {
        const response = await page.context().request.get(
          `${apiBaseURL}/v1/payments/${encodeURIComponent(invoiceID)}`,
          {
            failOnStatusCode: false,
          },
        );
        if (!response.ok()) {
          return { ok: false, status: response.status() };
        }
        const payload = (await response.json()) as { payment_status?: string; lifecycle?: { recommended_action?: string; requires_action?: boolean } };
        return {
          ok: true,
          payment_status: payload.payment_status ?? null,
          recommended_action: payload.lifecycle?.recommended_action ?? null,
          requires_action: payload.lifecycle?.requires_action ?? null,
        };
      },
      { timeout: 120000, intervals: [1000, 2000, 3000, 5000] },
    )
    .toMatchObject({ ok: true, payment_status: expectedStatus, recommended_action: expectedAction });
}

function reconcileProviderPaymentMethod() {
  execFileSync("bash", ["./scripts/reconcile_lago_stripe_customer_payment_method.sh"], {
    cwd: repoRoot,
    stdio: "inherit",
    env: {
      ...process.env,
      LAGO_ORG_ID: livePaymentSetupLagoOrgID,
      STRIPE_PROVIDER_CODE: livePaymentSetupStripeProviderCode,
      CUSTOMER_EXTERNAL_ID: livePaymentSetupCustomerExternalID,
      PAYMENT_METHOD_ACTION: "attach_default",
      PAYMENT_METHOD_FIXTURE: paymentMethodFixture,
    },
  });
}

test.describe("browser payment setup live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live browser payment-setup journey");
  test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required");

  test("writer can complete collect-payment recovery through the UI", async ({ page }) => {
    test.setTimeout(300000);
    test.skip(!livePaymentSetupInvoiceID, "PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID is required");
    test.skip(!livePaymentSetupCustomerExternalID, "PLAYWRIGHT_LIVE_PAYMENT_SETUP_CUSTOMER_EXTERNAL_ID is required");
    test.skip(!livePaymentSetupLagoOrgID || !livePaymentSetupStripeProviderCode, "browser payment-setup journey needs Lago organization and Stripe provider envs");
    test.skip(!lagoAPIKey, "LAGO_API_KEY is required for deterministic provider-side completion");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: `/payments/${livePaymentSetupInvoiceID}`,
    });

    await expect(page).toHaveURL(new RegExp(`/payments/${livePaymentSetupInvoiceID}(?:\\?.*)?$`));
    await expect(page.getByText("Retry and recovery")).toBeVisible();
    const customerSetupLink = page.getByRole("link", { name: "Open customer payment setup" });
    await expect(customerSetupLink).toBeVisible();
    await customerSetupLink.click();

    await expect(page).toHaveURL(/\/customers\/.+#payment-collection$/);
    await expect(page.getByText("Payment collection")).toBeVisible();
    await expect(page.getByRole("button", { name: "Send payment setup request" })).toBeVisible();

    await page.getByRole("button", { name: "Send payment setup request" }).click();
    await expect(page.getByText("Payment setup request sent")).toBeVisible();

    reconcileProviderPaymentMethod();

    await page.reload();
    const refreshButton = page.getByRole("button", { name: "Refresh payment setup" });
    await expect(refreshButton).toBeEnabled({ timeout: 30000 });
    await refreshButton.click();
    await waitForCustomerReadiness(page, livePaymentSetupCustomerExternalID, "ready");
    await page.reload();
    await expect(page.getByText("Customer is ready for payment operations.")).toBeVisible();

    await page.goto(`/payments/${livePaymentSetupInvoiceID}`);
    await expect(page.getByText("Retry and recovery")).toBeVisible();
    const retryCollectionButton = page.getByRole("button", { name: "Retry collection" });
    await expect(retryCollectionButton).toBeVisible();
    await retryCollectionButton.click();

    await waitForPaymentStatus(page, livePaymentSetupInvoiceID, "succeeded", "none");
    await page.reload();
    await expect(page.getByText("No action required")).toBeVisible();
    await expect(page.getByText("Payment succeeded. No collection action required.").first()).toBeVisible();
  });
});
