import { expect, test, type Page } from "@playwright/test";

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
  billing_provider_connection_id?: string;
  lago_organization_id?: string;
  lago_billing_provider_code?: string;
  created_at: string;
  updated_at: string;
};

type TenantOnboardingReadiness = {
  status: string;
  missing_steps: string[];
  tenant: {
    status: string;
    tenant_exists: boolean;
    tenant_active: boolean;
    tenant_admin_ready: boolean;
    missing_steps: string[];
  };
  billing_integration: {
    status: string;
    billing_mapping_ready: boolean;
    pricing_ready: boolean;
    missing_steps: string[];
  };
  first_customer: {
    status: string;
    managed: boolean;
    customer_exists: boolean;
    customer_active: boolean;
    billing_profile_status: string;
    payment_setup_status: string;
    missing_steps: string[];
  };
};

type BillingProviderConnection = {
  id: string;
  display_name: string;
  status: string;
  lago_organization_id?: string;
  lago_provider_code?: string;
};

function buildReadiness(pricingReady: boolean, customerExists: boolean): TenantOnboardingReadiness {
  return {
    status: pricingReady && customerExists ? "ready" : "pending",
    missing_steps: [
      ...(pricingReady ? [] : ["billing_integration.pricing"]),
      ...(customerExists ? [] : ["first_customer.customer_created"]),
    ],
    tenant: {
      status: "ready",
      tenant_exists: true,
      tenant_active: true,
      tenant_admin_ready: true,
      missing_steps: [],
    },
    billing_integration: {
      status: pricingReady ? "ready" : "pending",
      billing_mapping_ready: true,
      pricing_ready: pricingReady,
      missing_steps: pricingReady ? [] : ["pricing"],
    },
    first_customer: {
      status: customerExists ? "ready" : "pending",
      managed: true,
      customer_exists: customerExists,
      customer_active: customerExists,
      billing_profile_status: customerExists ? "ready" : "missing",
      payment_setup_status: customerExists ? "ready" : "missing",
      missing_steps: customerExists ? [] : ["customer_created"],
    },
  };
}

async function installWorkspaceMock(page: Page, session: PlatformSessionPayload) {
  let loggedIn = false;
  const connections: Record<string, BillingProviderConnection> = {
    bpc_alpha: {
      id: "bpc_alpha",
      display_name: "Stripe Alpha",
      status: "connected",
      lago_organization_id: "org_alpha",
      lago_provider_code: "stripe_default",
    },
    bpc_beta: {
      id: "bpc_beta",
      display_name: "Stripe Beta",
      status: "connected",
      lago_organization_id: "org_beta",
      lago_provider_code: "stripe_default",
    },
  };
  const tenants: TenantRecord[] = [
    {
      id: "tenant_alpha",
      name: "Tenant Alpha",
      status: "active",
      billing_provider_connection_id: "bpc_alpha",
      lago_organization_id: "org_alpha",
      lago_billing_provider_code: "stripe_default",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "tenant_beta",
      name: "Tenant Beta",
      status: "active",
      billing_provider_connection_id: "bpc_beta",
      lago_organization_id: "org_beta",
      lago_billing_provider_code: "stripe_default",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ];

  const readinessByTenant: Record<string, TenantOnboardingReadiness> = {
    tenant_alpha: buildReadiness(false, false),
    tenant_beta: buildReadiness(true, true),
  };

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
    if (path === "/internal/tenants" && method === "GET") {
      return json(200, tenants);
    }
    if (path.startsWith("/internal/onboarding/tenants/") && method === "GET") {
      const tenantID = decodeURIComponent(path.split("/").pop() || "");
      const tenant = tenants.find((item) => item.id === tenantID);
      if (!tenant) {
        return json(404, { error: "not found" });
      }
      return json(200, {
        tenant,
        readiness: readinessByTenant[tenantID],
        tenant_id: tenantID,
      });
    }
    if (path.startsWith("/internal/billing-provider-connections/") && method === "GET") {
      const connectionID = decodeURIComponent(path.split("/").pop() || "");
      const connection = connections[connectionID];
      return json(connection ? 200 : 404, connection ? { connection } : { error: "not found" });
    }

    return route.continue();
  });
}

test("platform admin can browse workspaces and open workspace detail", async ({ page }) => {
  await installWorkspaceMock(page, {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/workspaces");

  await page.getByTestId("session-login-api-key").fill("platform-key");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Workspaces" })).toBeVisible();
  await expect(page.getByRole("link", { name: "New workspace" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Tenant Alpha/i })).toBeVisible();
  await expect(page.getByText("Next action: pricing")).toBeVisible();

  await page.getByRole("link", { name: /Tenant Alpha/i }).click();
  await expect(page).toHaveURL(/\/workspaces\/tenant_alpha$/);
  await expect(page.getByRole("heading", { name: "Tenant Alpha" })).toBeVisible();
  await expect(page.getByText("Pricing rules still need to be configured").first()).toBeVisible();
  await expect(page.getByText("No billing-ready customer has been created yet").first()).toBeVisible();
  await expect(page.getByRole("link", { name: "Open billing connection" })).toBeVisible();
});
