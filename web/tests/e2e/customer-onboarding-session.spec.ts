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
    legal_name?: string;
    last_sync_error?: string;
    last_synced_at?: string;
    profile_status: string;
  };
  payment_setup: {
    setup_status: string;
    last_verified_at?: string;
  };
};

const sessionPayload: TenantSessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "tenant_writer_1",
  csrf_token: "csrf-tenant-123",
};

function buildReadiness(): CustomerReadiness {
  const now = new Date().toISOString();
  return {
    status: "pending",
    missing_steps: ["payment_setup_ready", "default_payment_method_verified"],
    customer_exists: true,
    customer_active: true,
    billing_provider_configured: true,
    provider_customer_synced: true,
    default_payment_method_verified: false,
    billing_profile_status: "ready",
    payment_setup_status: "pending",
    billing_profile: {
      legal_name: "Acme Primary Customer LLC",
      profile_status: "ready",
      last_synced_at: now,
      last_sync_error: "",
    },
    payment_setup: {
      setup_status: "pending",
      last_verified_at: now,
    },
  };
}

async function installCustomerOnboardingMock(page: Page, session: TenantSessionPayload) {
  let loggedIn = false;
  let customers: CustomerRecord[] = [];
  const readinessByCustomer: Record<string, CustomerReadiness> = {};
  let capturedCSRF = "";

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
      const externalID = url.searchParams.get("external_id");
      if (externalID) {
        return json(200, customers.filter((item) => item.external_id === externalID));
      }
      return json(200, customers);
    }
    if (path === "/v1/customer-onboarding" && method === "POST") {
      capturedCSRF = request.headers()["x-csrf-token"] || "";
      const body = request.postDataJSON() as Record<string, unknown>;
      const now = new Date().toISOString();
      const customer: CustomerRecord = {
        id: "cust_row_1",
        external_id: String(body.external_id ?? ""),
        display_name: String(body.display_name ?? ""),
        email: typeof body.email === "string" ? body.email : undefined,
        status: "active",
        provider_customer_id: "cus_1",
        created_at: now,
        updated_at: now,
      };
      customers = [customer, ...customers.filter((item) => item.external_id !== customer.external_id)];
      readinessByCustomer[customer.external_id] = buildReadiness();
      return json(201, {
        customer,
        customer_created: true,
        billing_profile_applied: true,
        payment_setup_started: true,
        checkout_url: "https://checkout.example.test/cust_acme_primary",
        billing_profile: readinessByCustomer[customer.external_id].billing_profile,
        payment_setup: readinessByCustomer[customer.external_id].payment_setup,
        readiness: readinessByCustomer[customer.external_id],
      });
    }
    if (path.startsWith("/v1/customers/") && path.endsWith("/readiness") && method === "GET") {
      const externalID = decodeURIComponent(path.split("/")[3] || "");
      const readiness = readinessByCustomer[externalID];
      if (!readiness) return json(404, { error: "not found" });
      return json(200, readiness);
    }
    if (path === "/runtime-config" && method === "GET") {
      return json(200, { apiBaseURL: "" });
    }

    return route.continue();
  });

  return {
    getCapturedCSRF: () => capturedCSRF,
  };
}

test("tenant writer can onboard a customer from the UI", async ({ page }) => {
  const mock = await installCustomerOnboardingMock(page, sessionPayload);

  await page.goto("/customer-onboarding");
  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Create customer" })).toBeVisible();
  await page.getByLabel("Customer external ID").fill("cust_acme_primary");
  await page.getByLabel("Display name").fill("Acme Primary Customer");
  await page.getByLabel("Billing email").fill("billing@acme.test");
  await page.getByLabel("Legal name").fill("Acme Primary Customer LLC");
  await page.getByLabel("Billing address line 1").fill("1 Billing Street");
  await page.getByLabel("Billing city").fill("Bengaluru");
  await page.getByLabel("Billing postal code").fill("560001");
  await page.getByLabel("Billing country").fill("IN");
  await page.getByLabel("Currency").fill("USD");
  await expect(page.getByText("Payment setup", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Run customer setup" }).click();

  await expect.poll(() => mock.getCapturedCSRF()).toBe("csrf-tenant-123");

  await page.goto("/customers/cust_acme_primary");
  await expect(page.getByRole("heading", { name: "Acme Primary Customer" })).toBeVisible();
  await expect(page.getByText("Billing details")).toBeVisible();
});
