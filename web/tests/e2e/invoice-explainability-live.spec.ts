import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveReaderAPIKey = process.env.PLAYWRIGHT_LIVE_READER_API_KEY || "";
const liveInvoiceID = process.env.PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID || "";

async function loginWithAPIKey(page: Page, apiKey: string) {
  await page.goto("/invoice-explainability");

  await expect(page.getByText("Line Item Computation Trace")).toBeVisible();
  await expect(page.getByTestId("session-login-submit")).toBeVisible();

  await page.getByTestId("session-login-api-key").fill(apiKey);
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByTestId("session-logout")).toBeVisible();
}

test.describe("invoice explainability live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live explainability smoke");
  test.skip(!liveReaderAPIKey, "PLAYWRIGHT_LIVE_READER_API_KEY is required for live explainability smoke");
  test.skip(!liveInvoiceID, "PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID is required for live explainability smoke");

  test("reader session can load live explainability for a known invoice", async ({ page }) => {
    await loginWithAPIKey(page, liveReaderAPIKey);

    await page.getByTestId("explainability-invoice-id").fill(liveInvoiceID);
    await page.getByTestId("explainability-load").click();

    await expect(page.getByTestId("explainability-meta-invoice-id")).toContainText(liveInvoiceID);
    await expect(page.getByTestId("explainability-meta-version")).not.toContainText("-");
    await expect(page.getByTestId("explainability-meta-digest")).not.toContainText("-");
  });
});
