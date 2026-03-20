import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const liveReaderEmail = process.env.PLAYWRIGHT_LIVE_READER_EMAIL || "";
const liveReaderPassword = process.env.PLAYWRIGHT_LIVE_READER_PASSWORD || "";
const liveReplayJobID = process.env.PLAYWRIGHT_LIVE_REPLAY_JOB_ID || "";
const liveReplayCustomerID = process.env.PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID || "";
const liveReplayMeterID = process.env.PLAYWRIGHT_LIVE_REPLAY_METER_ID || "";

test.describe("replay operations live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live staging replay smoke");

  test("reader session can inspect a known live replay fixture", async ({ page }) => {
    test.skip(!liveReaderEmail || !liveReaderPassword, "PLAYWRIGHT_LIVE_READER_EMAIL and PLAYWRIGHT_LIVE_READER_PASSWORD are required for live replay diagnostics smoke");
    test.skip(!liveReplayJobID, "PLAYWRIGHT_LIVE_REPLAY_JOB_ID is required for live replay diagnostics smoke");
    test.skip(!liveReplayCustomerID, "PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID is required for live replay diagnostics smoke");
    test.skip(!liveReplayMeterID, "PLAYWRIGHT_LIVE_REPLAY_METER_ID is required for live replay diagnostics smoke");

    await loginWithPassword(page, {
      email: liveReaderEmail,
      password: liveReaderPassword,
      nextPath: "/replay-operations",
    });

    await page.getByPlaceholder("cust_123").first().fill(liveReplayCustomerID);
    await page.getByPlaceholder("meter_abc").first().fill(liveReplayMeterID);

    await expect(page.getByTestId(`replay-job-row-${liveReplayJobID}`)).toBeVisible({ timeout: 15000 });
    await page.getByTestId(`replay-open-diagnostics-${liveReplayJobID}`).click();

    await expect(page.getByTestId("replay-diagnostics-drawer")).toBeVisible();
    await expect(page.getByRole("heading", { name: liveReplayJobID })).toBeVisible();
    await expect(page.getByText("Usage events")).toBeVisible();
    await expect(page.getByText("Billed entries")).toBeVisible();
    await expect(page.getByText("Workflow telemetry")).toBeVisible();
  });

  test("writer session can queue a fresh replay job from the live replay screen", async ({ page }) => {
    test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required for live replay queue smoke");
    test.skip(!liveReplayCustomerID, "PLAYWRIGHT_LIVE_REPLAY_CUSTOMER_ID is required for live replay queue smoke");
    test.skip(!liveReplayMeterID, "PLAYWRIGHT_LIVE_REPLAY_METER_ID is required for live replay queue smoke");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: "/replay-operations",
    });

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
