import { expect, test, type Page } from "@playwright/test";
import type { Locator } from "@playwright/test";

type SessionPayload = {
  authenticated: boolean;
  scope: "tenant";
  role: "reader" | "writer" | "admin";
  tenant_id?: string;
  api_key_id?: string;
  csrf_token: string;
};

type ExplainabilityPayload = {
  invoice_id: string;
  invoice_number: string;
  invoice_status: string;
  currency: string;
  generated_at: string;
  total_amount_cents: number;
  explainability_version: string;
  explainability_digest: string;
  line_items_count: number;
  line_items: Array<{
    fee_id: string;
    fee_type: string;
    item_name: string;
    item_code?: string;
    amount_cents: number;
    taxes_amount_cents: number;
    total_amount_cents: number;
    units?: number;
    events_count?: number;
    computation_mode: string;
    rule_reference: string;
    from_datetime?: string;
    to_datetime?: string;
    properties: Record<string, unknown>;
  }>;
};

const sessionPayload: SessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "reader",
  tenant_id: "tenant_a",
  api_key_id: "api_key_reader_1",
  csrf_token: "csrf-exp-123",
};


const explainabilityPayload: ExplainabilityPayload = {
  invoice_id: "inv_explain_123",
  invoice_number: "INV-EX-123",
  invoice_status: "finalized",
  currency: "USD",
  generated_at: new Date().toISOString(),
  total_amount_cents: 3250,
  explainability_version: "v1",
  explainability_digest: "digest_abc123",
  line_items_count: 2,
  line_items: [
    {
      fee_id: "fee_1",
      fee_type: "charge",
      item_name: "API Calls",
      item_code: "api_calls",
      amount_cents: 2500,
      taxes_amount_cents: 250,
      total_amount_cents: 2750,
      units: 5000,
      events_count: 5000,
      computation_mode: "graduated",
      rule_reference: "api_calls:v3",
      from_datetime: new Date(Date.now() - 86_400_000).toISOString(),
      to_datetime: new Date().toISOString(),
      properties: {
        region: "us-east-1",
      },
    },
    {
      fee_id: "fee_2",
      fee_type: "subscription",
      item_name: "Base Plan",
      item_code: "starter_monthly",
      amount_cents: 500,
      taxes_amount_cents: 0,
      total_amount_cents: 500,
      units: 1,
      events_count: 1,
      computation_mode: "flat",
      rule_reference: "starter_monthly:v1",
      from_datetime: new Date(Date.now() - 86_400_000).toISOString(),
      to_datetime: new Date().toISOString(),
      properties: {
        plan: "starter",
      },
    },
  ],
};

async function installExplainabilityMock(page: Page, session: SessionPayload, payload: ExplainabilityPayload) {
  await page.addInitScript(({ session, payload }: { session: SessionPayload; payload: ExplainabilityPayload }) => {
    let loggedIn = true;

    const json = (status: number, body: unknown) =>
      new Response(JSON.stringify(body), {
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

      if (path === "/v1/ui/sessions/me" && method === "GET") {
        return loggedIn ? json(200, session) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/ui/sessions/login" && method === "POST") {
        loggedIn = true;
        return json(201, session);
      }

      if (path === "/v1/ui/sessions/logout" && method === "POST") {
        loggedIn = false;
        return json(200, { logged_out: true });
      }

      if (path === `/v1/invoices/${encodeURIComponent(payload.invoice_id)}/explainability` && method === "GET") {
        const feeTypes = url.searchParams.get("fee_types") || "";
        if (feeTypes && feeTypes !== "charge,subscription") {
          return json(200, { ...payload, line_items: [], line_items_count: 0 });
        }
        return json(200, payload);
      }

      return originalFetch(input, init);
    };
  }, { session, payload });
}

async function fillUntilValue(locator: Locator, value: string) {
  await expect
    .poll(async () => {
      await locator.fill(value);
      return locator.inputValue();
    })
    .toBe(value);
}

test.beforeEach(async ({ page }) => {
  await installExplainabilityMock(page, sessionPayload, explainabilityPayload);
});

test("reader session can load invoice explainability and inspect line items", async ({ page }) => {
  await page.goto("/invoice-explainability");

  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();

  await expect(page.getByText("Invoice explainability")).toBeVisible();
  await fillUntilValue(page.getByTestId("explainability-invoice-id"), "inv_explain_123");
  await expect(page.getByTestId("explainability-load")).toBeEnabled();
  await page.getByTestId("explainability-load").click();

  await expect(page.getByTestId("explainability-line-item-fee_1")).toBeVisible();
  await expect(page.getByTestId("explainability-meta-invoice")).toContainText("INV-EX-123");
  await expect(page.getByTestId("explainability-line-item-fee_1")).toContainText("API Calls");
  await expect(page.getByTestId("explainability-line-item-fee_2")).toContainText("Base Plan");
  await expect(page.getByText("api_calls:v3")).toHaveCount(0);

  await page.getByTestId("explainability-view-line-item-fee_1").click();
  await expect(page.getByText("api_calls:v3")).toBeVisible();
});

test("refresh keeps explainability data loaded", async ({ page }) => {
  await page.goto("/invoice-explainability");

  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();

  await fillUntilValue(page.getByTestId("explainability-invoice-id"), "inv_explain_123");
  await expect(page.getByTestId("explainability-load")).toBeEnabled();
  await page.getByTestId("explainability-load").click();
  await expect(page.getByTestId("explainability-line-item-fee_1")).toBeVisible();

  await page.getByTestId("explainability-refresh").click();
  await expect(page.getByTestId("explainability-meta-version")).toContainText("v1");
  await expect(page.getByTestId("explainability-meta-total")).toContainText("$32.50");
});

test("reader sees empty state when explainability returns no line items", async ({ page }) => {
  await page.goto("/invoice-explainability");

  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();

  await fillUntilValue(page.getByTestId("explainability-invoice-id"), "inv_explain_123");
  await expect(page.getByTestId("explainability-load")).toBeEnabled();
  await page.getByTestId("explainability-load").click();
  await expect(page.getByTestId("explainability-line-item-fee_1")).toBeVisible();

  await page.getByTestId("explainability-refresh").click();

  await expect(page.getByTestId("explainability-empty")).toContainText("No line items yet");
});
