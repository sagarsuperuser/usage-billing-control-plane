import { expect, test, type Page } from "@playwright/test";

type BillingMockWindow = Window & typeof globalThis & {
  __invoiceMock: {
    retryCSRF: string;
  };
};

const sessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
};

const invoiceDetail = {
  invoice_id: "inv_123",
  invoice_number: "INV-123",
  customer_external_id: "cust_123",
  customer_display_name: "Acme Corp",
  organization_id: "org_test_1",
  currency: "USD",
  invoice_status: "finalized",
  payment_status: "failed",
  payment_overdue: true,
  total_amount_cents: 1200,
  total_due_amount_cents: 1200,
  total_paid_amount_cents: 0,
  last_payment_error: "card_declined",
  last_event_type: "invoice.payment_failure",
  last_event_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  billing_entity_code: "be_default",
  invoice_type: "subscription",
  lifecycle: {
    tenant_id: "tenant_a",
    organization_id: "org_test_1",
    invoice_id: "inv_123",
    invoice_status: "finalized",
    payment_status: "failed",
    payment_overdue: true,
    last_payment_error: "card_declined",
    last_event_type: "invoice.payment_failure",
    last_event_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    events_analyzed: 3,
    event_window_limit: 50,
    event_window_truncated: false,
    distinct_webhook_types: ["invoice.payment_failure"],
    failure_event_count: 1,
    success_event_count: 0,
    pending_event_count: 0,
    overdue_signal_count: 1,
    requires_action: true,
    retry_recommended: false,
    recommended_action: "collect_payment",
    recommended_action_note: "Customer payment setup is not ready. Send or refresh payment setup before retrying collection.",
  },
};

async function installInvoiceDetailMock(page: Page) {
  await page.addInitScript(({ session, detail }: { session: typeof sessionPayload; detail: typeof invoiceDetail }) => {
    let loggedIn = true;
    const originalFetch = window.fetch.bind(window);
    const w = window as BillingMockWindow;
    w.__invoiceMock = { retryCSRF: "" };

    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: { "Content-Type": "application/json" },
      });

    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : null;
      const method = (init?.method || request?.method || "GET").toUpperCase();
      const rawURL = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      const path = new URL(rawURL, window.location.origin).pathname;
      const headers = new Headers(init?.headers || request?.headers);

      if (path === "/v1/ui/sessions/me" && method === "GET") {
        return loggedIn ? json(200, session) : json(401, { error: "unauthorized" });
      }
      if (path === "/v1/ui/sessions/login" && method === "POST") {
        loggedIn = true;
        return json(201, session);
      }
      if (path === "/v1/invoices/inv_123" && method === "GET") {
        return json(200, detail);
      }
      if (path === "/v1/invoices/inv_123/payment-receipts" && method === "GET") {
        return json(200, { items: [] });
      }
      if (path === "/v1/invoices/inv_123/credit-notes" && method === "GET") {
        return json(200, { items: [] });
      }
      if (path === "/v1/invoices/inv_123/retry-payment" && method === "POST") {
        w.__invoiceMock.retryCSRF = headers.get("X-CSRF-Token") || "";
        return json(200, { accepted: true });
      }
      return originalFetch(input, init);
    };
  }, { session: sessionPayload, detail: invoiceDetail });
}

test("invoice detail follows lifecycle guidance instead of exposing retry prematurely", async ({ page }) => {
  await installInvoiceDetailMock(page);
  await page.goto("/invoices/inv_123");

  await expect(page.getByText("Collect payment before retrying")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open payment setup" }).first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry payment" })).toHaveCount(0);
  await expect(page.getByText("invoice payment failure")).toBeVisible();
});
