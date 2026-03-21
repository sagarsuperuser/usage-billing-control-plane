import { expect, test, type Browser, type Page } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const livePlatformEmail = process.env.PLAYWRIGHT_LIVE_PLATFORM_EMAIL || "";
const livePlatformPassword = process.env.PLAYWRIGHT_LIVE_PLATFORM_PASSWORD || "";
const liveTenantID = process.env.PLAYWRIGHT_LIVE_TENANT_ID || "";
const invitedAdminEmail = process.env.PLAYWRIGHT_LIVE_ACCESS_ADMIN_INVITE_EMAIL || "";
const invitedAdminDisplayName = process.env.PLAYWRIGHT_LIVE_ACCESS_ADMIN_INVITE_DISPLAY_NAME || "";
const invitedAdminPassword = process.env.PLAYWRIGHT_LIVE_ACCESS_ADMIN_INVITE_PASSWORD || "";
const invitedWriterEmail = process.env.PLAYWRIGHT_LIVE_ACCESS_WRITER_INVITE_EMAIL || "";
const invitedWriterDisplayName = process.env.PLAYWRIGHT_LIVE_ACCESS_WRITER_INVITE_DISPLAY_NAME || "";
const invitedWriterPassword = process.env.PLAYWRIGHT_LIVE_ACCESS_WRITER_INVITE_PASSWORD || "";

async function createIsolatedPage(browser: Browser): Promise<Page> {
  const context = await browser.newContext({
    baseURL: process.env.PLAYWRIGHT_LIVE_BASE_URL,
  });
  return context.newPage();
}

async function extractLatestInviteURL(page: Page): Promise<string> {
  const inviteLocator = page.locator("p").filter({ hasText: /\/invite\// }).last();
  await expect(inviteLocator).toBeVisible();
  const inviteURL = (await inviteLocator.textContent())?.trim() ?? "";
  expect(inviteURL).toContain("/invite/");
  return inviteURL;
}

async function registerFromInvite(page: Page, inviteURL: string, displayName: string, password: string) {
  await page.goto(inviteURL);
  await expect(page.getByRole("heading", { name: /Join / })).toBeVisible();
  await page.getByPlaceholder("Your name").fill(displayName);
  await page.getByPlaceholder("At least 12 characters").fill(password);
  await Promise.all([
    page.waitForURL(/\/customers(?:\?.*)?$/),
    page.getByRole("button", { name: "Create account and join workspace" }).click(),
  ]);
}

test.describe("access and invite live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live access journey");
  test.skip(!livePlatformEmail || !livePlatformPassword, "PLAYWRIGHT_LIVE_PLATFORM_EMAIL and PLAYWRIGHT_LIVE_PLATFORM_PASSWORD are required");

  test("platform and tenant admins can hand off workspace access through live invitations", async ({ page, browser }) => {
    test.setTimeout(300000);
    test.skip(!liveTenantID, "PLAYWRIGHT_LIVE_TENANT_ID is required");
    test.skip(!invitedAdminEmail || !invitedAdminDisplayName || !invitedAdminPassword, "admin invite env is required");
    test.skip(!invitedWriterEmail || !invitedWriterDisplayName || !invitedWriterPassword, "writer invite env is required");

    await loginWithPassword(page, {
      email: livePlatformEmail,
      password: livePlatformPassword,
      nextPath: `/workspaces/${liveTenantID}`,
    });

    await expect(page).toHaveURL(new RegExp(`/workspaces/${liveTenantID}(?:\\?.*)?$`));
    await expect(page.getByRole("heading", { name: liveTenantID })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Members and pending invites" })).toBeVisible();
    await expect(page.getByText("Last active admin")).toBeVisible();
    await page.getByPlaceholder("tenant-admin@example.com").fill(invitedAdminEmail);
    await page.getByLabel("Workspace role").selectOption("admin");
    await page.getByRole("button", { name: "Send invite" }).click();
    await expect(page.getByText(invitedAdminEmail)).toBeVisible();
    const adminInviteURL = await extractLatestInviteURL(page);

    const adminPage = await createIsolatedPage(browser);
    try {
      await registerFromInvite(adminPage, adminInviteURL, invitedAdminDisplayName, invitedAdminPassword);
      await adminPage.goto("/workspace-access");
      await expect(adminPage.getByRole("heading", { name: "Members, invitations, and machine credentials" })).toBeVisible();
      await expect(adminPage.getByText("You cannot change your own membership from this screen.")).toBeVisible();

      await adminPage.getByPlaceholder("teammate@example.com").fill(invitedWriterEmail);
      await adminPage.getByLabel("Workspace role").selectOption("writer");
      await adminPage.getByRole("button", { name: "Send invite" }).click();
      await expect(adminPage.getByText(invitedWriterEmail)).toBeVisible();
      const writerInviteURL = await extractLatestInviteURL(adminPage);

      const writerPage = await createIsolatedPage(browser);
      try {
        await registerFromInvite(writerPage, writerInviteURL, invitedWriterDisplayName, invitedWriterPassword);
        await writerPage.goto("/workspace-access");
        await expect(writerPage.getByText("Workspace admin role required")).toBeVisible();
        await expect(writerPage.getByText(/Only workspace admins can manage invitations, service accounts, roles, and member removal\./)).toBeVisible();
      } finally {
        await writerPage.context().close();
      }

      await adminPage.goto("/workspace-access");
      await expect(adminPage.getByText("No pending workspace invites.")).toBeVisible();
      await expect(adminPage.getByText(invitedWriterEmail)).toBeVisible();
    } finally {
      await adminPage.context().close();
    }
  });
});
