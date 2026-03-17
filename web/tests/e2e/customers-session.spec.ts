import { expect, test, type Page } from "@playwright/test";

type TenantSessionPayload = {
  authenticated: boolean;
  scope: "tenant";
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
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

function buildReadiness(paymentReady: boolean, syncError: boolean): CustomerReadiness {
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

async function installCustomersMock(page: Page, session: TenantSessionPayload) {
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
  ];

  const readinessByCustomer: Record<string, CustomerReadiness> = {
    cust_alpha: buildReadiness(false, false),
    cust_beta: buildReadiness(true, true),
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
    if (path === "/v1/customers" && method === "GET") {
      return json(200, customers);
    }
    if (path.startsWith("/v1/customers/") && path.endsWith("/readiness") && method === "GET") {
      const externalID = decodeURIComponent(path.split("/")[3] || "");
      const readiness = readinessByCustomer[externalID];
      if (!readiness) return json(404, { error: "not found" });
      return json(200, readiness);
    }

    return route.continue();
  });
}

test("tenant writer can browse customers and open customer detail", async ({ page }) => {
  await installCustomersMock(page, {
    authenticated: true,
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    api_key_id: "tenant_writer_1",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/customers");
  await page.getByTestId("session-login-api-key").fill("tenant-key");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Customers" })).toBeVisible();
  await expect(page.getByRole("link", { name: "New customer" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Customer Alpha/i })).toBeVisible();
  await expect(page.getByText("Next action: Customer has not completed payment setup")).toBeVisible();

  await page.getByRole("link", { name: /Customer Alpha/i }).click();
  await expect(page).toHaveURL(/\/customers\/cust_alpha$/);
  await expect(page.getByRole("heading", { name: "Customer Alpha" })).toBeVisible();
  await expect(page.getByText("Customer has not completed payment setup").first()).toBeVisible();
});
