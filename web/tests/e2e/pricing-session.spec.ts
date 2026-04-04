import { expect, test, type BrowserContext, type Locator, type Page } from "@playwright/test";

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
  add_on_ids: string[];
  coupon_ids: string[];
  created_at: string;
  updated_at: string;
};

type AddOn = {
  id: string;
  code: string;
  name: string;
  description?: string;
  currency: string;
  billing_interval: "monthly" | "yearly";
  status: "draft" | "active" | "archived";
  amount_cents: number;
  created_at: string;
  updated_at: string;
};

type Coupon = {
  id: string;
  code: string;
  name: string;
  description?: string;
  status: "draft" | "active" | "archived";
  discount_type: "amount_off" | "percent_off";
  currency?: string;
  amount_off_cents: number;
  percent_off: number;
  frequency: "once" | "recurring" | "forever";
  frequency_duration: number;
  expiration_at?: string;
  created_at: string;
  updated_at: string;
};

type Tax = {
  id: string;
  code: string;
  name: string;
  description?: string;
  status: "draft" | "active" | "archived";
  rate: number;
  created_at: string;
  updated_at: string;
};

async function installPricingMock(context: BrowserContext, session: TenantSessionPayload) {
  let metrics: PricingMetric[] = [];
  let addOns: AddOn[] = [];
  let coupons: Coupon[] = [];
  let taxes: Tax[] = [];
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
        add_on_ids: (body.add_on_ids as string[]) || [],
        coupon_ids: (body.coupon_ids as string[]) || [],
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

  await context.route("**/v1/add-ons", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(addOns),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      const now = new Date().toISOString();
      const addOn: AddOn = {
        id: "aon_support_1",
        code: String(body.code || "priority_support"),
        name: String(body.name || "Priority support"),
        description: String(body.description || ""),
        currency: String(body.currency || "USD"),
        billing_interval: (body.billing_interval as "monthly" | "yearly") || "monthly",
        status: (body.status as "draft" | "active" | "archived") || "draft",
        amount_cents: Number(body.amount_cents || 1500),
        created_at: now,
        updated_at: now,
      };
      addOns = [addOn];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(addOn),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/add-ons/aon_support_1", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(addOns[0]),
    });
  });

  await context.route("**/v1/coupons", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(coupons),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      const now = new Date().toISOString();
      const coupon: Coupon = {
        id: "cpn_launch_20",
        code: String(body.code || "launch_20"),
        name: String(body.name || "Launch 20"),
        description: String(body.description || ""),
        status: (body.status as "draft" | "active" | "archived") || "draft",
        discount_type: (body.discount_type as "amount_off" | "percent_off") || "percent_off",
        currency: typeof body.currency === "string" ? body.currency : undefined,
        amount_off_cents: Number(body.amount_off_cents || 0),
        percent_off: Number(body.percent_off || 0),
        frequency: (body.frequency as "once" | "recurring" | "forever") || "forever",
        frequency_duration: Number(body.frequency_duration || 0),
        expiration_at: typeof body.expiration_at === "string" ? body.expiration_at : undefined,
        created_at: now,
        updated_at: now,
      };
      coupons = [coupon];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(coupon),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/coupons/cpn_launch_20", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(coupons[0]),
    });
  });

  await context.route("**/v1/taxes", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(taxes),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      const now = new Date().toISOString();
      const tax: Tax = {
        id: "tax_gst_in_18",
        code: String(body.code || "gst_in_18"),
        name: String(body.name || "India GST 18"),
        description: String(body.description || ""),
        status: (body.status as "draft" | "active" | "archived") || "active",
        rate: Number(body.rate || 18),
        created_at: now,
        updated_at: now,
      };
      taxes = [tax];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(tax),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/taxes/tax_gst_in_18", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(taxes[0]),
    });
  });
}

async function fillUntilValue(locator: Locator, value: string) {
  await expect
    .poll(async () => {
      await locator.fill(value);
      return locator.inputValue();
    })
    .toBe(value);
}

async function waitForTenantSession(page: Page) {
  await expect(page.getByTestId("session-menu-toggle")).toBeVisible();
}

async function waitForCreatedResource(page: Page, pathFragment: string, action: () => Promise<void>) {
  await Promise.all([
    page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        response.url().includes(pathFragment) &&
        response.status() === 201,
    ),
    action(),
  ]);
}


test("tenant writer can create pricing metric tax add-on coupon and plan", async ({ page, context }) => {
  test.slow();

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
  await waitForTenantSession(page);
  await page.getByTestId("pricing-metric-name").fill("API Calls");
  await page.getByTestId("pricing-metric-code").fill("api_calls");
  await expect(page.getByTestId("pricing-metric-submit")).toBeEnabled();
  await waitForCreatedResource(page, "/v1/pricing/metrics", () => page.getByTestId("pricing-metric-submit").click());

  await expect(page).toHaveURL(/\/pricing\/metrics\/mtr_metric_1$/);
  await expect(page.getByRole("heading", { name: "API Calls" })).toBeVisible();

  await page.goto("/pricing/add-ons/new");
  await waitForTenantSession(page);
  await page.getByTestId("pricing-addon-name").fill("Priority support");
  await page.getByTestId("pricing-addon-code").fill("priority_support");
  await page.getByTestId("pricing-addon-amount").fill("15");
  await expect(page.getByTestId("pricing-addon-submit")).toBeEnabled();
  await waitForCreatedResource(page, "/v1/add-ons", () => page.getByTestId("pricing-addon-submit").click());

  await expect(page).toHaveURL(/\/pricing\/add-ons\/aon_support_1$/);
  await expect(page.getByRole("heading", { name: "Priority support" })).toBeVisible();
  await expect(page.getByText("15.00 USD")).toBeVisible();

  await page.goto("/pricing/coupons/new");
  await waitForTenantSession(page);
  await fillUntilValue(page.getByTestId("pricing-coupon-name"), "Launch 20");
  await fillUntilValue(page.getByTestId("pricing-coupon-code"), "launch_20");
  await expect(page.getByTestId("pricing-coupon-frequency")).toBeVisible();
  await expect(page.getByTestId("pricing-coupon-submit")).toBeEnabled();
  await waitForCreatedResource(page, "/v1/coupons", () => page.getByTestId("pricing-coupon-submit").click());

  await expect(page).toHaveURL(/\/pricing\/coupons\/cpn_launch_20$/);
  await expect(page.getByRole("heading", { name: "Launch 20" })).toBeVisible();
  await expect(page.getByText("20% off")).toBeVisible();
  await expect(page.getByText("Forever")).toBeVisible();

  await page.goto("/pricing/taxes/new");
  await waitForTenantSession(page);
  await page.getByTestId("pricing-tax-name").fill("India GST 18");
  await page.getByTestId("pricing-tax-code").fill("gst_in_18");
  await page.getByTestId("pricing-tax-rate").fill("18");
  await expect(page.getByTestId("pricing-tax-submit")).toBeEnabled();
  await waitForCreatedResource(page, "/v1/taxes", () => page.getByTestId("pricing-tax-submit").click());

  await expect(page).toHaveURL(/\/pricing\/taxes\/tax_gst_in_18$/);
  await expect(page.getByRole("heading", { name: "India GST 18" })).toBeVisible();
  await expect(page.getByText("18.00%", { exact: false })).toBeVisible();

  await page.goto("/pricing/plans/new");
  await waitForTenantSession(page);
  await fillUntilValue(page.getByTestId("pricing-plan-name"), "Growth");
  await fillUntilValue(page.getByTestId("pricing-plan-code"), "growth");
  await fillUntilValue(page.getByTestId("pricing-plan-base-price"), "49");
  await page.getByTestId("pricing-plan-metric-mtr_metric_1").check();
  await expect(page.getByTestId("pricing-plan-metric-mtr_metric_1")).toBeChecked();
  await page.getByTestId("pricing-plan-addon-aon_support_1").check();
  await expect(page.getByTestId("pricing-plan-addon-aon_support_1")).toBeChecked();
  await page.getByTestId("pricing-plan-coupon-cpn_launch_20").check();
  await expect(page.getByTestId("pricing-plan-coupon-cpn_launch_20")).toBeChecked();
  await expect(page.getByTestId("pricing-plan-submit")).toBeEnabled();
  await waitForCreatedResource(page, "/v1/plans", () => page.getByTestId("pricing-plan-submit").click());

  await expect(page).toHaveURL(/\/pricing\/plans\/pln_growth_1$/);
  await expect(page.getByText("Loading plan detail")).not.toBeVisible({ timeout: 15000 });
  await expect(page.getByRole("heading", { name: "Growth" })).toBeVisible();
  await expect(page.getByText("49.00 USD")).toBeVisible();
  await expect(page.getByText("Priority support")).toBeVisible();
  await expect(page.getByText("Launch 20")).toBeVisible();
});
