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

type CustomerOnboardingMockWindow = Window & typeof globalThis & {
  __customerOnboardingMock: {
    csrf: string;
    retryExternalID: string;
    refreshExternalID: string;
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
    lago_customer_synced: true,
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
  await page.addInitScript(({ session }: { session: TenantSessionPayload }) => {
    let loggedIn = true;
    let customers: CustomerRecord[] = [];
    const readinessByCustomer: Record<string, CustomerReadiness> = {};
    const w = window as CustomerOnboardingMockWindow;
    w.__customerOnboardingMock = {
      csrf: "",
      retryExternalID: "",
      refreshExternalID: "",
    };

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

      if (path === "/v1/customers" && method === "GET") {
        return loggedIn ? json(200, customers) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/customer-onboarding" && method === "POST") {
        const csrf = headers.get("X-CSRF-Token") || "";
        w.__customerOnboardingMock.csrf = csrf;
        const body = JSON.parse(String(init?.body || "{}"));
        const now = new Date().toISOString();
        const customer: CustomerRecord = {
          id: "cust_row_1",
          external_id: body.external_id,
          display_name: body.display_name,
          email: body.email,
          status: "active",
          lago_customer_id: "lago_cust_1",
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
        if (!readiness) {
          return json(404, { error: "not found" });
        }
        return json(200, readiness);
      }

      if (path.startsWith("/v1/customers/") && path.endsWith("/billing-profile/retry-sync") && method === "POST") {
        const externalID = decodeURIComponent(path.split("/")[3] || "");
        w.__customerOnboardingMock.retryExternalID = externalID;
        return json(200, {
          external_id: externalID,
          billing_profile: readinessByCustomer[externalID].billing_profile,
          payment_setup: readinessByCustomer[externalID].payment_setup,
          readiness: readinessByCustomer[externalID],
        });
      }

      if (path.startsWith("/v1/customers/") && path.endsWith("/payment-setup/refresh") && method === "POST") {
        const externalID = decodeURIComponent(path.split("/")[3] || "");
        w.__customerOnboardingMock.refreshExternalID = externalID;
        const ready = {
          ...readinessByCustomer[externalID],
          status: "ready",
          missing_steps: [],
          default_payment_method_verified: true,
          payment_setup_status: "ready",
          payment_setup: {
            ...readinessByCustomer[externalID].payment_setup,
            setup_status: "ready",
          },
        };
        readinessByCustomer[externalID] = ready;
        return json(200, {
          external_id: externalID,
          payment_setup: ready.payment_setup,
          readiness: ready,
        });
      }

      return originalFetch(input, init);
    };
  }, { session });
}

test.beforeEach(async ({ page }) => {
  await installCustomerOnboardingMock(page, sessionPayload);
});

test("tenant writer can onboard a customer from the UI", async ({ page }) => {
  await page.goto("/customer-onboarding");

  await expect(page.getByRole("heading", { name: "Customer Onboarding" })).toBeVisible();
  await page.getByLabel("Customer external ID").fill("cust_acme_primary");
  await page.getByLabel("Display name").fill("Acme Primary Customer");
  await page.getByLabel("Billing email").fill("billing@acme.test");
  await page.getByLabel("Legal name").fill("Acme Primary Customer LLC");
  await page.getByLabel("Billing address line 1").fill("1 Billing Street");
  await page.getByLabel("Billing city").fill("Bengaluru");
  await page.getByLabel("Billing postal code").fill("560001");
  await page.getByLabel("Billing country").fill("IN");
  await page.getByLabel("Currency").fill("USD");
  await page.getByLabel("Provider code").fill("stripe_default");
  await page.getByRole("button", { name: "Run customer onboarding" }).click();

  await expect.poll(async () =>
    page.evaluate(() => (window as CustomerOnboardingMockWindow).__customerOnboardingMock.csrf)
  ).toBe("csrf-tenant-123");

  await page.getByRole("button", { name: "Refresh" }).click();
  await expect(page.getByRole("button", { name: /Acme Primary Customer/i })).toBeVisible();
  await page.getByRole("button", { name: /Acme Primary Customer/i }).click();
  await expect(page.getByText("Acme Primary Customer")).toBeVisible();
  await expect(page.getByText("cust_acme_primary")).toBeVisible();
});
