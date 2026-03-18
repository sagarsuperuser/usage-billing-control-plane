import { expect, test, type Page } from "@playwright/test";

type PlatformSessionPayload = {
  authenticated: boolean;
  subject_type: "user";
  subject_id: string;
  user_email: string;
  scope: "platform";
  platform_role: "platform_admin";
  csrf_token: string;
};

type TenantSessionPayload = {
  authenticated: boolean;
  subject_type: "user";
  subject_id: string;
  user_email: string;
  scope: "tenant";
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  csrf_token: string;
};

async function installLoginMock(page: Page, session: PlatformSessionPayload | TenantSessionPayload) {
  let loggedIn = false;

  const jsonRoute = async (pattern: string, handler: Parameters<Page["route"]>[1]) => {
    await page.route(pattern, handler);
  };

  await jsonRoute("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });
  await jsonRoute("**/v1/ui/auth/providers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ password_enabled: true, sso_providers: [] }),
    });
  });
  await jsonRoute("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: loggedIn ? 200 : 401,
      contentType: "application/json",
      body: JSON.stringify(loggedIn ? session : { error: "unauthorized" }),
    });
  });
  await jsonRoute("**/v1/ui/sessions/login", async (route) => {
    const body = route.request().postDataJSON() as { email?: string; password?: string };
    await route.fulfill({
      status: body?.email && body?.password ? 201 : 400,
      contentType: "application/json",
      body: JSON.stringify(body?.email && body?.password ? session : { error: "email and password are required" }),
    });
    if (body?.email && body?.password) {
      loggedIn = true;
    }
  });
  await jsonRoute("**/internal/billing-provider-connections", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: [] }),
    });
  });
  await jsonRoute("**/v1/customers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });
}

async function installWorkspaceSelectionLoginMock(page: Page) {
  let pendingSelection = false;
  let selectedSession: TenantSessionPayload | null = null;

  await page.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });
  await page.route("**/v1/ui/auth/providers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ password_enabled: true, sso_providers: [] }),
    });
  });
  await page.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: selectedSession ? 200 : 401,
      contentType: "application/json",
      body: JSON.stringify(selectedSession ?? { error: "unauthorized" }),
    });
  });
  await page.route("**/v1/ui/sessions/login", async (route) => {
    pendingSelection = true;
    await route.fulfill({
      status: 409,
      contentType: "application/json",
      body: JSON.stringify({
        required: true,
        user_email: "tenant-admin@alpha.test",
        csrf_token: "csrf-select-123",
        items: [
          { tenant_id: "tenant_a", name: "Tenant A", role: "admin" },
          { tenant_id: "tenant_b", name: "Tenant B", role: "writer" },
        ],
      }),
    });
  });
  await page.route("**/v1/ui/workspaces/pending", async (route) => {
    await route.fulfill({
      status: pendingSelection ? 200 : 401,
      contentType: "application/json",
      body: JSON.stringify(
        pendingSelection
          ? {
              required: true,
              user_email: "tenant-admin@alpha.test",
              csrf_token: "csrf-select-123",
              items: [
                { tenant_id: "tenant_a", name: "Tenant A", role: "admin" },
                { tenant_id: "tenant_b", name: "Tenant B", role: "writer" },
              ],
            }
          : { error: "workspace selection not pending" }
      ),
    });
  });
  await page.route("**/v1/ui/workspaces/select", async (route) => {
    const body = route.request().postDataJSON() as { tenant_id?: string };
    selectedSession = {
      authenticated: true,
      subject_type: "user",
      subject_id: "usr_tenant_chooser",
      user_email: "tenant-admin@alpha.test",
      scope: "tenant",
      role: body.tenant_id === "tenant_b" ? "writer" : "admin",
      tenant_id: body.tenant_id || "tenant_a",
      csrf_token: "csrf-tenant-selected",
    };
    pendingSelection = false;
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify(selectedSession),
    });
  });
  await page.route("**/v1/customers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });
}

test("platform login lands on billing connections", async ({ page }) => {
  await installLoginMock(page, {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_platform_1",
    user_email: "platform-admin@alpha.test",
    scope: "platform",
    platform_role: "platform_admin",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/login");
  await page.getByTestId("session-login-email").fill("platform-admin@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page).toHaveURL(/\/billing-connections$/);
  await expect(page.getByRole("heading", { name: "Billing Connections" })).toBeVisible();
});

test("unauthenticated route redirects to login and returns to requested tenant page", async ({ page }) => {
  await installLoginMock(page, {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_tenant_1",
    user_email: "tenant-writer@alpha.test",
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/customers");
  await expect(page).toHaveURL(/\/login\?next=%2Fcustomers$/);

  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page).toHaveURL(/\/customers$/);
  await expect(page.getByRole("heading", { name: "Customers" })).toBeVisible();
});

test("multi-workspace login opens chooser before entering tenant surface", async ({ page }) => {
  await installWorkspaceSelectionLoginMock(page);

  await page.goto("/login?next=%2Fcustomers");
  await page.getByTestId("session-login-email").fill("tenant-admin@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page).toHaveURL(/\/workspace-select\?next=%2Fcustomers$/);
  await expect(page.getByRole("heading", { name: "Choose the workspace you want to open" })).toBeVisible();
  await page.getByRole("button", { name: /Tenant B/i }).click();
  await expect(page).toHaveURL(/\/customers$/);
});
