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
        sync_summary: "Connection has not been synced yet. Run the first sync before assigning it to workspaces.",
        linked_workspace_count: 0,
        lago_organization_id: body.lago_organization_id,
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
            sync_summary: "Connected and ready for workspace assignment.",
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
            sync_summary: "Connection is waiting for a successful provider sync.",
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
              lago_organization_id: body.lago_organization_id || undefined,
              lago_provider_code: body.lago_provider_code || undefined,
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
  const submitButton = page.getByRole("button", { name: "Create and sync connection" });

  await expect(page.getByRole("heading", { name: "New billing connection" })).toBeVisible();
  await expect(nameInput).toBeEditable();
  await nameInput.click();
  await nameInput.pressSequentially("Stripe Sandbox");
  await expect(nameInput).toHaveValue("Stripe Sandbox");
  await secretInput.click();
  await secretInput.pressSequentially("sk_test_123");
  await expect(secretInput).toHaveValue("sk_test_123");
  await expect(submitButton).toBeEnabled();
  await submitButton.click();
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
  await expect(page.locator("div").filter({ hasText: /^Connected and ready for workspace assignment\.$/ })).toBeVisible();
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
  await expect(page.getByRole("heading", { name: "Stripe Sandbox" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Edit" })).toBeVisible();
  await page.getByRole("button", { name: "Edit" }).click();
  await expect(page.getByRole("button", { name: "Save changes" })).toBeVisible();

  const connectionNameInput = page.getByLabel("Connection name");
  const orgOverrideInput = page.getByLabel("Billing organization override");
  const providerCodeInput = page.getByLabel("Provider code override");

  await expect(connectionNameInput).toBeEditable();
  await connectionNameInput.click();
  await connectionNameInput.press("Meta+A");
  await connectionNameInput.pressSequentially("Stripe Sandbox Updated");
  await orgOverrideInput.click();
  await orgOverrideInput.press("Meta+A");
  await orgOverrideInput.pressSequentially("org_updated");
  await providerCodeInput.click();
  await providerCodeInput.press("Meta+A");
  await providerCodeInput.pressSequentially("alpha_override");
  const saveButton = page.getByRole("button", { name: "Save changes" });
  await expect(saveButton).toBeEnabled();
  await saveButton.click({ force: true });

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page.getByRole("heading", { name: "Stripe Sandbox Updated" })).toBeVisible();
  await expect(page.getByText("org_updated", { exact: true })).toBeVisible();
  await expect(page.getByText("alpha_override", { exact: true })).toBeVisible();
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
      sync_summary: "Connected and ready for workspace assignment.",
      linked_workspace_count: 1,
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
  const rotatedSecretInput = page.getByLabel("New Stripe secret key");
  await rotatedSecretInput.click();
  await rotatedSecretInput.pressSequentially("sk_test_rotated");
  await page.getByRole("button", { name: "Rotate secret" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect(page.locator("div").filter({ hasText: /^Connection is waiting for a successful provider sync\.$/ })).toBeVisible();
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

  await expect(page.getByRole("heading", { name: "Health checks" })).toBeVisible();
  await expect(page.getByText("Last sync error", { exact: true })).toBeVisible();
  await expect(page.locator("div").filter({ hasText: /^provider timeout$/ }).first()).toBeVisible();
  await expect(page.getByText("Workspace assignment")).toBeVisible();
  await expect(page.getByText("There are 2 linked workspaces, but the connection is not currently ready.")).toBeVisible();
  await expect(page.getByText("Review the last sync error, correct the provider secret or override, then rerun sync.")).toBeVisible();
});
