import { expect, test, type Page } from "@playwright/test";

type BillingMockWindow = Window & typeof globalThis & {
  __billingMock: {
    retryCSRF: string;
  };
};

type SessionPayload = {
  authenticated: boolean;
  role: "reader" | "writer" | "admin";
  tenant_id: string;
  api_key_id: string;
  csrf_token: string;
};

type InitPayload = {
  session: SessionPayload;
  summary: typeof summaryPayload;
  row: typeof invoiceRow;
};

const sessionPayload: SessionPayload = {
  authenticated: true,
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
};

const summaryPayload = {
  total_invoices: 1,
  overdue_count: 1,
  attention_required_count: 1,
  stale_attention_required: 0,
  payment_status_counts: {
    failed: 1,
  },
  invoice_status_counts: {
    finalized: 1,
  },
};

const invoiceRow = {
  tenant_id: "tenant_a",
  organization_id: "org_test_1",
  invoice_id: "inv_123",
  invoice_number: "INV-123",
  currency: "USD",
  invoice_status: "finalized",
  payment_status: "failed",
  payment_overdue: true,
  total_amount_cents: 1200,
  total_due_amount_cents: 1200,
  total_paid_amount_cents: 0,
  last_event_type: "invoice.payment_failure",
  last_event_at: new Date().toISOString(),
  last_webhook_key: "whk_123",
  updated_at: new Date().toISOString(),
};

async function installPaymentOpsMock(page: Page, session: SessionPayload) {
  await page.addInitScript(({ session, summary, row }: InitPayload) => {
    let loggedIn = true;
    let retryCSRF = "";

    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: {
          "Content-Type": "application/json",
        },
      });

    const originalFetch = window.fetch.bind(window);
    const w = window as BillingMockWindow;
    w.__billingMock = { retryCSRF: "" };

    window.fetch = async (input, init) => {
      const request = input instanceof Request ? input : null;
      const method = (init?.method || request?.method || "GET").toUpperCase();
      const rawURL =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.toString()
            : input.url;
      const path = new URL(rawURL, window.location.origin).pathname;
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

      if (path === "/v1/invoice-payment-statuses" && method === "GET") {
        return loggedIn ? json(200, { items: [row], limit: 25, offset: 0 }) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/invoice-payment-statuses/summary" && method === "GET") {
        return loggedIn ? json(200, summary) : json(401, { error: "unauthorized" });
      }

      if (path === "/v1/invoices/inv_123/retry-payment" && method === "POST") {
        retryCSRF = headers.get("X-CSRF-Token") || "";
        w.__billingMock.retryCSRF = retryCSRF;
        return json(200, {
          invoice: {
            lago_id: "inv_123",
            payment_status: "pending",
          },
        });
      }

      return originalFetch(input, init);
    };
  }, { session, summary: summaryPayload, row: invoiceRow });
}

test.beforeEach(async ({ page }) => {
  await installPaymentOpsMock(page, sessionPayload);
});

test("supports session logout in payment operations UI", async ({ page }) => {
  await page.goto("/payment-operations");

  await expect(page.getByTestId("session-logout")).toBeVisible();
  await expect(page.getByText("INV-123")).toBeVisible();

  await page.getByTestId("session-logout").click();
  await expect(page.getByTestId("session-login-submit")).toBeVisible();
});

test("sends CSRF token when retrying failed payment", async ({ page }) => {
  await page.goto("/payment-operations");

  await expect(page.getByText("INV-123")).toBeVisible();
  await page.getByRole("button", { name: "Retry" }).click();

  await expect.poll(async () => page.evaluate(() => (window as BillingMockWindow).__billingMock.retryCSRF)).toBe("csrf-abc-123");
  await expect(page.getByText("Retry request sent to billing engine for invoice")).toBeVisible();
});

test("disables retry for reader sessions", async ({ page }) => {
  await installPaymentOpsMock(page, {
    ...sessionPayload,
    role: "reader",
    api_key_id: "api_key_reader_1",
  });

  await page.goto("/payment-operations");

  const retryButton = page.getByRole("button", { name: "Retry" }).first();
  await expect(retryButton).toBeDisabled();
  await expect(page.getByText("Current session role reader is read-only for payment retry operations.")).toBeVisible();
});
