import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const livePlatformAdminEmail = process.env.PLAYWRIGHT_LIVE_PLATFORM_ADMIN_EMAIL || process.env.PLAYWRIGHT_LIVE_PLATFORM_EMAIL || "";
const livePlatformAdminPassword = process.env.PLAYWRIGHT_LIVE_PLATFORM_ADMIN_PASSWORD || process.env.PLAYWRIGHT_LIVE_PLATFORM_PASSWORD || "";
const workspaceID = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_ID || "";
const workspaceName = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_WORKSPACE_NAME || "";
const primaryConnectionName = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_PRIMARY_NAME || "";
const secondaryConnectionName = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_SECONDARY_NAME || "";
const lagoOrganizationID = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_LAGO_ORGANIZATION_ID || "";
const stripeSecretKey = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY || "";
const rotatedStripeSecretKey = process.env.PLAYWRIGHT_LIVE_BILLING_CONNECTION_ROTATED_STRIPE_SECRET_KEY || stripeSecretKey;

async function createConnection(page: import("@playwright/test").Page, name: string, secretKey: string): Promise<string> {
  await page.goto("/billing-connections/new");
  await expect(page).toHaveURL(/\/billing-connections\/new(?:\?.*)?$/);
  await expect(page.getByRole("heading", { name: "New billing connection" })).toBeVisible();
  await page.waitForLoadState("networkidle");
  const nameInput = page.getByLabel("Connection name");
  const secretInput = page.getByLabel("Stripe secret key");
  const submitButton = page.getByRole("button", { name: "Create and sync connection" });
  const nameChecklist = page.locator("div").filter({ hasText: /Connection name is set/ }).first();
  const secretChecklist = page.locator("div").filter({ hasText: /Stripe secret key is set/ }).first();
  await expect(nameInput).toBeEditable();
  await nameInput.click();
  await nameInput.pressSequentially(name);
  await expect(nameInput).toHaveValue(name);
  await secretInput.click();
  await secretInput.pressSequentially(secretKey);
  await expect(secretInput).toHaveValue(secretKey);
  if (lagoOrganizationID) {
    const orgOverrideInput = page.getByLabel("Billing organization override");
    await orgOverrideInput.click();
    await orgOverrideInput.pressSequentially(lagoOrganizationID);
    await expect(orgOverrideInput).toHaveValue(lagoOrganizationID);
  }
  await expect(nameChecklist).toContainText("OK");
  await expect(secretChecklist).toContainText("OK");
  await expect(submitButton).toBeEnabled();
  await submitButton.click();

  await expect(page).toHaveURL(/\/billing-connections\/[^/?]+(?:\?.*)?$/);
  await expect(page.getByRole("heading", { name })).toBeVisible({ timeout: 60000 });
  await expect(page.getByText(/^connected$/i).first()).toBeVisible({ timeout: 60000 });
  await expect(page.locator("div").filter({ hasText: /^Connected and ready for workspace assignment\.$/ }).first()).toBeVisible({ timeout: 60000 });

  const id = decodeURIComponent(page.url().split("/").pop()?.split("?")[0] || "");
  expect(id).not.toBe("");
  return id;
}

test.describe("billing connection lifecycle live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live billing connection lifecycle journey");
  test.skip(!livePlatformAdminEmail || !livePlatformAdminPassword, "PLAYWRIGHT_LIVE_PLATFORM_EMAIL and PLAYWRIGHT_LIVE_PLATFORM_PASSWORD are required");

  test("platform admin can create, rotate, switch, and disable billing connections without backend repair", async ({ page }) => {
    test.setTimeout(300000);
    test.skip(!workspaceID || !workspaceName, "workspace identity envs are required");
    test.skip(!primaryConnectionName || !secondaryConnectionName, "connection name envs are required");
    test.skip(!stripeSecretKey, "PLAYWRIGHT_LIVE_BILLING_CONNECTION_STRIPE_SECRET_KEY is required");

    await loginWithPassword(page, {
      email: livePlatformAdminEmail,
      password: livePlatformAdminPassword,
      nextPath: "/billing-connections/new",
    });

    const primaryConnectionID = await createConnection(page, primaryConnectionName, stripeSecretKey);

    await page.goto("/workspaces/new");
    await expect(page.getByRole("heading", { name: "Create workspace" })).toBeVisible();
    await page.waitForLoadState("networkidle");
    const workspaceIDInput = page.getByLabel("Workspace ID");
    const workspaceNameInput = page.getByLabel("Workspace name");
    const billingConnectionSelect = page.getByLabel("Billing connection");
    const runWorkspaceSetupButton = page.getByRole("button", { name: "Run workspace setup" });
    const workspaceIDChecklist = page.locator("div").filter({ hasText: /Workspace ID is set/ }).first();
    const workspaceNameChecklist = page.locator("div").filter({ hasText: /Workspace name is set/ }).first();
    await expect(workspaceIDInput).toBeEditable();
    await workspaceIDInput.click();
    await workspaceIDInput.pressSequentially(workspaceID);
    await expect(workspaceIDInput).toHaveValue(workspaceID);
    await workspaceNameInput.click();
    await workspaceNameInput.pressSequentially(workspaceName);
    await expect(workspaceNameInput).toHaveValue(workspaceName);
    await billingConnectionSelect.selectOption(primaryConnectionID);
    await expect(workspaceIDChecklist).toContainText("OK");
    await expect(workspaceNameChecklist).toContainText("OK");
    await expect(runWorkspaceSetupButton).toBeEnabled();
    await runWorkspaceSetupButton.click();

    await expect(page.getByText(new RegExp(`Workspace ${workspaceID} created successfully\\.|Workspace ${workspaceID} updated and readiness refreshed\\.`))).toBeVisible({ timeout: 60000 });
    await expect(page.getByText(primaryConnectionID).first()).toBeVisible();
    await page.getByRole("link", { name: "View workspace detail" }).click();

    await expect(page).toHaveURL(new RegExp(`/workspaces/${workspaceID}(?:\\?.*)?$`));
    await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
    await expect(page.getByText(primaryConnectionID).first()).toBeVisible();
    await expect(page.getByText(primaryConnectionName).first()).toBeVisible();

    const secondaryConnectionID = await createConnection(page, secondaryConnectionName, stripeSecretKey);

    const rotatedSecretInput = page.getByLabel("New Stripe secret key");
    const rotateSecretButton = page.getByRole("button", { name: "Rotate secret" });
    await expect(rotatedSecretInput).toBeEditable();
    await rotatedSecretInput.click();
    await rotatedSecretInput.pressSequentially(rotatedStripeSecretKey);
    await expect(rotatedSecretInput).toHaveValue(rotatedStripeSecretKey);
    await expect(rotateSecretButton).toBeEnabled();
    await rotateSecretButton.click();
    await expect(page.getByText("Connection is waiting for a successful provider sync.")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/^Pending$/)).toBeVisible();

    await page.getByRole("button", { name: "Sync now" }).click();
    await expect(page.locator("div").filter({ hasText: /^Connected and ready for workspace assignment\.$/ }).first()).toBeVisible({ timeout: 60000 });
    await expect(page.getByText(/^Connected$/)).toBeVisible();

    await page.goto(`/workspaces/${encodeURIComponent(workspaceID)}`);
    await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
    await page.getByLabel("Active billing connection").selectOption(secondaryConnectionID);
    await page.getByRole("button", { name: "Save active connection" }).click();
    await expect(page.getByText(secondaryConnectionID).first()).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(secondaryConnectionName).first()).toBeVisible();

    await page.goto(`/billing-connections/${encodeURIComponent(primaryConnectionID)}`);
    await expect(page.getByRole("heading", { name: primaryConnectionName })).toBeVisible();
    await page.getByRole("button", { name: "Disable connection" }).click();
    await expect(page.getByText("Connection is disabled and cannot be assigned to new workspaces.")).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/^Disabled$/)).toBeVisible();

    await page.goto(`/workspaces/${encodeURIComponent(workspaceID)}`);
    await expect(page.getByRole("heading", { name: workspaceName })).toBeVisible();
    await expect(page.getByText(secondaryConnectionID).first()).toBeVisible();
    await expect(page.getByText(secondaryConnectionName).first()).toBeVisible();
  });
});
