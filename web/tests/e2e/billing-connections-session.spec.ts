import { expect, test, type Page } from "@playwright/test";

test.describe.configure({ mode: "serial" });

type PlatformSessionPayload = {
  authenticated: boolean;
  subject_type: "user";
  subject_id: string;
  user_email: string;
  scope: "platform";
  platform_role: "platform_admin";
  csrf_token: string;
};

type BillingProviderConnection = {
  id: string;
  provider_type: "stripe";
  environment: "test" | "live";
  display_name: string;
  scope: "platform";
  status: "pending" | "connected" | "sync_error" | "disabled";
  workspace_ready: boolean;
  sync_state: "healthy" | "failed" | "never_synced" | "pending" | "disabled";
  sync_summary: string;
  linked_workspace_count: number;
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

async function installBillingConnectionMock(page: Page, session: PlatformSessionPayload, initialConnections: BillingProviderConnection[] = []) {
  let loggedIn = true;
  let capturedCSRF = "";
  let connections: BillingProviderConnection[] = [...initialConnections];

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
        workspace_ready: false,
        sync_state: "never_synced",
        sync_summary: "Connection has not been synced yet. Run the first sync before assigning it to workspaces.",
        linked_workspace_count: 0,
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
              workspace_ready: true,
              sync_state: "healthy",
              sync_summary: "Connected and ready for workspace assignment.",
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
    if (path === "/internal/billing-provider-connections/bpc_alpha" && method === "PATCH") {
      capturedCSRF = request.headers()["x-csrf-token"] || "";
      const body = request.postDataJSON() as Record<string, string>;
      const now = new Date().toISOString();
      connections = connections.map((item) =>
        item.id === "bpc_alpha"
          ? {
              ...item,
              display_name: body.display_name || item.display_name,
              environment: body.environment === "live" ? "live" : body.environment === "test" ? "test" : item.environment,
              lago_organization_id: body.lago_organization_id || undefined,
              lago_provider_code: body.lago_provider_code || undefined,
              updated_at: now,
            }
          : item
      );
      return json(200, { connection: connections[0] });
    }

    return route.continue();
  });

  return {
    getCapturedCSRF: () => capturedCSRF,
  };
}

async function createConnectionFromNewScreen(page: Page) {
  const nameInput = page.getByLabel("Connection name");
  const secretInput = page.getByLabel("Stripe secret key");
  const submitButton = page.getByRole("button", { name: "Create and sync connection" });

  await expect(nameInput).toBeEditable();
  await nameInput.fill("Stripe Sandbox");
  await expect(nameInput).toHaveValue("Stripe Sandbox");

  await secretInput.fill("sk_test_123");
  await expect(secretInput).toHaveValue("sk_test_123");

  await expect(submitButton).toBeEnabled();
  await submitButton.click();
}

test("platform admin can create and sync a billing connection", async ({ page }) => {
  const session: PlatformSessionPayload = {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_platform_1",
    user_email: "platform-admin@alpha.test",
    scope: "platform",
    platform_role: "platform_admin",
    csrf_token: "csrf-platform-123",
  };
  const mock = await installBillingConnectionMock(page, session);

  await page.goto("/billing-connections/new");
  await createConnectionFromNewScreen(page);

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page).toHaveURL(/\/billing-connections\/bpc_alpha$/);
  await expect(page.getByRole("heading", { name: "Stripe Sandbox" })).toBeVisible();
  await expect(page.locator("div").filter({ hasText: /^Connected and ready for workspace assignment\.$/ })).toBeVisible();
});

test("platform admin can edit billing connection detail metadata", async ({ page }) => {
  const session: PlatformSessionPayload = {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_platform_1",
    user_email: "platform-admin@alpha.test",
    scope: "platform",
    platform_role: "platform_admin",
    csrf_token: "csrf-platform-123",
  };
  const seededNow = new Date().toISOString();
  const mock = await installBillingConnectionMock(page, session, [
    {
      id: "bpc_alpha",
      provider_type: "stripe",
      environment: "test",
      display_name: "Stripe Sandbox",
      scope: "platform",
      status: "connected",
      workspace_ready: true,
      sync_state: "healthy",
      sync_summary: "Connected and ready for workspace assignment.",
      linked_workspace_count: 0,
      lago_organization_id: "org_original",
      lago_provider_code: "alpha_original",
      secret_configured: true,
      created_by_type: "platform_api_key",
      created_at: seededNow,
      updated_at: seededNow,
      connected_at: seededNow,
      last_synced_at: seededNow,
    },
  ]);

  await page.goto("/billing-connections/bpc_alpha");

  await page.getByRole("button", { name: "Edit" }).click();
  await page.getByLabel("Connection name").fill("Stripe Sandbox Updated");
  await page.getByLabel("Billing organization reference").fill("org_updated");
  await page.getByLabel("Provider code").fill("alpha_override");
  await page.getByRole("button", { name: "Save changes" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page.getByRole("heading", { name: "Stripe Sandbox Updated" })).toBeVisible();
  const configPanel = page.locator("aside").first();
  await expect(configPanel.getByText("org_updated")).toBeVisible();
  await expect(configPanel.getByText("alpha_override")).toBeVisible();
});
