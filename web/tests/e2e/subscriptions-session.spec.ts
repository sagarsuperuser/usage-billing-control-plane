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

type Customer = {
  id: string;
  external_id: string;
  display_name: string;
  email?: string;
  status: "active";
  created_at: string;
  updated_at: string;
};

type Plan = {
  id: string;
  code: string;
  name: string;
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

type Subscription = {
  id: string;
  code: string;
  display_name: string;
  status: "draft" | "pending_payment_setup" | "active" | "action_required" | "archived";
  customer_id: string;
  customer_external_id: string;
  customer_display_name: string;
  plan_id: string;
  plan_code: string;
  plan_name: string;
  billing_interval: "monthly" | "yearly";
  currency: string;
  base_amount_cents: number;
  payment_setup_status: "missing" | "pending" | "ready" | "error";
  default_payment_method_verified: boolean;
  payment_setup_action_required: boolean;
  created_at: string;
  updated_at: string;
  payment_setup_requested_at?: string;
};

async function buildDetail(subscription: Subscription, customer: Customer, plan: Plan, now: string) {
  return {
    ...subscription,
    customer,
    plan,
    billing_profile: { customer_id: customer.id, profile_status: "ready", created_at: now, updated_at: now },
    payment_setup: {
      customer_id: customer.id,
      setup_status: subscription.payment_setup_status,
      default_payment_method_present: false,
      payment_method_type: "card",
      last_verification_result: "checkout_url_generated",
      created_at: now,
      updated_at: now,
    },
    missing_steps: subscription.payment_setup_status === "ready" ? [] : ["default_payment_method_verified"],
  };
}

async function installSubscriptionMock(context: BrowserContext, session: TenantSessionPayload) {
  const now = new Date().toISOString();
  const customers: Customer[] = [
    { id: "cus_1", external_id: "acme", display_name: "Acme Corp", email: "billing@acme.test", status: "active", created_at: now, updated_at: now },
  ];
  const plans: Plan[] = [
    { id: "pln_growth", code: "growth", name: "Growth", currency: "USD", billing_interval: "monthly", status: "active", base_amount_cents: 4900, meter_ids: ["mtr_1"], add_on_ids: [], coupon_ids: [], created_at: now, updated_at: now },
  ];
  let subscriptions: Subscription[] = [];

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

  await context.route("**/v1/customers**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(customers),
    });
  });

  await context.route("**/v1/plans", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(plans),
    });
  });

  await context.route("**/v1/subscriptions", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();

    if (method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(subscriptions),
      });
      return;
    }

    if (method === "POST") {
      const body = request.postDataJSON() as Record<string, unknown>;
      const subscription: Subscription = {
        id: "sub_1",
        code: String(body.code || "acme_growth"),
        display_name: String(body.display_name || "Acme Corp - Growth"),
        status: body.request_payment_setup ? "pending_payment_setup" : "draft",
        customer_id: "cus_1",
        customer_external_id: "acme",
        customer_display_name: "Acme Corp",
        plan_id: "pln_growth",
        plan_code: "growth",
        plan_name: "Growth",
        billing_interval: "monthly",
        currency: "USD",
        base_amount_cents: 4900,
        payment_setup_status: body.request_payment_setup ? "pending" : "missing",
        default_payment_method_verified: false,
        payment_setup_action_required: false,
        payment_setup_requested_at: body.request_payment_setup ? now : undefined,
        created_at: now,
        updated_at: now,
      };
      subscriptions = [subscription];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          subscription: {
            ...subscription,
            customer: customers[0],
            plan: plans[0],
            billing_profile: { customer_id: "cus_1", profile_status: "ready", created_at: now, updated_at: now },
            payment_setup: {
              customer_id: "cus_1",
              setup_status: subscription.payment_setup_status,
              default_payment_method_present: false,
              payment_method_type: "card",
              created_at: now,
              updated_at: now,
            },
            missing_steps: ["default_payment_method_verified"],
          },
          payment_setup_started: true,
          checkout_url: "https://checkout.alpha.test/session/sub_1",
        }),
      });
      return;
    }

    await route.fallback();
  });

  await context.route("**/v1/subscriptions/sub_1", async (route) => {
    const subscription = subscriptions[0];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ...subscription,
        customer: customers[0],
        plan: plans[0],
        billing_profile: { customer_id: "cus_1", profile_status: "ready", created_at: now, updated_at: now },
        payment_setup: {
          customer_id: "cus_1",
          setup_status: subscription.payment_setup_status,
          default_payment_method_present: false,
          payment_method_type: "card",
          last_verification_result: subscription.payment_setup_status === "pending" ? "checkout_url_generated" : "",
          created_at: now,
          updated_at: now,
        },
        missing_steps: subscription.payment_setup_status === "ready" ? [] : ["default_payment_method_verified"],
      }),
    });
  });

  await context.route("**/v1/subscriptions/sub_1/payment-setup/request", async (route) => {
    subscriptions = subscriptions.map((item) => ({ ...item, status: "pending_payment_setup", payment_setup_status: "pending", payment_setup_requested_at: now }));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ action: "requested", checkout_url: "https://checkout.alpha.test/session/sub_1", subscription: await buildDetail(subscriptions[0], customers[0], plans[0], now) }),
    });
  });

  await context.route("**/v1/subscriptions/sub_1/payment-setup/resend", async (route) => {
    subscriptions = subscriptions.map((item) => ({ ...item, status: "pending_payment_setup", payment_setup_status: "pending", payment_setup_requested_at: now }));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ action: "resent", checkout_url: "https://checkout.alpha.test/session/sub_1?resent=1", subscription: await buildDetail(subscriptions[0], customers[0], plans[0], now) }),
    });
  });
}

test("tenant writer can create subscription and resend payment setup", async ({ page, context }) => {
  await installSubscriptionMock(context, {
    authenticated: true,
    subject_type: "user",
    subject_id: "usr_tenant_1",
    user_email: "tenant-writer@alpha.test",
    scope: "tenant",
    role: "writer",
    tenant_id: "tenant_a",
    csrf_token: "csrf-tenant-123",
  });

  await page.goto("/subscriptions/new");
  const nameInput = page.getByTestId("subscription-name");
  const codeInput = page.getByTestId("subscription-code");
  const customerSelect = page.getByTestId("subscription-customer");
  const planSelect = page.getByTestId("subscription-plan");

  await expect(page.getByRole("heading", { name: "Create subscription" })).toBeVisible();
  await expect(customerSelect.locator("option")).toHaveCount(2);
  await expect(planSelect.locator("option")).toHaveCount(2);

  for (let attempt = 0; attempt < 5; attempt += 1) {
    await nameInput.fill("Acme Growth");
    await codeInput.fill("acme_growth");
    await customerSelect.selectOption("acme");
    await planSelect.selectOption("pln_growth");

    if (
      (await nameInput.inputValue()) === "Acme Growth" &&
      (await codeInput.inputValue()) === "acme_growth" &&
      (await customerSelect.inputValue()) === "acme" &&
      (await planSelect.inputValue()) === "pln_growth"
    ) {
      break;
    }

    await page.waitForTimeout(150);
  }

  await expect(nameInput).toHaveValue("Acme Growth");
  await expect(codeInput).toHaveValue("acme_growth");
  await expect(customerSelect).toHaveValue("acme");
  await expect(planSelect).toHaveValue("pln_growth");
  await expect(page.getByTestId("subscription-submit")).toBeEnabled();
  await page.getByTestId("subscription-submit").click();

  await expect(page.getByText("Subscription created")).toBeVisible();
  await page.getByRole("link", { name: "Open subscription" }).click();

  await expect(page).toHaveURL(/\/subscriptions\/sub_1$/);
  await expect(page.getByRole("heading", { name: "Acme Growth" })).toBeVisible();
  await expect(page.getByText("Pending payment setup", { exact: true }).first()).toBeVisible();

  await page.getByRole("button", { name: "Resend payment setup" }).click();
  await expect(page.getByRole("link", { name: "Open latest setup link" })).toBeVisible();
});
