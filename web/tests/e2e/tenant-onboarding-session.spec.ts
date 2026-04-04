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
  stripe_account_id?: string;
  stripe_provider_code?: string;
  created_at: string;
  updated_at: string;
};

type BillingProviderConnection = {
  id: string;
  provider_type: "stripe";
  environment: "test" | "live";
  display_name: string;
  scope: "platform";
  status: "connected" | "pending" | "sync_error" | "disabled";
  stripe_account_id?: string;
  stripe_provider_code?: string;
  secret_configured: boolean;
  created_by_type: string;
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
    note?: string;
  };
};

const sessionPayload: PlatformSessionPayload = {
  authenticated: true,
  scope: "platform",
  platform_role: "platform_admin",
  api_key_id: "platform_ui_1",
  csrf_token: "csrf-platform-123",
};

function buildReadiness(tenantID: string): TenantOnboardingReadiness {
  return {
    status: "pending",
    missing_steps: ["billing_integration.pricing", "first_customer.customer_created"],
    tenant: {
      status: "ready",
      tenant_exists: true,
      tenant_active: true,
      tenant_admin_ready: true,
      missing_steps: [],
    },
    billing_integration: {
      status: "pending",
      billing_mapping_ready: true,
      pricing_ready: false,
      missing_steps: ["pricing"],
    },
    first_customer: {
      status: "pending",
      managed: true,
      customer_exists: false,
      customer_active: false,
      billing_profile_status: "missing",
      payment_setup_status: "missing",
      missing_steps: ["customer_created"],
      note: `tenant ${tenantID} still needs a billing-ready customer`,
    },
  };
}

async function installTenantOnboardingMock(page: Page, session: PlatformSessionPayload) {
  let loggedIn = false;
  let tenants: TenantRecord[] = [];
  const readinessByTenant: Record<string, TenantOnboardingReadiness> = {};
  let capturedCSRF = "";
  let capturedOnboardingBody: Record<string, unknown> | null = null;
  const connection: BillingProviderConnection = {
    id: "bpc_alpha",
    provider_type: "stripe",
    environment: "test",
    display_name: "Stripe Sandbox",
    scope: "platform",
    status: "connected",
    stripe_account_id: "org_acme",
    stripe_provider_code: "alpha_stripe_test_bpc_alpha",
    secret_configured: true,
    created_by_type: "platform_api_key",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
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

    if (path === "/v1/ui/sessions/logout" && method === "POST") {
      const csrf = request.headers()["x-csrf-token"] || "";
      if (csrf !== session.csrf_token) {
        return json(403, { error: "forbidden" });
      }
      loggedIn = false;
      return json(200, { logged_out: true });
    }

    if (path === "/internal/tenants" && method === "GET") {
      return json(loggedIn ? 200 : 401, loggedIn ? tenants : { error: "unauthorized" });
    }

    if (path === "/internal/billing-provider-connections" && method === "GET") {
      return json(200, { items: [connection] });
    }

    if (path === "/internal/onboarding/tenants" && method === "POST") {
      capturedCSRF = request.headers()["x-csrf-token"] || "";
      const body = request.postDataJSON() as Record<string, string>;
      capturedOnboardingBody = body;
      const now = new Date().toISOString();
      const tenant: TenantRecord = {
        id: body.id,
        name: body.name || body.id,
        status: "active",
        billing_provider_connection_id: body.billing_provider_connection_id,
        stripe_account_id: connection.stripe_account_id,
        stripe_provider_code: connection.stripe_provider_code,
        created_at: now,
        updated_at: now,
      };
      tenants = [tenant, ...tenants.filter((item) => item.id !== tenant.id)];
      readinessByTenant[tenant.id] = buildReadiness(tenant.id);
      return json(201, {
        tenant,
        tenant_created: true,
        tenant_admin_bootstrap: {
          created: true,
          existing_active_keys: 0,
          api_key: {
            id: "key_tenant_acme",
            key_prefix: "pref_tenant_acme",
            name: "bootstrap-admin-tenant_acme",
            role: "admin",
            tenant_id: tenant.id,
            created_at: now,
          },
          secret: "tenant-admin-secret-123",
        },
        readiness: readinessByTenant[tenant.id],
      });
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
      return json(200, { connection });
    }

    if (path === "/runtime-config" && method === "GET") {
      return json(200, { apiBaseURL: "" });
    }

    return route.continue();
  });

  return {
    getCapturedCSRF: () => capturedCSRF,
    getCapturedOnboardingBody: () => capturedOnboardingBody,
  };
}

test("platform admin can onboard a tenant from the UI before selecting billing", async ({ page }) => {
  const mock = await installTenantOnboardingMock(page, sessionPayload);

  await page.goto("/tenant-onboarding");

  await page.getByTestId("session-login-email").fill("platform-admin@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Create workspace" })).toBeVisible();
  await page.getByLabel("Workspace ID").fill("tenant_acme");
  await page.getByLabel("Workspace name").fill("Acme Corp");
  await page.getByRole("button", { name: "Run workspace setup" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-platform-123");
  await expect.poll(() => mock.getCapturedOnboardingBody()).toMatchObject({
    id: "tenant_acme",
    name: "Acme Corp",
  });
  expect(mock.getCapturedOnboardingBody()).not.toHaveProperty("billing_provider_connection_id");

  await page.goto("/workspaces/tenant_acme");
  await expect(page.getByRole("heading", { name: "Acme Corp" })).toBeVisible();
  await expect(page.getByText("Create at least one metric and plan before going live").first()).toBeVisible();
  await expect(page.getByText("Change active connection")).toBeVisible();
  await expect(page.getByLabel("Active billing connection")).toBeVisible();
});
