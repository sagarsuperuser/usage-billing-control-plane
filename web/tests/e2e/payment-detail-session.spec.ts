import { expect, test, type Page } from "@playwright/test";

const sessionPayload = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
};

const paymentDetail = {
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
  dunning: {
    run_id: "drun_123",
    state: "active",
    attempt_count: 1,
    next_action_type: "collect_payment_reminder",
    next_action_at: new Date().toISOString(),
    paused: false,
    last_event_type: "collect_payment_reminder_queued",
    last_event_at: new Date().toISOString(),
    last_notification_intent_type: "collect_payment_reminder",
    last_notification_status: "queued",
    last_notification_at: new Date().toISOString(),
  },
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
    events_analyzed: 4,
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
} as const;

async function installPaymentDetailMock(page: Page) {
  await page.addInitScript(({ session, detail }: { session: typeof sessionPayload; detail: typeof paymentDetail }) => {
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
      if (path === "/v1/payments/inv_123" && method === "GET") {
        return json(200, detail);
      }
      if (path === "/v1/invoice-payment-statuses/inv_123/events" && method === "GET") {
        return json(200, {
          items: [
            {
              id: "evt_1",
              webhook_key: "invoice.payment_failure:inv_123",
              webhook_type: "invoice.payment_failure",
              object_type: "invoice",
              invoice_id: "inv_123",
              payment_status: "failed",
              received_at: "2026-03-23T10:05:00Z",
              occurred_at: "2026-03-23T10:04:00Z",
              payload: {},
            },
          ],
        });
      }
      if (path === "/v1/dunning/runs/drun_123" && method === "GET") {
        return json(200, {
          run: {
            id: "drun_123",
            invoice_id: "inv_123",
            customer_external_id: "cust_123",
            policy_id: "policy_1",
            state: "active",
            attempt_count: 1,
            next_action_at: "2026-03-23T10:15:00Z",
            next_action_type: "collect_payment_reminder",
            paused: false,
            created_at: "2026-03-23T10:00:00Z",
            updated_at: "2026-03-23T10:05:00Z",
          },
          events: [],
          notification_intents: [],
        });
      }

      return originalFetch(input, init);
    };
  }, { session: sessionPayload, detail: paymentDetail });
}

test("payment detail shows evidence-backed failure reasoning", async ({ page }) => {
  await installPaymentDetailMock(page);

  await page.goto("/payments/inv_123");

  const evidence = page.locator("section").filter({ has: page.getByText("Why Alpha thinks this failed") });

  await expect(page.getByText("Payment collection is blocked")).toBeVisible();
  await expect(evidence.getByText("Why Alpha thinks this failed")).toBeVisible();
  await expect(evidence.getByText("Recommended action")).toBeVisible();
  await expect(evidence.getByText("collect payment", { exact: true })).toBeVisible();
  await expect(evidence.getByText("Last payment error")).toBeVisible();
  await expect(evidence.getByText("card_declined")).toBeVisible();
  await expect(evidence.getByText("Dunning state")).toBeVisible();
  await expect(evidence.getByText("active", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open customer collection path" }).first()).toBeVisible();
});
