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

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method().toUpperCase();

    const json = async (status: number, payload: unknown) => {
      await route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(payload),
      });
    };

    if (path === "/runtime-config" && method === "GET") {
      return json(200, { apiBaseURL: "" });
    }
    if (path === "/v1/ui/sessions/me" && method === "GET") {
      return json(loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    }
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      const body = request.postDataJSON() as { email?: string; password?: string };
      if (!body?.email || !body?.password) {
        return json(400, { error: "email and password are required" });
      }
      loggedIn = true;
      return json(201, session);
    }
    if (path === "/internal/billing-provider-connections" && method === "GET") {
      return json(200, { items: [] });
    }
    if (path === "/v1/customers" && method === "GET") {
      return json(200, []);
    }

    return route.continue();
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
