import { expect, test, type Page } from "@playwright/test";

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

async function installSubscriptionMock(page: Page, session: TenantSessionPayload) {
  let loggedIn = true;
  const now = new Date().toISOString();
  const customers: Customer[] = [
    { id: "cus_1", external_id: "acme", display_name: "Acme Corp", email: "billing@acme.test", status: "active", created_at: now, updated_at: now },
  ];
  const plans: Plan[] = [
    { id: "pln_growth", code: "growth", name: "Growth", currency: "USD", billing_interval: "monthly", status: "active", base_amount_cents: 4900, meter_ids: ["mtr_1"], created_at: now, updated_at: now },
  ];
  let subscriptions: Subscription[] = [];

  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method().toUpperCase();

    const json = async (status: number, payload: unknown) => {
      await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(payload) });
    };

    if (path === "/runtime-config" && method === "GET") return json(200, { apiBaseURL: "" });
    if (path === "/v1/ui/sessions/me" && method === "GET") return json(loggedIn ? 200 : 401, loggedIn ? session : { error: "unauthorized" });
    if (path === "/v1/ui/sessions/login" && method === "POST") {
      loggedIn = true;
      return json(201, session);
    }
    if (path === "/v1/customers" && method === "GET") return json(200, customers);
    if (path === "/v1/plans" && method === "GET") return json(200, plans);
    if (path === "/v1/subscriptions" && method === "GET") return json(200, subscriptions);
    if (path === "/v1/subscriptions" && method === "POST") {
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
      return json(201, {
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
      });
    }
    if (path === "/v1/subscriptions/sub_1" && method === "GET") {
      const subscription = subscriptions[0];
      return json(200, {
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
      });
    }
    if (path === "/v1/subscriptions/sub_1/payment-setup/request" && method === "POST") {
      subscriptions = subscriptions.map((item) => ({ ...item, status: "pending_payment_setup", payment_setup_status: "pending", payment_setup_requested_at: now }));
      return json(200, { action: "requested", checkout_url: "https://checkout.alpha.test/session/sub_1", subscription: await buildDetail(subscriptions[0], customers[0], plans[0], now) });
    }
    if (path === "/v1/subscriptions/sub_1/payment-setup/resend" && method === "POST") {
      subscriptions = subscriptions.map((item) => ({ ...item, status: "pending_payment_setup", payment_setup_status: "pending", payment_setup_requested_at: now }));
      return json(200, { action: "resent", checkout_url: "https://checkout.alpha.test/session/sub_1?resent=1", subscription: await buildDetail(subscriptions[0], customers[0], plans[0], now) });
    }

    return route.continue();
  });
}

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

test("tenant writer can create subscription and resend payment setup", async ({ page }) => {
  await installSubscriptionMock(page, {
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
  await page.getByTestId("subscription-name").fill("Acme Growth");
  await page.getByTestId("subscription-code").fill("acme_growth");
  await page.getByTestId("subscription-customer").selectOption("acme");
  await page.getByTestId("subscription-plan").selectOption("pln_growth");
  await page.getByTestId("subscription-submit").click();

  await expect(page.getByText("Subscription created")).toBeVisible();
  await page.getByRole("link", { name: "Open subscription" }).click();

  await expect(page).toHaveURL(/\/subscriptions\/sub_1$/);
  await expect(page.getByRole("heading", { name: "Acme Corp - Growth" })).toBeVisible();
  await expect(page.locator("span").filter({ hasText: "Pending payment setup" })).toBeVisible();

  await page.getByTestId("subscription-resend-setup").click();
  await expect(page.getByRole("link", { name: "Open resent setup link" })).toBeVisible();
});
