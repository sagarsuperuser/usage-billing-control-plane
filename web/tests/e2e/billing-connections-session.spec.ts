import { expect, test, type BrowserContext, type Page } from "@playwright/test";

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
  check_ready: boolean;
  check_blocker_code?: "disabled" | "missing_secret" | "billing_setup_incomplete";
  check_blocker_summary?: string;
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
  disabled_at?: string;
};

async function installBillingConnectionMock(context: BrowserContext, session: PlatformSessionPayload, initialConnections: BillingProviderConnection[] = []) {
  let capturedCSRF = "";
  let connections: BillingProviderConnection[] = [...initialConnections];

  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });

  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(session),
    });
  });

  await context.route("**/internal/billing-provider-connections", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ items: connections }),
      });
      return;
    }

    if (method === "POST") {
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
        sync_summary: "Run the first connection check after billing setup is complete.",
        check_ready: true,
        linked_workspace_count: 0,
        secret_configured: true,
        created_by_type: "platform_api_key",
        created_at: now,
        updated_at: now,
      };
      connections = [created, ...connections];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({ connection: created }),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/internal/billing-provider-connections/bpc_alpha/sync", async (route) => {
    const now = new Date().toISOString();
    connections = connections.map((item) =>
      item.id === "bpc_alpha"
        ? {
            ...item,
            status: "connected",
            workspace_ready: true,
            sync_state: "healthy",
            sync_summary: "Stripe is connected and ready for billing.",
            check_ready: true,
            lago_provider_code: "alpha_stripe_test_bpc_alpha",
            connected_at: now,
            last_synced_at: now,
            updated_at: now,
          }
        : item,
    );
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ connection: connections[0] }),
    });
  });

  await context.route("**/internal/billing-provider-connections/bpc_alpha/rotate-secret", async (route) => {
    capturedCSRF = route.request().headers()["x-csrf-token"] || "";
    const now = new Date().toISOString();
    connections = connections.map((item) =>
      item.id === "bpc_alpha"
        ? {
            ...item,
            status: "pending",
            workspace_ready: false,
            sync_state: "pending",
            sync_summary: "Run another connection check before using this connection for billing.",
            check_ready: true,
            last_synced_at: undefined,
            updated_at: now,
          }
        : item,
    );
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ connection: connections[0] }),
    });
  });

  await context.route("**/internal/billing-provider-connections/bpc_alpha", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();
    const connection = connections.find((item) => item.id === "bpc_alpha");

    if (method === "GET") {
      await route.fulfill({
        status: connection ? 200 : 404,
        contentType: "application/json",
        body: JSON.stringify(connection ? { connection } : { error: "not found" }),
      });
      return;
    }

    if (method === "PATCH") {
      capturedCSRF = request.headers()["x-csrf-token"] || "";
      const body = request.postDataJSON() as Record<string, string>;
      const now = new Date().toISOString();
      connections = connections.map((item) =>
        item.id === "bpc_alpha"
          ? {
              ...item,
              display_name: body.display_name || item.display_name,
              environment: body.environment === "live" ? "live" : body.environment === "test" ? "test" : item.environment,
              updated_at: now,
            }
          : item,
      );
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ connection: connections[0] }),
      });
      return;
    }

    await route.fallback();
  });

  return {
    getCapturedCSRF: () => capturedCSRF,
  };
}

async function createConnectionFromNewScreen(page: Page) {
  const nameInput = page.getByLabel("Connection name");
  const secretInput = page.getByLabel("Stripe secret key");
  const submitButton = page.getByRole("button", { name: "Create and check connection" });

  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();
  await expect(page.getByRole("heading", { name: "New billing connection" })).toBeVisible();
  await expect(nameInput).toBeEditable();
  await nameInput.click();
  await nameInput.pressSequentially("Stripe Sandbox");
  await expect(nameInput).toHaveValue("Stripe Sandbox");
  await secretInput.click();
  await secretInput.pressSequentially("sk_test_123");
  await expect(secretInput).toHaveValue("sk_test_123");
  await expect(submitButton).toBeEnabled();
  await submitButton.click({ force: true });
}

test("platform admin can create and sync a billing connection", async ({ page, context }) => {
  const session: PlatformSessionPayload = {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_platform_1",
    user_email: "platform-admin@alpha.test",
    scope: "platform",
    platform_role: "platform_admin",
    csrf_token: "csrf-platform-123",
  };
  const mock = await installBillingConnectionMock(context, session);

  await page.goto("/billing-connections/new");
  await createConnectionFromNewScreen(page);

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page).toHaveURL(/\/billing-connections\/bpc_alpha$/);
  await expect(page.getByRole("heading", { name: "Stripe Sandbox" })).toBeVisible();
  await expect(page.locator("div").filter({ hasText: /^Stripe is connected and ready for billing\.$/ })).toBeVisible();
});

test("platform admin can edit billing connection detail metadata", async ({ page, context }) => {
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
  const mock = await installBillingConnectionMock(context, session, [
    {
      id: "bpc_alpha",
      provider_type: "stripe",
      environment: "test",
      display_name: "Stripe Sandbox",
      scope: "platform",
      status: "connected",
      workspace_ready: true,
      sync_state: "healthy",
      sync_summary: "Stripe is connected and ready for billing.",
      check_ready: true,
      linked_workspace_count: 0,
      secret_configured: true,
      created_by_type: "platform_api_key",
      created_at: seededNow,
      updated_at: seededNow,
      connected_at: seededNow,
      last_synced_at: seededNow,
    },
  ]);

  await page.goto("/billing-connections/bpc_alpha");
  await expect(page.getByRole("heading", { name: "Stripe Sandbox" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit" })).toBeVisible();
  await page.getByRole("button", { name: "Edit" }).click();
  await expect(page.getByRole("button", { name: "Save changes" })).toBeVisible();

  const connectionNameInput = page.getByLabel("Connection name");

  await expect(connectionNameInput).toBeEditable();
  await connectionNameInput.click();
  await connectionNameInput.press("Meta+A");
  await connectionNameInput.pressSequentially("Stripe Sandbox Updated");
  const saveButton = page.getByRole("button", { name: "Save changes" });
  await expect(saveButton).toBeEnabled();
  await saveButton.click({ force: true });

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page.getByRole("heading", { name: "Stripe Sandbox Updated" })).toBeVisible();
});

test("platform admin can rotate a billing connection secret and see it return to pending", async ({ page, context }) => {
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
  const mock = await installBillingConnectionMock(context, session, [
    {
      id: "bpc_alpha",
      provider_type: "stripe",
      environment: "test",
      display_name: "Stripe Sandbox",
      scope: "platform",
      status: "connected",
      workspace_ready: true,
      sync_state: "healthy",
      sync_summary: "Stripe is connected and ready for billing.",
      check_ready: true,
      linked_workspace_count: 1,
      secret_configured: true,
      created_by_type: "platform_api_key",
      created_at: seededNow,
      updated_at: seededNow,
      connected_at: seededNow,
      last_synced_at: seededNow,
    },
  ]);

  await page.goto("/billing-connections/bpc_alpha");
  const rotatedSecretInput = page.getByLabel("New Stripe secret key");
  await rotatedSecretInput.click();
  await rotatedSecretInput.pressSequentially("sk_test_rotated");
  await page.getByRole("button", { name: "Rotate secret" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page.locator("div").filter({ hasText: /^Run another connection check before using this connection for billing\.$/ })).toBeVisible();
});

test("platform admin sees billing setup required before the first connection check", async ({ page, context }) => {
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
  await installBillingConnectionMock(context, session, [
    {
      id: "bpc_alpha",
      provider_type: "stripe",
      environment: "test",
      display_name: "Stripe Sandbox",
      scope: "platform",
      status: "pending",
      workspace_ready: false,
      sync_state: "never_synced",
      sync_summary: "Run the first connection check after billing setup is complete.",
      check_ready: false,
      check_blocker_code: "billing_setup_incomplete",
      check_blocker_summary: "Complete billing setup for this connection before running a full connection check.",
      linked_workspace_count: 0,
      secret_configured: true,
      created_by_type: "platform_api_key",
      created_at: seededNow,
      updated_at: seededNow,
    },
  ]);

  await page.goto("/billing-connections/bpc_alpha");

  await expect(page.getByRole("button", { name: "Billing setup required" })).toBeDisabled();
  await expect(page.getByText("Complete billing setup for this connection before running a full connection check.").first()).toBeVisible();
  await expect(page.getByText("Billing setup required").nth(1)).toBeVisible();
});

test("platform admin sees explicit verification checks for a failed billing connection", async ({ page, context }) => {
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
  await installBillingConnectionMock(context, session, [
    {
      id: "bpc_alpha",
      provider_type: "stripe",
      environment: "test",
      display_name: "Stripe Sandbox",
      scope: "platform",
      status: "sync_error",
      workspace_ready: false,
      sync_state: "failed",
      sync_summary: "provider timeout",
      check_ready: true,
      linked_workspace_count: 2,
      lago_organization_id: "org_original",
      secret_configured: true,
      created_by_type: "platform_api_key",
      created_at: seededNow,
      updated_at: seededNow,
      last_sync_error: "provider timeout",
    },
  ]);

  await page.goto("/billing-connections/bpc_alpha");

  await expect(page.getByRole("heading", { name: "Current status" })).toBeVisible();
  await expect(page.getByText("Status", { exact: true }).nth(1)).toBeVisible();
  await expect(page.getByRole("heading", { name: "Needs attention" })).toBeVisible();
  await expect(page.getByText("2 linked workspaces depend on this path.")).toBeVisible();
  await expect(page.getByText("Latest issue", { exact: true })).toBeVisible();
  await expect(page.locator("div").filter({ hasText: /^provider timeout$/ }).first()).toBeVisible();
  await expect(page.getByText("Workspace assignment")).toBeVisible();
  await expect(page.getByText("There are 2 linked workspaces, but the connection is not currently ready.")).toBeVisible();
  await expect(page.getByText("Correct the issue, then refresh the connection.")).toBeVisible();
});
