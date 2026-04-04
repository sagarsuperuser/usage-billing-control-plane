import { expect, test, type Page } from "@playwright/test";

const tenantSession = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
} as const;

const detailPayload = {
  run: {
    id: "drun_123",
    tenant_id: "tenant_a",
    invoice_id: "inv_123",
    customer_external_id: "cust_123",
    policy_id: "policy_1",
    state: "awaiting_payment_setup",
    attempt_count: 1,
    next_action_at: "2026-03-23T12:00:00Z",
    next_action_type: "collect_payment_reminder",
    paused: false,
    created_at: "2026-03-23T10:00:00Z",
    updated_at: "2026-03-23T10:05:00Z",
  },
  events: [
    {
      id: "devt_1",
      run_id: "drun_123",
      tenant_id: "tenant_a",
      invoice_id: "inv_123",
      customer_external_id: "cust_123",
      event_type: "payment_failed",
      state: "awaiting_payment_setup",
      action_type: "collect_payment_reminder",
      reason: "payment_failure",
      attempt_count: 1,
      created_at: "2026-03-23T10:06:00Z",
    },
  ],
  notification_intents: [
    {
      id: "intent_1",
      run_id: "drun_123",
      tenant_id: "tenant_a",
      invoice_id: "inv_123",
      customer_external_id: "cust_123",
      intent_type: "collect_payment_reminder",
      action_type: "collect_payment_reminder",
      status: "queued",
      delivery_backend: "email",
      recipient_email: "billing@acme.test",
      created_at: "2026-03-23T10:07:00Z",
    },
  ],
} as const;

async function installDunningRunDetailMock(page: Page, session: unknown) {
  await page.addInitScript(
    ({ session, detail }: { session: unknown; detail: typeof detailPayload }) => {
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
        if (path === "/v1/dunning/runs/drun_123" && method === "GET") {
          return json(200, detail);
        }

        return originalFetch(input, init);
      };
    },
    { session, detail: detailPayload },
  );
}

test("shows normalized diagnosis guidance on dunning run detail", async ({ page }) => {
  await installDunningRunDetailMock(page, tenantSession);

  await page.goto("/dunning/drun_123");

  await expect(page.getByRole("heading", { name: "Dunning run" })).toBeVisible();
  await expect(page.getByText("Awaiting payment setup", { exact: true })).toBeVisible();
  await expect(page.getByText("inv_123").first()).toBeVisible();
  await expect(page.getByRole("button", { name: /Actions/ })).toBeVisible();
});

test("blocks platform sessions from workspace-scoped dunning run detail", async ({ page }) => {
  await installDunningRunDetailMock(page, {
    authenticated: true,
    scope: "platform",
    platform_role: "platform_admin",
    api_key_id: "platform_ui_1",
    csrf_token: "csrf-platform-123",
  });

  await page.goto("/dunning/drun_123");

  await expect(page.getByText("Workspace session required")).toBeVisible();
  await expect(page.getByText("Dunning run detail is workspace-scoped.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Open platform home" })).toBeVisible();
});
