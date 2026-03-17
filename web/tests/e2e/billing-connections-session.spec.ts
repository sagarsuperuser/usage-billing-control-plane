import { expect, test, type Page } from "@playwright/test";

type PlatformSessionPayload = {
  authenticated: boolean;
  scope: "platform";
  platform_role: "platform_admin";
  api_key_id: string;
  csrf_token: string;
};

type BillingProviderConnection = {
  id: string;
  provider_type: "stripe";
  environment: "test" | "live";
  display_name: string;
  scope: "platform";
  status: "pending" | "connected" | "sync_error" | "disabled";
  lago_organization_id?: string;
  lago_provider_code?: string;
  secret_configured: boolean;
  created_by_type: string;
  created_at: string;
  updated_at: string;
  connected_at?: string;
  last_synced_at?: string;
  last_sync_error?: string;
};

async function installBillingConnectionMock(page: Page, session: PlatformSessionPayload) {
  let loggedIn = false;
  let capturedCSRF = "";
  let connections: BillingProviderConnection[] = [];

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

    if (path === "/v1/ui/sessions/me" && method === "GET") {
      return json(loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    }
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      loggedIn = true;
      return json(201, session);
    }
    if (path === "/runtime-config" && method === "GET") {
      return json(200, { apiBaseURL: "" });
    }
    if (path === "/internal/billing-provider-connections" && method === "GET") {
      return json(200, { items: connections });
    }
    if (path === "/internal/billing-provider-connections" && method === "POST") {
      capturedCSRF = request.headers()["x-csrf-token"] || "";
      const body = request.postDataJSON() as Record<string, string>;
      const now = new Date().toISOString();
      const created: BillingProviderConnection = {
        id: "bpc_alpha",
        provider_type: "stripe",
        environment: body.environment === "live" ? "live" : "test",
        display_name: body.display_name || "Stripe Sandbox",
        scope: "platform",
        status: "pending",
        lago_organization_id: body.lago_organization_id,
        secret_configured: true,
        created_by_type: "platform_api_key",
        created_at: now,
        updated_at: now,
      };
      connections = [created, ...connections];
      return json(201, { connection: created });
    }
    if (path === "/internal/billing-provider-connections/bpc_alpha/sync" && method === "POST") {
      const now = new Date().toISOString();
      connections = connections.map((item) =>
        item.id === "bpc_alpha"
          ? {
              ...item,
              status: "connected",
              lago_provider_code: "alpha_stripe_test_bpc_alpha",
              connected_at: now,
              last_synced_at: now,
              updated_at: now,
            }
          : item
      );
      return json(200, { connection: connections[0] });
    }
    if (path === "/internal/billing-provider-connections/bpc_alpha" && method === "GET") {
      const connection = connections.find((item) => item.id === "bpc_alpha");
      return json(connection ? 200 : 404, connection ? { connection } : { error: "not found" });
    }

    return route.continue();
  });

  return {
    getCapturedCSRF: () => capturedCSRF,
  };
}

test("platform admin can create and sync a billing connection", async ({ page }) => {
  const session: PlatformSessionPayload = {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  };
  const mock = await installBillingConnectionMock(page, session);

  await page.goto("/billing-connections/new");
  await page.getByTestId("session-login-api-key").fill("platform-key");
  await page.getByTestId("session-login-submit").click();

  await page.getByLabel("Connection name").fill("Stripe Sandbox");
  await page.getByLabel("Billing organization ID").fill("org_acme");
  await page.getByLabel("Stripe secret key").fill("sk_test_123");
  await page.getByRole("button", { name: "Create and sync connection" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page).toHaveURL(/\/billing-connections\/bpc_alpha$/);
  await expect(page.getByRole("heading", { name: "Stripe Sandbox" })).toBeVisible();
  await expect(page.getByText("alpha_stripe_test_bpc_alpha")).toBeVisible();
});
