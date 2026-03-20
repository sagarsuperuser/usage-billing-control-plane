import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const livePlatformEmail = process.env.PLAYWRIGHT_LIVE_PLATFORM_EMAIL || "";
const livePlatformPassword = process.env.PLAYWRIGHT_LIVE_PLATFORM_PASSWORD || "";
const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";

test.describe("control plane overview live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live overview smoke");

  test("platform admin can view live workspace attention", async ({ page }) => {
    test.skip(!livePlatformEmail || !livePlatformPassword, "PLAYWRIGHT_LIVE_PLATFORM_EMAIL and PLAYWRIGHT_LIVE_PLATFORM_PASSWORD are required for platform overview smoke");

    await loginWithPassword(page, {
      email: livePlatformEmail,
      password: livePlatformPassword,
      nextPath: "/control-plane",
    });

    await expect(page.getByText("Workspaces missing pricing")).toBeVisible();
    await expect(page.getByText("Workspaces missing first customer")).toBeVisible();
    await expect(page.getByText("Workspaces missing billing connection")).toBeVisible();
  });

  test("tenant writer can view live customer attention", async ({ page }) => {
    test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required for tenant overview smoke");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: "/control-plane",
    });

    await expect(page.getByText("Customers waiting on payment setup")).toBeVisible();
    await expect(page.getByText("Customers with billing sync errors")).toBeVisible();
    await expect(page.getByText("Billing-ready customers")).toBeVisible();
  });
});
