import { expect, test, type BrowserContext } from "@playwright/test";

type TenantSessionPayload = {
  authenticated: boolean;
  subject_type: "user";
  subject_id: string;
  user_email: string;
  scope: "tenant";
  role: "writer";
  tenant_id: string;
  csrf_token: string;
};

type PricingMetric = {
  id: string;
  key: string;
  name: string;
  unit: string;
  aggregation: string;
  rating_rule_version_id: string;
  created_at: string;
  updated_at: string;
};

type Plan = {
  id: string;
  code: string;
  name: string;
  description?: string;
  currency: string;
  billing_interval: "monthly" | "yearly";
  status: "draft" | "active" | "archived";
  base_amount_cents: number;
  meter_ids: string[];
  created_at: string;
  updated_at: string;
};

async function installPricingMock(context: BrowserContext, session: TenantSessionPayload) {
  let metrics: PricingMetric[] = [];
  let plans: Plan[] = [];

  await context.route("**/runtime-config", async (route) => {
    const url = new URL(route.request().url());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ apiBaseURL: url.origin }),
    });
  });

  await context.route("**/v1/ui/sessions/me", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(session),
    });
  });

  await context.route("**/v1/pricing/metrics", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(metrics),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, string>;
      const now = new Date().toISOString();
      const metric: PricingMetric = {
        id: "mtr_metric_1",
        key: body.key || "api_calls",
        name: body.name || "API Calls",
        unit: body.unit || "request",
        aggregation: body.aggregation || "sum",
        rating_rule_version_id: "rrv_metric_1",
        created_at: now,
        updated_at: now,
      };
      metrics = [metric];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(metric),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/pricing/metrics/mtr_metric_1", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(metrics[0]),
    });
  });

  await context.route("**/v1/plans", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(plans),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      const now = new Date().toISOString();
      const plan: Plan = {
        id: "pln_growth_1",
        code: String(body.code || "growth"),
        name: String(body.name || "Growth"),
        description: String(body.description || ""),
        currency: String(body.currency || "USD"),
        billing_interval: (body.billing_interval as "monthly" | "yearly") || "monthly",
        status: (body.status as "draft" | "active" | "archived") || "draft",
        base_amount_cents: Number(body.base_amount_cents || 4900),
        meter_ids: (body.meter_ids as string[]) || [],
        created_at: now,
        updated_at: now,
      };
      plans = [plan];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(plan),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/plans/pln_growth_1", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(plans[0]),
    });
  });
}

test("tenant writer can create pricing metric and plan", async ({ page, context }) => {
  await installPricingMock(context, {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_tenant_1",
    user_email: "tenant-writer@alpha.test",
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/pricing/metrics/new");
  await page.getByTestId("pricing-metric-name").fill("API Calls");
  await page.getByTestId("pricing-metric-code").fill("api_calls");
  await expect(page.getByTestId("pricing-metric-submit")).toBeEnabled();
  await page.getByTestId("pricing-metric-submit").click();

  await expect(page).toHaveURL(/\/pricing\/metrics\/mtr_metric_1$/);
  await expect(page.getByRole("heading", { name: "API Calls" })).toBeVisible();

  await page.goto("/pricing/plans/new");
  await page.getByTestId("pricing-plan-name").fill("Growth");
  await page.getByTestId("pricing-plan-code").fill("growth");
  await page.getByTestId("pricing-plan-base-price").fill("49");
  await page.getByTestId("pricing-plan-metric-mtr_metric_1").check();
  await expect(page.getByTestId("pricing-plan-submit")).toBeEnabled();
  await page.getByTestId("pricing-plan-submit").click();

  await expect(page).toHaveURL(/\/pricing\/plans\/pln_growth_1$/);
  await expect(page.getByRole("heading", { name: "Growth" })).toBeVisible();
  await expect(page.getByText("49.00 USD")).toBeVisible();
});
