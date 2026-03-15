import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveAPIBaseURL = process.env.PLAYWRIGHT_LIVE_API_BASE_URL || "";
const liveWriterAPIKey = process.env.PLAYWRIGHT_LIVE_WRITER_API_KEY || process.env.PLAYWRIGHT_LIVE_API_KEY || "";
const liveReaderAPIKey = process.env.PLAYWRIGHT_LIVE_READER_API_KEY || process.env.PLAYWRIGHT_LIVE_API_KEY || "";
const liveReplayJobID = process.env.PLAYWRIGHT_LIVE_REPLAY_JOB_ID || "";
const liveReplayCustomerID = process.env.PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID || "";
const liveReplayMeterID = process.env.PLAYWRIGHT_LIVE_REPLAY_METER_ID || "";

async function loginWithAPIKey(page: Page, apiKey: string) {
  await page.goto("/replay-operations");

  await expect(page.getByText("Replay + Reprocess Operations")).toBeVisible();
  await expect(page.getByTestId("session-login-submit")).toBeVisible();

  await page.getByTestId("session-login-api-key").fill(apiKey);
  if (liveAPIBaseURL) {
    await page.getByTestId("session-login-api-base-url").fill(liveAPIBaseURL);
  }
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByTestId("session-logout")).toBeVisible();
}

test.describe("replay operations live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live staging replay smoke");
  test.skip(!liveAPIBaseURL, "PLAYWRIGHT_LIVE_API_BASE_URL is required for live staging replay smoke");

  test("reader session can inspect a known live replay fixture", async ({ page }) => {
    test.skip(!liveReaderAPIKey, "PLAYWRIGHT_LIVE_READER_API_KEY is required for live replay diagnostics smoke");
    test.skip(!liveReplayJobID, "PLAYWRIGHT_LIVE_REPLAY_JOB_ID is required for live replay diagnostics smoke");
    test.skip(!liveReplayCustomerID, "PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID is required for live replay diagnostics smoke");
    test.skip(!liveReplayMeterID, "PLAYWRIGHT_LIVE_REPLAY_METER_ID is required for live replay diagnostics smoke");

    await loginWithAPIKey(page, liveReaderAPIKey);

    await page.getByPlaceholder("cust_123").first().fill(liveReplayCustomerID);
    await page.getByPlaceholder("meter_abc").first().fill(liveReplayMeterID);

    await expect(page.getByTestId(`replay-job-row-${liveReplayJobID}`)).toBeVisible({ timeout: 15000 });
    await page.getByTestId(`replay-open-diagnostics-${liveReplayJobID}`).click();

    await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();
    await expect(page.getByText(liveReplayJobID)).toBeVisible();
    await expect(page.getByText("Usage events")).toBeVisible();
    await expect(page.getByText("Billed entries")).toBeVisible();
    await expect(page.getByText("Report JSON")).toBeVisible();
  });

  test("writer session can queue a fresh replay job from the live replay screen", async ({ page }) => {
    test.skip(!liveWriterAPIKey, "PLAYWRIGHT_LIVE_WRITER_API_KEY is required for live replay queue smoke");
    test.skip(!liveReplayCustomerID, "PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID is required for live replay queue smoke");
    test.skip(!liveReplayMeterID, "PLAYWRIGHT_LIVE_REPLAY_METER_ID is required for live replay queue smoke");

    await loginWithAPIKey(page, liveWriterAPIKey);

    const idempotencyKey = `playwright-live-replay-${Date.now()}`;
    await page.getByTestId("replay-create-customer-id").fill(liveReplayCustomerID);
    await page.getByTestId("replay-create-meter-id").fill(liveReplayMeterID);
    await page.getByTestId("replay-create-idempotency-key").fill(idempotencyKey);
    await page.getByTestId("replay-create-submit").click();

    await expect(page.getByTestId("replay-flash-message")).toContainText(/Replay job .* queued for customer/i);
    await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();
    await expect(page.getByText("Replay Diagnostics")).toBeVisible();
    await expect(page.getByText("Workflow telemetry")).toBeVisible();
  });
});
