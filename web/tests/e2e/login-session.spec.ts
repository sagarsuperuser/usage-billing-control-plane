import { test, type BrowserContext, type Page } from "@playwright/test";

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

async function waitForNavigationIntent(page: Page, pattern: RegExp) {
  await page.waitForURL(pattern, { timeout: 10_000 });
}

async function installLoginMock(context: BrowserContext, session: PlatformSessionPayload | TenantSessionPayload) {
  let loggedIn = false;

  const jsonRoute = async (pattern: string, handler: Parameters<BrowserContext["route"]>[1]) => {
    await context.route(pattern, handler);
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

async function installWorkspaceSelectionLoginMock(context: BrowserContext) {
  // Multi-workspace user: backend auto-selects first workspace (no chooser page)
  const autoSelectedSession: TenantSessionPayload = {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_tenant_chooser",
    user_email: "tenant-admin@alpha.test",
    scope: "tenant",
    role: "admin",
    tenant_id: "tenant_a",
    csrf_token: "csrf-tenant-auto",
  };

  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });
  await context.route("**/v1/ui/auth/providers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ password_enabled: true, sso_providers: [] }),
    });
  });
  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ error: "unauthorized" }),
    });
  });
  await context.route("**/v1/ui/sessions/login", async (route) => {
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify(autoSelectedSession),
    });
  });
  await context.route("**/v1/customers", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });
}

test("platform login navigates to control plane overview", async ({ page, context }) => {
  await installLoginMock(context, {
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
  await Promise.all([
    waitForNavigationIntent(page, /\/control-plane/),
    page.getByTestId("session-login-submit").click(),
  ]);
});

test("tenant login requests the requested customer route", async ({ page, context }) => {
  await installLoginMock(context, {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_tenant_1",
    user_email: "tenant-writer@alpha.test",
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/login?next=%2Fcustomers");

  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await Promise.all([
    waitForNavigationIntent(page, /\/customers(?:\?_rsc=.*)?$/),
    page.getByTestId("session-login-submit").click(),
  ]);
});

test("multi-workspace login auto-selects first workspace and navigates to requested route", async ({ page, context }) => {
  await installWorkspaceSelectionLoginMock(context);

  await page.goto("/login?next=%2Fcustomers");
  await page.getByTestId("session-login-email").fill("tenant-admin@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await Promise.all([
    waitForNavigationIntent(page, /\/customers(?:\?_rsc=.*)?$/),
    page.getByTestId("session-login-submit").click(),
  ]);
});
