import { expect, test, type Page } from "@playwright/test";

type PlatformSessionPayload = {
  authenticated: boolean;
  scope: "platform";
  platform_role: "platform_admin";
  api_key_id: string;
  csrf_token: string;
};

type TenantSessionPayload = {
  authenticated: boolean;
  scope: "tenant";
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
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
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/login");
  await page.getByTestId("session-login-api-key").fill("platform-key");
  await page.getByTestId("session-login-submit").click();

  await expect(page).toHaveURL(/\/billing-connections$/);
  await expect(page.getByRole("heading", { name: "Billing Connections" })).toBeVisible();
});

test("unauthenticated route redirects to login and returns to requested tenant page", async ({ page }) => {
  await installLoginMock(page, {
    authenticated: true,
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    api_key_id: "tenant_writer_1",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/customers");
  await expect(page).toHaveURL(/\/login\?next=%2Fcustomers$/);

  await page.getByTestId("session-login-api-key").fill("tenant-key");
  await page.getByTestId("session-login-submit").click();

  await expect(page).toHaveURL(/\/customers$/);
  await expect(page.getByRole("heading", { name: "Customers" })).toBeVisible();
});
