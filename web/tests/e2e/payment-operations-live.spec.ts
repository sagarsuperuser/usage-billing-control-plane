import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveBaseURL = process.env.PLAYWRIGHT_LIVE_BASE_URL || "";
const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const liveReaderEmail = process.env.PLAYWRIGHT_LIVE_READER_EMAIL || "";
const liveReaderPassword = process.env.PLAYWRIGHT_LIVE_READER_PASSWORD || "";

test.describe("payment operations live staging", () => {
  test.skip(!liveBaseURL, "PLAYWRIGHT_LIVE_BASE_URL is required for live staging smoke");
  test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required for live staging smoke");

  test("writer session can inspect live payment detail guidance", async ({ page }) => {
    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: "/payments",
    });
    await expect(page.getByText("Payments")).toBeVisible();

    await page.locator('a[href^="/payments/"]').first().click();
    await expect(page).toHaveURL(/\/payments\/.+$/);
    await expect(page.getByText("Retry and recovery")).toBeVisible();

    const visibleActions = await Promise.all([
      page.getByRole("button", { name: "Retry collection" }).count(),
      page.getByRole("link", { name: "Open recovery tools" }).count(),
      page.getByRole("link", { name: "Open customer payment setup" }).count(),
      page.getByRole("link", { name: "Open explainability" }).count(),
    ]);
    expect(visibleActions.some((count) => count > 0)).toBeTruthy();
  });

  test("writer session can open live recovery handoff from payment detail", async ({ page }) => {
    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: "/payments",
    });

    await page.locator('a[href^="/payments/"]').first().click();

    const recoveryLink = page.getByRole("link", { name: "Open recovery tools" });
    const explainabilityLink = page.getByRole("link", { name: "Open explainability" });
    const customerSetupLink = page.getByRole("link", { name: "Open customer payment setup" });

    if (await recoveryLink.count()) {
      await recoveryLink.click();
      await expect(page).toHaveURL(/\/replay-operations(?:\?.*)?$/);
      await expect(page.getByText("Replay + Reprocess Operations")).toBeVisible();
      return;
    }

    if (await explainabilityLink.count()) {
      await explainabilityLink.click();
      await expect(page).toHaveURL(/\/invoice-explainability(?:\?.*)?$/);
      await expect(page.getByText("Invoice Explainability")).toBeVisible();
      return;
    }

    await expect(customerSetupLink).toBeVisible();
    await customerSetupLink.click();
    await expect(page).toHaveURL(/\/customers\/.+#payment-collection$/);
  });

  test("reader session is read-only for retry operations", async ({ page }) => {
    test.skip(!liveReaderEmail || !liveReaderPassword, "PLAYWRIGHT_LIVE_READER_EMAIL and PLAYWRIGHT_LIVE_READER_PASSWORD are required for reader RBAC smoke");

    await loginWithPassword(page, {
      email: liveReaderEmail,
      password: liveReaderPassword,
      nextPath: "/payments",
    });

    await page.locator('a[href^="/payments/"]').first().click();
    await expect(page.getByText("Retry and recovery")).toBeVisible();

    const retryButton = page.getByRole("button", { name: "Retry collection" });
    if (await retryButton.count()) {
      await expect(retryButton).toBeDisabled();
    }

    const visibleActions = await Promise.all([
      page.getByRole("link", { name: "Open recovery tools" }).count(),
      page.getByRole("link", { name: "Open customer payment setup" }).count(),
      page.getByRole("link", { name: "Open explainability" }).count(),
    ]);
    expect(visibleActions.some((count) => count > 0)).toBeTruthy();
  });
});
