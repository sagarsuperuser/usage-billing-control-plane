import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const livePaymentSetupInvoiceID = process.env.PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID || "";

test.describe("browser payment setup live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live browser payment-setup journey");
  test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required");

  test("writer can follow collect-payment handoff and send payment setup request", async ({ page }) => {
    test.skip(!livePaymentSetupInvoiceID, "PLAYWRIGHT_LIVE_PAYMENT_SETUP_INVOICE_ID is required");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: `/payments/${livePaymentSetupInvoiceID}`,
    });

    await expect(page).toHaveURL(new RegExp(`/payments/${livePaymentSetupInvoiceID}(?:\\?.*)?$`));
    await expect(page.getByText("Retry and recovery")).toBeVisible();
    await expect(page.getByText(/Collect payment/i)).toBeVisible();

    const customerSetupLink = page.getByRole("link", { name: "Open customer payment setup" });
    await expect(customerSetupLink).toBeVisible();
    await customerSetupLink.click();

    await expect(page).toHaveURL(/\/customers\/.+#payment-collection$/);
    await expect(page.getByText("Payment collection")).toBeVisible();
    await expect(page.getByRole("button", { name: "Send payment setup request" })).toBeVisible();

    await page.getByRole("button", { name: "Send payment setup request" }).click();

    await expect(page.getByText("Payment setup request sent")).toBeVisible();
    await expect(page.getByText(/Sent to .+ on /)).toBeVisible();

    const sentLink = page.getByRole("link", { name: "Open latest sent link" });
    if (await sentLink.count()) {
      await expect(sentLink).toBeVisible();
    }
  });
});
