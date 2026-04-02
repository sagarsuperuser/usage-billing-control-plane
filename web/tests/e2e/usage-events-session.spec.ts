import { expect, test, type Page } from "@playwright/test";

type TenantSessionPayload = {
  authenticated: boolean;
  scope: "tenant";
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
};

type UsageEventRecord = {
  id: string;
  tenant_id?: string;
  customer_id: string;
  meter_id: string;
  subscription_id?: string;
  quantity: number;
  idempotency_key?: string;
  timestamp: string;
};

async function installUsageEventsMock(page: Page, session: TenantSessionPayload) {
  let loggedIn = true;
  const items: UsageEventRecord[] = [
    {
      id: "evt_2",
      tenant_id: "tenant_a",
      customer_id: "cust_beta",
      meter_id: "mtr_api_calls",
      subscription_id: "sub_beta",
      quantity: 7,
      idempotency_key: "idem_evt_2",
      timestamp: "2026-03-23T10:30:00Z",
    },
    {
      id: "evt_1",
      tenant_id: "tenant_a",
      customer_id: "cust_alpha",
      meter_id: "mtr_api_calls",
      subscription_id: "sub_alpha",
      quantity: 12,
      idempotency_key: "idem_evt_1",
      timestamp: "2026-03-23T10:00:00Z",
    },
  ];

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
    if (path === "/v1/usage-events" && method === "GET") {
      const customerID = url.searchParams.get("customer_id") || "";
      const meterID = url.searchParams.get("meter_id") || "";
      let filtered = items.slice();
      if (customerID) {
        filtered = filtered.filter((item) => item.customer_id === customerID);
      }
      if (meterID) {
        filtered = filtered.filter((item) => item.meter_id === meterID);
      }
      return json(200, { items: filtered, limit: 100, offset: 0, next_cursor: "" });
    }

    return route.continue();
  });
}

test("tenant operator can browse raw usage events", async ({ page }) => {
  await installUsageEventsMock(page, {
    authenticated: true,
    scope: "tenant",
    role: "admin",
    tenant_id: "tenant_a",
    api_key_id: "ak_tenant_admin",
    csrf_token: "csrf-usage-events",
  });

  await page.goto("/usage-events");

  await expect(page.getByRole("heading", { name: "Usage events" })).toBeVisible();
  await expect(page.getByText("cust_beta")).toBeVisible();
  await expect(page.getByText("cust_alpha")).toBeVisible();
  await expect(page.getByText("evt_2")).toHaveCount(0);
  await expect(page.getByText("Visible quantity")).toBeVisible();
  await expect(page.getByText("19")).toBeVisible();

  await page.getByLabel("Customer ID").fill("cust_alpha");
  await page.getByRole("button", { name: "Apply filters" }).click();

  await expect(page.getByText("cust_alpha")).toBeVisible();
  await expect(page.getByText("cust_beta")).toHaveCount(0);
  await expect(page.getByText("evt_1")).toHaveCount(0);

  await page.getByRole("button", { name: "View details for usage event evt_1" }).click();
  await expect(page.getByText("evt_1", { exact: true })).toBeVisible();
  await expect(page.getByText("idem_evt_1")).toBeVisible();
});
