import { expect, test, type Page, type Route } from "@playwright/test";


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
  provider_customer_id?: string;
  created_at: string;
  updated_at: string;
};

type CustomerReadiness = {
  status: string;
  missing_steps: string[];
  customer_exists: boolean;
  customer_active: boolean;
  billing_provider_configured: boolean;
  provider_customer_synced: boolean;
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


function buildCustomerReadiness(
  paymentReady: boolean,
  syncError: boolean
): CustomerReadiness {
  const now = new Date().toISOString();
  return {
    status: paymentReady && !syncError ? "ready" : "pending",
    missing_steps: [
      ...(paymentReady ? [] : ["payment_setup_ready", "default_payment_method_verified"]),
      ...(syncError ? ["provider_customer_synced"] : []),
    ],
    customer_exists: true,
    customer_active: true,
    billing_provider_configured: true,
    provider_customer_synced: !syncError,
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
  return page.locator("a").filter({ has: page.getByText(title, { exact: true }) }).first();
}



async function installTenantOverviewMock(page: Page, session: TenantSessionPayload) {
  let loggedIn = false;
  const customers: CustomerRecord[] = [
    {
      id: "cust_row_1",
      external_id: "cust_alpha",
      display_name: "Customer Alpha",
      status: "active",
      provider_customer_id: "cus_alpha",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    {
      id: "cust_row_2",
      external_id: "cust_beta",
      display_name: "Customer Beta",
      status: "active",
      provider_customer_id: "cus_beta",
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

  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Needs attention" })).toBeVisible();
  await expect(page.getByText("customers waiting on payment setup")).toBeVisible();
  await expect(page.getByText("Customers with billing sync errors")).toBeVisible();
  await expect(page.getByText("Billing-ready customers")).toBeVisible();

  await expect(page.getByText("customers waiting on payment setup")).toBeVisible();
  await expect(page.getByText("customers with billing sync errors")).toBeVisible();
  await expect(page.getByText("billing-ready customers")).toBeVisible();
});
