import { expect, test } from "@playwright/test";

import { loginWithPassword } from "./live-browser-auth";

const liveWriterEmail = process.env.PLAYWRIGHT_LIVE_WRITER_EMAIL || "";
const liveWriterPassword = process.env.PLAYWRIGHT_LIVE_WRITER_PASSWORD || "";
const onboardingExternalID = process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EXTERNAL_ID || "";
const onboardingDisplayName = process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_DISPLAY_NAME || "";
const onboardingEmail = process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EMAIL || "";
const onboardingLegalName = process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_LEGAL_NAME || "";
const onboardingProviderCode = process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_PROVIDER_CODE || "";

const billingAddress = {
  line1: process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_ADDRESS_LINE1 || "1 Billing Street",
  city: process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_CITY || "Bengaluru",
  postalCode: process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_POSTAL_CODE || "560001",
  country: process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_COUNTRY || "IN",
  currency: process.env.PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_CURRENCY || "USD",
};

test.describe("customer onboarding live staging", () => {
  test.skip(!process.env.PLAYWRIGHT_LIVE_BASE_URL, "PLAYWRIGHT_LIVE_BASE_URL is required for live customer onboarding journey");
  test.skip(!liveWriterEmail || !liveWriterPassword, "PLAYWRIGHT_LIVE_WRITER_EMAIL and PLAYWRIGHT_LIVE_WRITER_PASSWORD are required");

  test("writer can create a customer, sync billing profile, and inspect readiness", async ({ page }) => {
    test.setTimeout(180000);
    test.skip(!onboardingExternalID, "PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EXTERNAL_ID is required");
    test.skip(!onboardingDisplayName, "PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_DISPLAY_NAME is required");
    test.skip(!onboardingEmail, "PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_EMAIL is required");
    test.skip(!onboardingLegalName, "PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_LEGAL_NAME is required");
    test.skip(!onboardingProviderCode, "PLAYWRIGHT_LIVE_CUSTOMER_ONBOARDING_PROVIDER_CODE is required");

    await loginWithPassword(page, {
      email: liveWriterEmail,
      password: liveWriterPassword,
      nextPath: "/customers/new",
    });

    await expect(page).toHaveURL(/\/customers\/new(?:\?.*)?$/);
    await expect(page.getByRole("heading", { name: "Create customer" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Customer onboarding" })).toBeVisible();

    await page.getByLabel("Customer external ID").fill(onboardingExternalID);
    await page.getByLabel("Display name").fill(onboardingDisplayName);
    await page.getByLabel("Billing email").fill(onboardingEmail);
    await page.getByLabel("Legal name").fill(onboardingLegalName);
    await page.getByLabel("Billing address line 1").fill(billingAddress.line1);
    await page.getByLabel("Billing city").fill(billingAddress.city);
    await page.getByLabel("Billing postal code").fill(billingAddress.postalCode);
    await page.getByLabel("Billing country").fill(billingAddress.country);
    await page.getByLabel("Currency").fill(billingAddress.currency);
    await page.getByLabel("Billing connection code").fill(onboardingProviderCode);

    const paymentSetupCheckbox = page.getByLabel("Start payment setup now");
    await expect(paymentSetupCheckbox).toBeChecked();

    await page.getByRole("button", { name: "Run customer setup" }).click();

    await expect(page.getByText(`Customer ${onboardingExternalID} created and payment setup is ready to continue.`)).toBeVisible();
    await expect(page.getByText("Payment setup link")).toBeVisible();
    await expect(page.getByText("Customer created")).toBeVisible();
    await expect(page.getByRole("heading", { name: onboardingDisplayName })).toBeVisible();
    await expect(page.getByRole("link", { name: "View customer detail" })).toBeVisible();

    await page.getByRole("link", { name: "View customer detail" }).click();

    await expect(page).toHaveURL(new RegExp(`/customers/${onboardingExternalID}(?:\\?.*)?$`));
    await expect(page.getByRole("heading", { name: onboardingDisplayName })).toBeVisible();
    await expect(page.getByText(onboardingExternalID)).toBeVisible();
    await expect(page.getByText("Billing profile").first()).toBeVisible();
    await expect(page.getByText("Synced to billing")).toBeVisible();
    await expect(page.getByText("Awaiting setup")).toBeVisible();
    await expect(page.getByText("What still needs action")).toBeVisible();
    await expect(page.getByText("Customer has not completed payment setup").first()).toBeVisible();

    await page.goto("/customers");
    await expect(page.getByRole("heading", { name: "Customer directory" })).toBeVisible();
    await expect(page.getByText(onboardingDisplayName)).toBeVisible();
    await expect(page.getByText(onboardingExternalID)).toBeVisible();
  });
});
