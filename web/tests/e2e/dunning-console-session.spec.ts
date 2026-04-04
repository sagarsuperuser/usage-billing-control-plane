import { expect, test, type Page } from "@playwright/test";

const tenantSession = {
  authenticated: true,
  scope: "tenant",
  role: "writer",
  tenant_id: "tenant_a",
  api_key_id: "api_key_writer_1",
  csrf_token: "csrf-abc-123",
} as const;

const policyPayload = {
  id: "policy_1",
  tenant_id: "tenant_a",
  name: "Default collections",
  enabled: true,
  retry_schedule: ["1d", "3d"],
  max_retry_attempts: 3,
  collect_payment_reminder_schedule: ["0d", "2d"],
  final_action: "manual_review",
  grace_period_days: 0,
  created_at: "2026-03-23T10:00:00Z",
  updated_at: "2026-03-23T10:05:00Z",
} as const;

const runsPayload = {
  items: [
    {
      id: "drun_setup",
      tenant_id: "tenant_a",
      invoice_id: "inv_setup",
      customer_external_id: "cust_setup",
      policy_id: "policy_1",
      state: "awaiting_payment_setup",
      attempt_count: 1,
      next_action_at: "2026-03-23T12:00:00Z",
      next_action_type: "collect_payment_reminder",
      paused: false,
      created_at: "2026-03-23T10:00:00Z",
      updated_at: "2026-03-23T10:05:00Z",
    },
    {
      id: "drun_escalated",
      tenant_id: "tenant_a",
      invoice_id: "inv_escalated",
      customer_external_id: "cust_escalated",
      policy_id: "policy_1",
      state: "escalated",
      attempt_count: 4,
      next_action_at: "2026-03-23T13:00:00Z",
      next_action_type: "retry_payment",
      paused: false,
      created_at: "2026-03-22T10:00:00Z",
      updated_at: "2026-03-23T09:05:00Z",
    },
    {
      id: "drun_paused",
      tenant_id: "tenant_a",
      invoice_id: "inv_paused",
      customer_external_id: "cust_paused",
      policy_id: "policy_1",
      state: "retry_due",
      attempt_count: 2,
      next_action_at: "2026-03-23T14:00:00Z",
      next_action_type: "retry_payment",
      paused: true,
      created_at: "2026-03-22T11:00:00Z",
      updated_at: "2026-03-23T08:00:00Z",
    },
  ],
  limit: 100,
  offset: 0,
} as const;

async function installDunningConsoleMock(page: Page, session: unknown) {
  await page.addInitScript(
    ({ session, policy, runs }: { session: unknown; policy: typeof policyPayload; runs: typeof runsPayload }) => {
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
        if (path === "/v1/dunning/policy" && method === "GET") {
          return json(200, { policy });
        }
        if (path === "/v1/dunning/runs" && method === "GET") {
          return json(200, runs);
        }

        return originalFetch(input, init);
      };
    },
    { session, policy: policyPayload, runs: runsPayload },
  );
}

test("shows normalized dunning diagnosis guidance in the run inventory", async ({ page }) => {
  await installDunningConsoleMock(page, tenantSession);

  await page.goto("/dunning");

  const awaitingRow = page.locator("tr", { hasText: "inv_setup" });
  const escalatedRow = page.locator("tr", { hasText: "inv_escalated" });
  const pausedRow = page.locator("tr", { hasText: "inv_paused" });

  await expect(awaitingRow.getByText("awaiting payment setup", { exact: true })).toBeVisible();
  await expect(escalatedRow).toBeVisible();
});

