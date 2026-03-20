import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveReaderEmail = process.env.PLAYWRIGHT_LIVE_READER_EMAIL || "";
const liveReaderPassword = process.env.PLAYWRIGHT_LIVE_READER_PASSWORD || "";
const liveInvoiceID = process.env.PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID || "";

test.describe("invoice explainability live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live explainability smoke");
  test.skip(!liveReaderEmail || !liveReaderPassword, "PLAYWRIGHT_LIVE_READER_EMAIL and PLAYWRIGHT_LIVE_READER_PASSWORD are required for live explainability smoke");
  test.skip(!liveInvoiceID, "PLAYWRIGHT_LIVE_EXPLAINABILITY_INVOICE_ID is required for live explainability smoke");

  test("reader session can load live explainability for a known invoice", async ({ page }) => {
    await loginWithPassword(page, {
      email: liveReaderEmail,
      password: liveReaderPassword,
      nextPath: "/invoice-explainability",
    });

    await page.getByTestId("explainability-invoice-id").fill(liveInvoiceID);
    await page.getByTestId("explainability-load").click();

    await expect(page.getByTestId("explainability-meta-invoice-id")).toContainText(liveInvoiceID);
    await expect(page.getByTestId("explainability-meta-version")).not.toContainText("-");
    await expect(page.getByTestId("explainability-meta-digest")).not.toContainText("-");
  });
});
