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

type CustomerBillingProfile = {
  customer_id: string;
  tenant_id?: string;
  legal_name?: string;
  email?: string;
  phone?: string;
  billing_address_line1?: string;
  billing_address_line2?: string;
  billing_city?: string;
  billing_state?: string;
  billing_postal_code?: string;
  billing_country?: string;
  currency?: string;
  tax_identifier?: string;
  provider_code?: string;
  profile_status: string;
  last_synced_at?: string;
  last_sync_error?: string;
  created_at: string;
  updated_at: string;
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
  const billingProfilesByCustomer: Record<string, CustomerBillingProfile> = {
    cust_alpha: {
      customer_id: "cust_row_1",
      tenant_id: "tenant_a",
      legal_name: "Customer Alpha LLC",
      email: "billing@alpha.test",
      billing_address_line1: "1 Billing Street",
      billing_city: "Bengaluru",
      billing_postal_code: "560001",
      billing_country: "IN",
      currency: "USD",
      provider_code: "stripe_default",
      profile_status: "ready",
      last_synced_at: new Date().toISOString(),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    cust_beta: {
      customer_id: "cust_row_2",
      tenant_id: "tenant_a",
      legal_name: "Customer Beta LLC",
      email: "billing@beta.test",
      billing_address_line1: "2 Billing Street",
      billing_city: "Mumbai",
      billing_postal_code: "400001",
      billing_country: "IN",
      currency: "USD",
      provider_code: "stripe_default",
      profile_status: "sync_error",
      last_synced_at: new Date().toISOString(),
      last_sync_error: "provider timeout",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
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
    if (path.startsWith("/v1/customers/") && path.endsWith("/billing-profile")) {
      const externalID = decodeURIComponent(path.split("/")[3] || "");
      const profile = billingProfilesByCustomer[externalID];
      if (!profile) return json(404, { error: "not found" });
      if (method === "GET") {
        return json(200, profile);
      }
      if (method === "PUT") {
        const body = (request.postDataJSON() as Record<string, string | undefined>) || {};
        const updatedAt = new Date().toISOString();
        billingProfilesByCustomer[externalID] = {
          ...profile,
          ...body,
          profile_status: "ready",
          last_sync_error: "",
          last_synced_at: updatedAt,
          updated_at: updatedAt,
        };
        readinessByCustomer[externalID] = {
          ...readinessByCustomer[externalID],
          billing_profile_status: "ready",
          billing_profile: {
            profile_status: "ready",
            last_synced_at: updatedAt,
            last_sync_error: "",
          },
        };
        return json(200, billingProfilesByCustomer[externalID]);
      }
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
  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Customer directory" })).toBeVisible();
  await expect(page.getByRole("link", { name: "New customer" })).toBeVisible();
  await expect(page.getByRole("link", { name: /Customer Alpha/i })).toBeVisible();
  await expect(page.getByText("Collection is blocked until the customer completes the payment setup path and verification succeeds.")).toBeVisible();

  await page.getByRole("link", { name: /Customer Alpha/i }).click();
  await expect(page).toHaveURL(/\/customers\/cust_alpha$/);
  await expect(page.getByRole("heading", { name: "Customer Alpha" })).toBeVisible();
  await expect(page.getByText("Customer has not completed payment setup").first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "Awaiting customer payment setup" })).toBeVisible();
  await expect(page.getByText("Use one clear setup path, then refresh verification here before retrying collection elsewhere.")).toBeVisible();
});

test("tenant writer can edit the customer billing profile", async ({ page }) => {
  await installCustomersMock(page, {
    authenticated: true,
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    api_key_id: "tenant_writer_1",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/customers/cust_alpha");
  await page.getByTestId("session-login-email").fill("tenant-writer@alpha.test");
  await page.getByTestId("session-login-password").fill("correct horse battery");
  await page.getByTestId("session-login-submit").click();

  await expect(page.getByRole("heading", { name: "Customer Alpha" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Commercial and tax settings" })).toBeVisible();
  await expect(page.getByLabel("Tax identifier")).toHaveValue("");

  await page.getByLabel("Tax identifier").fill("GSTIN-29ABCDE1234F2Z5");
  await page.getByLabel("Phone").fill("+91 80 5555 0100");
  await page.getByRole("button", { name: "Save billing profile" }).click();

  await expect(page.getByLabel("Tax identifier")).toHaveValue("GSTIN-29ABCDE1234F2Z5");
  await expect(page.getByLabel("Phone")).toHaveValue("+91 80 5555 0100");
});
