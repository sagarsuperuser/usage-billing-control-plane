import { expect, test, type BrowserContext } from "@playwright/test";

type PlatformSessionPayload = {
  authenticated: boolean;
  scope: "platform";
  platform_role: "platform_admin";
  api_key_id: string;
  csrf_token: string;
};

type TenantRecord = {
  id: string;
  name: string;
  status: "active" | "suspended" | "deleted";
  created_at: string;
  updated_at: string;
};

type TenantAuditEvent = {
  id: string;
  tenant_id: string;
  actor_api_key_id?: string;
  action: string;
  event_code: string;
  event_category: string;
  event_title: string;
  event_summary: string;
  metadata?: Record<string, unknown>;
  created_at: string;
};

async function installTenantAuditMock(context: BrowserContext, session: PlatformSessionPayload) {
  const now = new Date().toISOString();
  const tenants: TenantRecord[] = [
    { id: "tenant_alpha", name: "Tenant Alpha", status: "active", created_at: now, updated_at: now },
    { id: "tenant_beta", name: "Tenant Beta", status: "active", created_at: now, updated_at: now },
  ];
  const events: TenantAuditEvent[] = [
    {
      id: "tae_1",
      tenant_id: "tenant_alpha",
      actor_api_key_id: "apk_platform_admin",
      action: "customer.payment_setup_requested",
      event_code: "customer.payment_setup_requested",
      event_category: "Billing",
      event_title: "Payment setup requested",
      event_summary: "A payment setup request was sent to the customer.",
      metadata: { customer_id: "cust_123", payment_method_type: "card" },
      created_at: now,
    },
    {
      id: "tae_2",
      tenant_id: "tenant_beta",
      actor_api_key_id: "apk_platform_admin",
      action: "workspace.billing_connection_changed",
      event_code: "workspace.billing_connection_changed",
      event_category: "Billing",
      event_title: "Billing connection changed",
      event_summary: "Billing connection changed from bpc_old to bpc_beta.",
      metadata: { billing_provider_connection_id: "bpc_beta" },
      created_at: now,
    },
  ];

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

  await context.route("**/internal/tenants", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(tenants),
    });
  });

  await context.route("**/internal/tenants/audit**", async (route) => {
    const url = new URL(route.request().url());
    const tenantID = url.searchParams.get("tenant_id");
    const action = url.searchParams.get("action");
    const actorAPIKeyID = url.searchParams.get("actor_api_key_id");
    const filtered = events.filter((item) => {
      if (tenantID && item.tenant_id !== tenantID) return false;
      if (action && item.action !== action && item.event_code !== action) return false;
      if (actorAPIKeyID && item.actor_api_key_id !== actorAPIKeyID) return false;
      return true;
    });
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        items: filtered,
        total: filtered.length,
        limit: 50,
        offset: 0,
      }),
    });
  });
}

test("platform admin can inspect tenant audit history", async ({ page, context }) => {
  await installTenantAuditMock(context, {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "apk_platform_admin",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/tenant-audit");

  await expect(page.getByRole("heading", { name: "Workspace audit" })).toBeVisible();
  await expect(page.getByText("Payment setup requested")).toBeVisible();
  await expect(page.getByText("Billing connection changed")).toBeVisible();
  await expect(page.getByText("cust_123")).toHaveCount(0);
  await expect(page.getByText("card")).toHaveCount(0);

  await page.getByRole("combobox").first().selectOption("tenant_alpha");
  await expect(page.getByText("Payment setup requested")).toBeVisible();
  await expect(page.getByText("Billing connection changed")).toHaveCount(0);

  await page.getByPlaceholder("workspace.created").fill("customer.payment_setup_requested");
  await page.locator("tr", { hasText: "Payment setup requested" }).click();
  await expect(page.getByText("cust_123")).toBeVisible();
  await expect(page.getByText("card")).toBeVisible();
  await expect(page.getByText("Payment setup requested").first()).toBeVisible();
});
