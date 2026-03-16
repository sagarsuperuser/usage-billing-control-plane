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
    customer_external_id?: string;
    customer_active: boolean;
    billing_profile_status: string;
    payment_setup_status: string;
    missing_steps: string[];
    note?: string;
  };
};

type TenantOnboardingMockWindow = Window & typeof globalThis & {
  __tenantOnboardingMock: {
    csrf: string;
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
  await page.addInitScript(({ session }: { session: PlatformSessionPayload }) => {
    let loggedIn = true;
    let tenants: TenantRecord[] = [];
    const readinessByTenant: Record<string, TenantOnboardingReadiness> = {};
    const w = window as TenantOnboardingMockWindow;
    w.__tenantOnboardingMock = { csrf: "" };

    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: { "Content-Type": "application/json" },
      });

    const originalFetch = window.fetch.bind(window);

    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : null;
      const method = (init?.method || request?.method || "GET").toUpperCase();
      const rawURL =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;
      const url = new URL(rawURL, window.location.origin);
      const path = url.pathname;
      const headers = new Headers(init?.headers || request?.headers);

      if (path === "/v1/ui/sessions/me" && method === "GET") {
        return loggedIn ? json(200, session) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/ui/sessions/login" && method === "POST") {
        loggedIn = true;
        return json(201, session);
      }

      if (path === "/v1/ui/sessions/logout" && method === "POST") {
        const csrf = headers.get("X-CSRF-Token") || "";
        if (csrf !== session.csrf_token) {
          return json(403, { error: "forbidden" });
        }
        loggedIn = false;
        return json(200, { logged_out: true });
      }

      if (path === "/internal/tenants" && method === "GET") {
        return loggedIn ? json(200, tenants) : json(401, { error: "unauthorized" });
      }

      if (path === "/internal/onboarding/tenants" && method === "POST") {
        const csrf = headers.get("X-CSRF-Token") || "";
        w.__tenantOnboardingMock.csrf = csrf;
        const body = JSON.parse(String(init?.body || "{}"));
        const now = new Date().toISOString();
        const tenant: TenantRecord = {
          id: body.id,
          name: body.name || body.id,
          status: "active",
          lago_organization_id: body.lago_organization_id,
          lago_billing_provider_code: body.lago_billing_provider_code,
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

      return originalFetch(input, init);
    };
  }, { session });
}

test.beforeEach(async ({ page }) => {
  await installTenantOnboardingMock(page, sessionPayload);
});

test("platform admin can onboard a tenant from the UI", async ({ page }) => {
  await page.goto("/tenant-onboarding");

  await expect(page.getByRole("heading", { name: "Workspace Setup" })).toBeVisible();
  await page.getByLabel("Workspace ID").fill("tenant_acme");
  await page.getByLabel("Workspace name").fill("Acme Corp");
  await page.getByLabel("Billing organization ID").fill("org_acme");
  await page.getByLabel("Billing connection code").fill("stripe_default");
  await page.getByRole("button", { name: "Run workspace setup" }).click();

  await expect.poll(async () =>
    page.evaluate(() => (window as TenantOnboardingMockWindow).__tenantOnboardingMock.csrf)
  ).toBe("csrf-platform-123");

  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.getByRole("button", { name: /Acme Corp/i })).toBeVisible();
  await page.getByRole("button", { name: /Acme Corp/i }).click();
  await expect(page.getByText("Acme Corp")).toBeVisible();
  await expect(page.getByText("tenant_acme")).toBeVisible();
});
