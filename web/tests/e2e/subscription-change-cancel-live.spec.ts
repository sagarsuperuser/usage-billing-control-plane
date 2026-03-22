import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const subscriptionID = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID || "";
const subscriptionCode = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE || "";
const currentPlanName = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_NAME || "";
const currentPlanCode = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_CURRENT_PLAN_CODE || "";
const targetPlanID = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_ID || "";
const targetPlanName = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_NAME || "";
const targetPlanCode = process.env.PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_TARGET_PLAN_CODE || "";
const lagoAPIURL = (process.env.LAGO_API_URL || "").replace(/\/+$/, "");
const lagoAPIKey = process.env.LAGO_API_KEY || "";

async function waitForLagoSubscription(
  request: import("@playwright/test").APIRequestContext,
  code: string,
  expected: { plan_code?: string; status?: string },
  statusParam?: string,
) {
  const url = `${lagoAPIURL}/api/v1/subscriptions/${encodeURIComponent(code)}${statusParam ? `?status=${encodeURIComponent(statusParam)}` : ""}`;
  await expect
    .poll(
      async () => {
        const response = await request.get(url, {
          failOnStatusCode: false,
          headers: {
            Authorization: `Bearer ${lagoAPIKey}`,
          },
        });
        if (!response.ok()) {
          return { ok: false, http_status: response.status() };
        }
        const payload = (await response.json()) as { subscription?: { external_id?: string; plan_code?: string; status?: string } };
        return {
          ok: true,
          external_id: payload.subscription?.external_id ?? null,
          plan_code: payload.subscription?.plan_code ?? null,
          status: payload.subscription?.status ?? null,
        };
      },
      { timeout: 120000, intervals: [1000, 2000, 3000, 5000] },
    )
    .toMatchObject({ ok: true, external_id: code, ...expected });
}

test.describe("subscription change and cancellation live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live subscription change journey");
  test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required");

  test("writer can change plan and cancel subscription with Lago staying consistent", async ({ page, request }) => {
    test.setTimeout(300000);
    test.skip(!subscriptionID, "PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_ID is required");
    test.skip(!subscriptionCode, "PLAYWRIGHT_LIVE_SUBSCRIPTION_CHANGE_SUBSCRIPTION_CODE is required");
    test.skip(!currentPlanName || !currentPlanCode, "current plan envs are required");
    test.skip(!targetPlanID || !targetPlanName || !targetPlanCode, "target plan envs are required");
    test.skip(!lagoAPIURL || !lagoAPIKey, "LAGO_API_URL and LAGO_API_KEY are required for Lago verification");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: `/subscriptions/${subscriptionID}`,
    });

    await expect(page).toHaveURL(new RegExp(`/subscriptions/${subscriptionID}(?:\\?.*)?$`));
    await expect(page.getByRole("heading", { name: /change plan or cancel billing/i })).toBeVisible();
    await expect(page.getByTestId("subscription-plan-name")).toHaveText(currentPlanName);
    await expect(page.getByTestId("subscription-plan-code")).toHaveText(currentPlanCode);

    await page.getByTestId("subscription-plan-select").selectOption(targetPlanID);
    await expect(page.getByText(`Selected plan code: ${targetPlanCode}`)).toBeVisible();
    await page.getByTestId("subscription-change-plan").click();

    await expect(page.getByTestId("subscription-plan-name")).toHaveText(targetPlanName, { timeout: 30000 });
    await expect(page.getByTestId("subscription-plan-code")).toHaveText(targetPlanCode);
    await waitForLagoSubscription(request, subscriptionCode, { plan_code: targetPlanCode });

    await page.getByTestId("subscription-archive").click();
    await expect(page.getByTestId("subscription-status-badge")).toContainText(/archived/i, { timeout: 30000 });
    await waitForLagoSubscription(request, subscriptionCode, { plan_code: targetPlanCode, status: "terminated" }, "terminated");
  });
});
