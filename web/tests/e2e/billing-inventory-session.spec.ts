import { expect, test, type Page } from "@playwright/test";

const sessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
};

const paymentRow = {
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
};

const invoiceRow = {
  invoice_id: "inv_456",
  invoice_number: "INV-456",
  customer_external_id: "cust_456",
  customer_display_name: "Beta Corp",
  organization_id: "org_test_1",
  currency: "USD",
  invoice_status: "finalized",
  payment_status: "failed",
  payment_overdue: true,
  total_amount_cents: 2400,
  total_due_amount_cents: 2400,
  total_paid_amount_cents: 0,
  last_payment_error: "card_declined",
  last_event_type: "invoice.payment_failure",
  last_event_at: new Date().toISOString(),
  issuing_date: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

async function installInventoryMock(page: Page) {
  await page.addInitScript(
    ({ session, payment, invoice }: { session: typeof sessionPayload; payment: typeof paymentRow; invoice: typeof invoiceRow }) => {
      let loggedIn = true;
      const originalFetch = window.fetch.bind(window);

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

        if (path === "/v1/ui/sessions/me" && method === "GET") {
          return loggedIn ? json(200, session) : json(401, { error: "unauthorized" });
        }
        if (path === "/v1/ui/sessions/login" && method === "POST") {
          loggedIn = true;
          return json(201, session);
        }
        if (path === "/v1/payments" && method === "GET") {
          return json(200, { items: [payment] });
        }
        if (path === "/v1/invoices" && method === "GET") {
          return json(200, { items: [invoice] });
        }

        return originalFetch(input, init);
      };
    },
    { session: sessionPayload, payment: paymentRow, invoice: invoiceRow },
  );
}

test("payment inventory surfaces compact failure diagnosis before detail drill-in", async ({ page }) => {
  await installInventoryMock(page);
  await page.goto("/payments");

  await expect(page.getByText("INV-123")).toBeVisible();
  await expect(page.getByText("Payment failed")).toBeVisible();
  await expect(page.getByText("card_declined")).toBeVisible();
});

test("invoice inventory surfaces compact failure diagnosis before detail drill-in", async ({ page }) => {
  await installInventoryMock(page);
  await page.goto("/invoices");

  await expect(page.getByText("INV-456")).toBeVisible();
  await expect(page.getByText("Payment failed")).toBeVisible();
  await expect(page.getByText("card_declined")).toBeVisible();
});
