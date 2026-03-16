import type { Page } from "@playwright/test";
import { expect, test } from "@playwright/test";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveAPIBaseURL = process.env.PLAYWRIGHT_LIVE_API_BASE_URL || "";
const livePlatformAPIKey = process.env.PLAYWRIGHT_LIVE_PLATFORM_API_KEY || "";
const liveWriterAPIKey = process.env.PLAYWRIGHT_LIVE_WRITER_API_KEY || "";

async function loginWithAPIKey(page: Page, apiKey: string) {
  await page.goto("/control-plane");

  await expect(page.getByRole("heading", { name: "Primary onboarding journeys" })).toBeVisible();
  await expect(page.getByTestId("session-login-submit")).toBeVisible();

  await page.getByTestId("session-login-api-key").fill(apiKey);
  if (liveAPIBaseURL) {
    await page.getByTestId("session-login-api-base-url").fill(liveAPIBaseURL);
  }
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByTestId("session-logout")).toBeVisible();
}

test.describe("control plane overview live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live overview smoke");
  test.skip(!liveAPIBaseURL, "PLAYWRIGHT_LIVE_API_BASE_URL is required for live overview smoke");

  test("platform admin can view live workspace attention", async ({ page }) => {
    test.skip(!livePlatformAPIKey, "PLAYWRIGHT_LIVE_PLATFORM_API_KEY is required for platform overview smoke");

    await loginWithAPIKey(page, livePlatformAPIKey);

    await expect(page.getByText("Workspaces missing pricing")).toBeVisible();
    await expect(page.getByText("Workspaces missing first customer")).toBeVisible();
    await expect(page.getByText("Workspaces missing billing connection")).toBeVisible();
  });

  test("tenant writer can view live customer attention", async ({ page }) => {
    test.skip(!liveWriterAPIKey, "PLAYWRIGHT_LIVE_WRITER_API_KEY is required for tenant overview smoke");

    await loginWithAPIKey(page, liveWriterAPIKey);

    await expect(page.getByText("Customers waiting on payment setup")).toBeVisible();
    await expect(page.getByText("Customers with billing sync errors")).toBeVisible();
    await expect(page.getByText("Billing-ready customers")).toBeVisible();
  });
});
