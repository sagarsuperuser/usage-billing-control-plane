import { expect, test, type Page, type Route } from "@playwright/test";

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
    customer_active: boolean;
    billing_profile_status: string;
    payment_setup_status: string;
    missing_steps: string[];
  };
};

type CustomerRecord = {
  id: string;
  external_id: string;
  display_name: string;
  email?: string;
  status: "active" | "suspended" | "archived";
  lago_customer_id?: string;
  created_at: string;
  updated_at: string;
};

type CustomerReadiness = {
  status: string;
  missing_steps: string[];
  customer_exists: boolean;
  customer_active: boolean;
  billing_provider_configured: boolean;
  lago_customer_synced: boolean;
  default_payment_method_verified: boolean;
  billing_profile_status: string;
  payment_setup_status: string;
  billing_profile: {
    profile_status: string;
    last_sync_error?: string;
    last_synced_at?: string;
  };
  payment_setup: {
    setup_status: string;
    last_verified_at?: string;
  };
};

function buildPlatformReadiness(
  pricingReady: boolean,
  customerExists: boolean
): TenantOnboardingReadiness {
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

function buildCustomerReadiness(
  paymentReady: boolean,
  syncError: boolean
): CustomerReadiness {
  const now = new Date().toISOString();
  return {
    status: paymentReady && !syncError ? "ready" : "pending",
    missing_steps: [
      ...(paymentReady ? [] : ["payment_setup_ready", "default_payment_method_verified"]),
      ...(syncError ? ["lago_customer_synced"] : []),
    ],
    customer_exists: true,
    customer_active: true,
    billing_provider_configured: true,
    lago_customer_synced: !syncError,
    default_payment_method_verified: paymentReady,
    billing_profile_status: syncError ? "sync_error" : "ready",
    payment_setup_status: paymentReady ? "ready" : "pending",
    billing_profile: {
      profile_status: syncError ? "sync_error" : "ready",
      last_sync_error: syncError ? "provider timeout" : "",
      last_synced_at: now,
    },
    payment_setup: {
      setup_status: paymentReady ? "ready" : "pending",
      last_verified_at: now,
    },
  };
}

async function fulfillJSON(route: Route, status: number, payload: unknown) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(payload),
  });
}

function focusLine(page: Page, title: string) {
  return page.locator("div.rounded-2xl").filter({ has: page.getByText(title, { exact: true }) }).first();
}

async function installPlatformOverviewMock(page: Page, session: PlatformSessionPayload) {
  let loggedIn = false;
  const tenants: TenantRecord[] = [
    {
      id: "tenant_alpha",
      name: "Tenant Alpha",
      status: "active",
      lago_organization_id: "org_alpha",
      lago_billing_provider_code: "stripe_default",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "tenant_beta",
      name: "Tenant Beta",
      status: "active",
      lago_organization_id: "",
      lago_billing_provider_code: "",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ];

  const readinessByTenant: Record<string, TenantOnboardingReadiness> = {
    tenant_alpha: buildPlatformReadiness(false, false),
    tenant_beta: buildPlatformReadiness(true, false),
  };

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method().toUpperCase();

    if (path === "/v1/ui/sessions/me" && method === "GET") {
      return fulfillJSON(route, loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    }
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      loggedIn = true;
      return fulfillJSON(route, 201, session);
    }
    if (path === "/v1/ui/sessions/logout" && method === "POST") {
      loggedIn = false;
      return fulfillJSON(route, 200, { logged_out: true });
    }
    if (path === "/internal/tenants" && method === "GET") {
      return fulfillJSON(route, 200, tenants);
    }
    if (path.startsWith("/internal/onboarding/tenants/") && method === "GET") {
      const tenantID = decodeURIComponent(path.split("/").pop() || "");
      const tenant = tenants.find((item) => item.id === tenantID);
      if (!tenant) {
        return fulfillJSON(route, 404, { error: "not found" });
      }
      return fulfillJSON(route, 200, {
        tenant,
        readiness: readinessByTenant[tenantID],
        tenant_id: tenantID,
      });
    }

    return route.continue();
  });
}

async function installTenantOverviewMock(page: Page, session: TenantSessionPayload) {
  let loggedIn = false;
  const customers: CustomerRecord[] = [
    {
      id: "cust_row_1",
      external_id: "cust_alpha",
      display_name: "Customer Alpha",
      status: "active",
      lago_customer_id: "lago_cust_alpha",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "cust_row_2",
      external_id: "cust_beta",
      display_name: "Customer Beta",
      status: "active",
      lago_customer_id: "lago_cust_beta",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "cust_row_3",
      external_id: "cust_gamma",
      display_name: "Customer Gamma",
      status: "active",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ];

  const readinessByCustomer: Record<string, CustomerReadiness> = {
    cust_alpha: buildCustomerReadiness(false, false),
    cust_beta: buildCustomerReadiness(true, true),
    cust_gamma: buildCustomerReadiness(true, false),
  };

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method().toUpperCase();

    if (path === "/v1/ui/sessions/me" && method === "GET") {
      return fulfillJSON(route, loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    }
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      loggedIn = true;
      return fulfillJSON(route, 201, session);
    }
    if (path === "/v1/ui/sessions/logout" && method === "POST") {
      loggedIn = false;
      return fulfillJSON(route, 200, { logged_out: true });
    }
    if (path === "/v1/customers" && method === "GET") {
      return fulfillJSON(route, 200, customers);
    }
    if (path.startsWith("/v1/customers/") && path.endsWith("/readiness") && method === "GET") {
      const externalID = decodeURIComponent(path.split("/")[3] || "");
      const readiness = readinessByCustomer[externalID];
      if (!readiness) {
        return fulfillJSON(route, 404, { error: "not found" });
      }
      return fulfillJSON(route, 200, readiness);
    }

    return route.continue();
  });
}

test("platform overview shows live workspace attention counts", async ({ page }) => {
  await installPlatformOverviewMock(page, {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/control-plane");

  await page.getByTestId("session-login-api-key").fill("platform-key");
  await page.getByTestId("session-login-submit").click();

  const nav = page.locator("nav");
  await expect(page.getByRole("heading", { name: "Primary onboarding journeys" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Workspaces", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Workspace Setup", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Payments", exact: true })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Recovery", exact: true })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Explainability", exact: true })).toHaveCount(0);
  await expect(page.getByText("Create a tenant workspace, connect billing, and hand off the first admin credential.")).toBeVisible();
  await expect(page.getByText("Create the first billable customer, sync the billing profile, and start payment setup.")).toHaveCount(0);
  await expect(page.getByText("Monitor invoice payment failures, inspect webhook history, and trigger payment retries.")).toHaveCount(0);
  await expect(page.getByText("Queue replay jobs, inspect diagnostics, and recover failed reprocessing runs.")).toHaveCount(0);
  await expect(page.getByText("Show deterministic line-item computation trace and digest for financial correctness workflows.")).toHaveCount(0);
  await expect(page.getByText("Workspaces missing pricing")).toBeVisible();
  await expect(page.getByText("Workspaces missing first customer")).toBeVisible();
  await expect(page.getByText("Workspaces missing billing connection")).toBeVisible();

  await expect(focusLine(page, "Workspaces missing pricing").locator("div.text-lg")).toHaveText("1");
  await expect(focusLine(page, "Workspaces missing first customer").locator("div.text-lg")).toHaveText("2");
  await expect(focusLine(page, "Workspaces missing billing connection").locator("div.text-lg")).toHaveText("1");
});

test("tenant overview shows live customer attention counts", async ({ page }) => {
  await installTenantOverviewMock(page, {
    authenticated: true,
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    api_key_id: "tenant_writer_1",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/control-plane");

  await page.getByTestId("session-login-api-key").fill("tenant-key");
  await page.getByTestId("session-login-submit").click();

  const nav = page.locator("nav");
  await expect(page.getByRole("heading", { name: "Primary onboarding journeys" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Customers", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Customer Setup", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Payments", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Recovery", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Explainability", exact: true })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Workspaces", exact: true })).toHaveCount(0);
  await expect(nav.getByRole("link", { name: "Workspace Setup", exact: true })).toHaveCount(0);
  await expect(page.getByText("Create the first billable customer, sync the billing profile, and start payment setup.")).toBeVisible();
  await expect(page.getByText("Browse customer billing readiness, payment setup state, and recovery needs from one tenant directory.")).toBeVisible();
  await expect(page.getByText("Monitor invoice payment failures, inspect webhook history, and trigger payment retries.")).toBeVisible();
  await expect(page.getByText("Queue replay jobs, inspect diagnostics, and recover failed reprocessing runs.")).toBeVisible();
  await expect(page.getByText("Show deterministic line-item computation trace and digest for financial correctness workflows.")).toBeVisible();
  await expect(page.getByText("Create a tenant workspace, connect billing, and hand off the first admin credential.")).toHaveCount(0);
  await expect(page.getByText("Customers waiting on payment setup")).toBeVisible();
  await expect(page.getByText("Customers with billing sync errors")).toBeVisible();
  await expect(page.getByText("Billing-ready customers")).toBeVisible();

  await expect(focusLine(page, "Customers waiting on payment setup").locator("div.text-lg")).toHaveText("1");
  await expect(focusLine(page, "Customers with billing sync errors").locator("div.text-lg")).toHaveText("1");
  await expect(focusLine(page, "Billing-ready customers").locator("div.text-lg")).toHaveText("1");
});
