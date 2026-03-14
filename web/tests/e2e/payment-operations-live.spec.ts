import { expect, test } from "@playwright/test";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveAPIKey = process.env.PLAYWRIGHT_LIVE_API_KEY || "";

test.describe("payment operations live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live staging smoke");
  test.skip(!liveAPIKey, "PLAYWRIGHT_LIVE_API_KEY is required for live staging smoke");

  test("supports live session login and failed-payment retry submission", async ({ page }) => {
    await page.goto("/payment-operations");

    await expect(page.getByText("Failed Payment Triage")).toBeVisible();
    await expect(page.getByTestId("session-login-submit")).toBeVisible();

    await page.getByTestId("session-login-api-key").fill(liveAPIKey);
    await page.getByTestId("session-login-submit").click();

    await expect(page.getByTestId("session-logout")).toBeVisible();
    await expect(page.getByRole("button", { name: "Failed" })).toBeVisible();

    await page.getByRole("button", { name: "Failed" }).click();

    const retryButton = page.getByRole("button", { name: "Retry" }).first();
    await expect(retryButton).toBeVisible();
    await retryButton.click();

    await expect(page.getByText(/Retry request sent to billing engine for invoice/i)).toBeVisible();
  });

  test("opens live invoice timeline drawer", async ({ page }) => {
    await page.goto("/payment-operations");

    await page.getByTestId("session-login-api-key").fill(liveAPIKey);
    await page.getByTestId("session-login-submit").click();

    await expect(page.getByTestId("session-logout")).toBeVisible();

    const timelineButton = page.getByRole("button", { name: "Timeline" }).first();
    await expect(timelineButton).toBeVisible();
    await timelineButton.click();

    await expect(page.getByText("Invoice Timeline")).toBeVisible();
    await expect(page.getByRole("button", { name: "Refresh" }).first()).toBeVisible();
  });
});
