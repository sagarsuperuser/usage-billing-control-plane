import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveAPIBaseURL = process.env.PLAYWRIGHT_LIVE_API_BASE_URL || "";
const liveWriterAPIKey = process.env.PLAYWRIGHT_LIVE_WRITER_API_KEY || process.env.PLAYWRIGHT_LIVE_API_KEY || "";
const liveReaderAPIKey = process.env.PLAYWRIGHT_LIVE_READER_API_KEY || "";

async function loginWithAPIKey(page: Page, apiKey: string) {
  await page.goto("/payment-operations");

  await expect(page.getByText("Failed Payment Triage")).toBeVisible();
  await expect(page.getByTestId("session-login-submit")).toBeVisible();

  await page.getByTestId("session-login-api-key").fill(apiKey);
  if (liveAPIBaseURL) {
    await page.getByTestId("session-login-api-base-url").fill(liveAPIBaseURL);
  }
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByTestId("session-logout")).toBeVisible();
}

test.describe("payment operations live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live staging smoke");
  test.skip(!liveWriterAPIKey, "PLAYWRIGHT_LIVE_WRITER_API_KEY is required for live staging smoke");
  test.skip(!liveAPIBaseURL, "PLAYWRIGHT_LIVE_API_BASE_URL is required for live staging smoke");

  test("writer session can submit retry from live payment ops UI", async ({ page }) => {
    await loginWithAPIKey(page, liveWriterAPIKey);
    await expect(page.getByRole("button", { name: "Failed" })).toBeVisible();

    await page.getByRole("button", { name: "Failed" }).click();

    const retryButton = page.getByRole("button", { name: "Retry" }).first();
    await expect(retryButton).toBeVisible();
    await retryButton.click();

    await expect(page.getByText(/Retry request sent to billing engine for invoice/i)).toBeVisible();
  });

  test("writer session can open live invoice timeline drawer", async ({ page }) => {
    await loginWithAPIKey(page, liveWriterAPIKey);

    const timelineButton = page.getByRole("button", { name: "Timeline" }).first();
    await expect(timelineButton).toBeVisible();
    await timelineButton.click();

    await expect(page.getByText("Invoice Timeline")).toBeVisible();
    await expect(page.getByRole("button", { name: "Refresh" }).first()).toBeVisible();
  });

  test("reader session is read-only for retry operations", async ({ page }) => {
    test.skip(!liveReaderAPIKey, "PLAYWRIGHT_LIVE_READER_API_KEY is required for reader RBAC smoke");

    await loginWithAPIKey(page, liveReaderAPIKey);

    await expect(page.getByRole("button", { name: "Failed" })).toBeVisible();
    await page.getByRole("button", { name: "Failed" }).click();

    const retryButton = page.getByRole("button", { name: "Retry" }).first();
    await expect(retryButton).toBeDisabled();
    await expect(page.getByText(/read-only for payment retry operations/i)).toBeVisible();

    const timelineButton = page.getByRole("button", { name: "Timeline" }).first();
    await expect(timelineButton).toBeVisible();
  });
});
