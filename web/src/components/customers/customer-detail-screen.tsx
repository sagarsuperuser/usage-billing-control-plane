
import { Link } from "@tanstack/react-router";
import { CreditCard, ExternalLink, LoaderCircle, RefreshCw, RotateCcw, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { type InputHTMLAttributes, useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import {
  beginCustomerPaymentSetup,
  fetchCustomerBillingProfile,
  fetchCustomerReadiness,
  fetchCustomers,
  refreshCustomerPaymentSetup,
  requestCustomerPaymentSetup,
  resendCustomerPaymentSetup,
  retryCustomerBillingSync,
  updateCustomerBillingProfile,
} from "@/lib/api";
import { customerCollectionDiagnosisToneClass, diagnoseCustomerCollection } from "@/lib/customer-collection-diagnosis";
import { showError, showSuccess } from "@/lib/toast";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { type CustomerBillingProfile, type CustomerBillingProfileInput } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function tone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending":
    case "incomplete":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "sync_error":
    case "error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

const billingProfileSchema = z.object({
  legal_name: z.string().min(1),
  email: z.string().min(1),
  phone: z.string(),
  tax_identifier: z.string(),
  tax_codes_raw: z.string(),
  billing_address_line1: z.string().min(1),
  billing_address_line2: z.string(),
  billing_city: z.string().min(1),
  billing_state: z.string(),
  billing_postal_code: z.string().min(1),
  billing_country: z.string().min(1),
  currency: z.string().min(1),
  provider_code: z.string(),
});

type BillingProfileFormValues = z.infer<typeof billingProfileSchema>;

export function CustomerDetailScreen({ externalID }: { externalID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [profileFlash, setProfileFlash] = useState<string | null>(null);
  const { register, handleSubmit: handleProfileSubmit, reset: resetProfile, watch: watchProfile, formState: profileFormState } = useForm<BillingProfileFormValues>({
    resolver: zodResolver(billingProfileSchema),
    defaultValues: {
      legal_name: "", email: "", phone: "", tax_identifier: "", tax_codes_raw: "",
      billing_address_line1: "", billing_address_line2: "", billing_city: "",
      billing_state: "", billing_postal_code: "", billing_country: "", currency: "", provider_code: "",
    },
  });

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, externalID],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, externalID, limit: 1 }),
    enabled: isTenantSession && externalID.trim().length > 0,
  });

  const readinessQuery = useQuery({
    queryKey: ["customer-readiness", apiBaseURL, externalID],
    queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID }),
    enabled: isTenantSession && externalID.trim().length > 0,
  });
  const billingProfileQuery = useQuery({
    queryKey: ["customer-billing-profile", apiBaseURL, externalID],
    queryFn: () => fetchCustomerBillingProfile({ runtimeBaseURL: apiBaseURL, externalID }),
    enabled: isTenantSession && externalID.trim().length > 0,
  });

  const retryMutation = useMutation({
    mutationFn: () => retryCustomerBillingSync({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      showSuccess("Billing sync retry initiated");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
    onError: (err: Error) => {
      showError("Billing sync retry failed", err.message || "Could not retry billing sync.");
    },
  });

  const refreshMutation = useMutation({
    mutationFn: () => refreshCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      showSuccess("Payment setup refreshed");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
    onError: (err: Error) => {
      showError("Refresh failed", err.message || "Could not refresh payment setup.");
    },
  });
  const beginSetupMutation = useMutation({
    mutationFn: () => beginCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
    onError: (err: Error) => showError(err.message),
  });
  const requestSetupMutation = useMutation({
    mutationFn: () => requestCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      showSuccess("Payment setup request sent", "The customer will receive an email with the checkout link.");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
    onError: (err: Error) => {
      showError("Request failed", err.message || "Could not send payment setup request.");
    },
  });
  const resendSetupMutation = useMutation({
    mutationFn: () => resendCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      showSuccess("Payment setup request resent", "A new checkout link has been sent to the customer.");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
    onError: (err: Error) => {
      showError("Resend failed", err.message || "Could not resend payment setup request.");
    },
  });
  const billingProfileMutation = useMutation({
    mutationFn: (data: BillingProfileFormValues) =>
      updateCustomerBillingProfile({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        externalID,
        body: { ...data, tax_codes: parseCodeList(data.tax_codes_raw) },
      }),
    onSuccess: async () => {
      setProfileFlash("Billing profile saved.");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch(), billingProfileQuery.refetch()]);
    },
    onError: (err: Error) => showError(err.message),
  });

  const customer = customersQuery.data?.[0] ?? null;
  const readiness = readinessQuery.data ?? null;
  const billingProfile = billingProfileQuery.data ?? readiness?.billing_profile ?? null;
  const readinessMissingSteps = normalizeMissingSteps(readiness?.missing_steps);
  const nextActions = readinessMissingSteps.map(describeCustomerMissingStep);
  const collectionDiagnosis = readiness ? diagnoseCustomerCollection(readiness) : null;
  const canBeginPaymentSetup = Boolean(
    canWrite &&
      csrfToken &&
      readiness?.customer_active &&
      readiness?.billing_profile_status === "ready" &&
      readiness?.payment_setup_status !== "ready",
  );
  const showResendRequest = readiness?.payment_setup.last_request_status === "sent" || readiness?.payment_setup.last_request_status === "failed";
  const setupRequestActionLabel = showResendRequest ? "Resend payment setup request" : "Send payment setup request";
  const latestCheckoutURL = beginSetupMutation.data?.checkout_url;
  const latestRequestedCheckoutURL = requestSetupMutation.data?.checkout_url || resendSetupMutation.data?.checkout_url;
  const profileBaseline = billingProfileDraftFromProfile(billingProfile, customer?.email);
  const profileSourceKey = [externalID, billingProfile?.updated_at || "", billingProfile?.last_synced_at || "", customer?.email || ""].join(":");

  useEffect(() => {
    resetProfile(
      {
        legal_name: profileBaseline.legal_name || "",
        email: profileBaseline.email || "",
        phone: profileBaseline.phone || "",
        tax_identifier: profileBaseline.tax_identifier || "",
        tax_codes_raw: (profileBaseline.tax_codes || []).join(", "),
        billing_address_line1: profileBaseline.billing_address_line1 || "",
        billing_address_line2: profileBaseline.billing_address_line2 || "",
        billing_city: profileBaseline.billing_city || "",
        billing_state: profileBaseline.billing_state || "",
        billing_postal_code: profileBaseline.billing_postal_code || "",
        billing_country: profileBaseline.billing_country || "",
        currency: profileBaseline.currency || "",
        provider_code: profileBaseline.provider_code || "",
      },
      { keepDirtyValues: true },
    );
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profileSourceKey]);

  const _profileValues = watchProfile();
  const billingProfileDirty = profileFormState.isDirty;
  const billingProfileReady = profileFormState.isValid;

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/customers", label: "Customers" }, { label: customer?.display_name || externalID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? (
          customersQuery.isLoading || readinessQuery.isLoading ? (
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading customer detail
              </div>
            </section>
          ) : customersQuery.isError || readinessQuery.isError || billingProfileQuery.isError || !customer || !readiness ? (
            <section className="rounded-lg border border-stone-200 bg-white shadow-sm p-5">
              <p className="text-sm font-semibold text-slate-900">Customer not available</p>
              <p className="mt-1 text-sm text-slate-500">The requested customer could not be loaded from the workspace APIs.</p>
            </section>
          ) : (
          <SectionErrorBoundary>
            <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
              {/* ---- Header ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900 truncate">{customer.display_name}</h1>
                    <span className="font-mono text-xs text-slate-400">{customer.external_id}</span>
                    <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${tone(readiness.status)}`}>
                      {formatReadinessStatus(readiness.status)}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Link
                      to={`/invoices?customer_external_id=${encodeURIComponent(customer.external_id)}`}
                      className="inline-flex h-8 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                    >
                      View invoices
                    </Link>
                    <Link to="/customers/new" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800">
                      <CreditCard className="h-3.5 w-3.5" />
                      New customer
                    </Link>
                  </div>
                </div>
              </div>

              {/* ---- Details ---- */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Billing profile</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatReadinessStatus(readiness.billing_profile_status)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Payment setup</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatReadinessStatus(readiness.payment_setup_status)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Payment method</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{readiness.default_payment_method_verified ? "Verified" : "Awaiting setup"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Email</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{customer.email || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Created</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(customer.created_at)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last synced</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(readiness.billing_profile.last_synced_at)}</dd>
                  </div>
                </dl>
              </div>

              {/* ---- Readiness ---- */}
              {nextActions.length > 0 ? (
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-slate-400 mb-3">Open actions</p>
                  <div className="grid gap-2">
                    {nextActions.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-sm text-slate-700">
                        <span className="mt-0.5 inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-amber-100 text-[10px] font-semibold text-amber-700">!</span>
                        {item}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {/* ---- Billing profile form ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-3">Billing profile</p>
                <div className="grid gap-3 md:grid-cols-2">
                  <InputField label="Legal name" placeholder="Acme Billing LLC" {...register("legal_name")} />
                  <InputField label="Billing email" placeholder="billing@acme.test" {...register("email")} />
                  <InputField label="Phone" placeholder="+1 415 555 0100" {...register("phone")} />
                  <InputField label="Tax identifier" placeholder="VAT / GST / EIN" {...register("tax_identifier")} />
                  <InputField label="Tax codes" placeholder="GST_IN, VAT_DE" {...register("tax_codes_raw")} />
                  <InputField label="Address line 1" placeholder="1 Billing Street" {...register("billing_address_line1")} />
                  <InputField label="Address line 2" placeholder="Suite 200" {...register("billing_address_line2")} />
                  <InputField label="City" placeholder="Bengaluru" {...register("billing_city")} />
                  <InputField label="State" placeholder="Karnataka" {...register("billing_state")} />
                  <InputField label="Postal code" placeholder="560001" {...register("billing_postal_code")} />
                  <InputField label="Country" placeholder="IN" {...register("billing_country")} />
                  <InputField label="Currency" placeholder="USD" {...register("currency")} />
                  <InputField label="Billing connection code" placeholder="stripe_default" {...register("provider_code")} />
                </div>

                <div className="mt-4 flex flex-wrap gap-3">
                  <button
                    type="button"
                    onClick={handleProfileSubmit((data) => {
                      setProfileFlash(null);
                      billingProfileMutation.mutate(data);
                    })}
                    disabled={!canWrite || !csrfToken || billingProfileMutation.isPending || !billingProfileDirty}
                    className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {billingProfileMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <CreditCard className="h-3.5 w-3.5" />}
                    Save billing profile
                  </button>
                  <button
                    type="button"
                    onClick={() => resetProfile({
                      legal_name: profileBaseline.legal_name || "",
                      email: profileBaseline.email || "",
                      phone: profileBaseline.phone || "",
                      tax_identifier: profileBaseline.tax_identifier || "",
                      tax_codes_raw: (profileBaseline.tax_codes || []).join(", "),
                      billing_address_line1: profileBaseline.billing_address_line1 || "",
                      billing_address_line2: profileBaseline.billing_address_line2 || "",
                      billing_city: profileBaseline.billing_city || "",
                      billing_state: profileBaseline.billing_state || "",
                      billing_postal_code: profileBaseline.billing_postal_code || "",
                      billing_country: profileBaseline.billing_country || "",
                      currency: profileBaseline.currency || "",
                      provider_code: profileBaseline.provider_code || "",
                    })}
                    disabled={!billingProfileDirty || billingProfileMutation.isPending}
                    className="inline-flex h-8 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Reset
                  </button>
                </div>

                {profileFlash ? (
                  <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                    <p className="font-medium">{profileFlash}</p>
                    <p className="mt-0.5 text-xs opacity-80">{billingProfileReady ? "Required billing fields are complete." : "Profile still needs required readiness fields."}</p>
                  </div>
                ) : null}
                {billingProfileMutation.isError ? (
                  <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <p className="font-medium">Billing profile could not be saved</p>
                    <p className="mt-0.5 text-xs">{billingProfileMutation.error instanceof Error ? billingProfileMutation.error.message : "Saving the billing profile failed."}</p>
                  </div>
                ) : null}
                {!billingProfileReady ? (
                  <p className="mt-3 text-xs text-slate-500">Payment setup is blocked until the required billing fields are complete.</p>
                ) : null}
              </div>

              {/* ---- Payment collection ---- */}
              <div id="payment-collection" className="scroll-mt-24 px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-1">Payment collection</p>
                {collectionDiagnosis ? (
                  <div className={`mt-2 rounded-md border px-4 py-3 text-sm ${customerCollectionDiagnosisToneClass(collectionDiagnosis.tone || "warning")}`}>
                    <p className="font-medium">{collectionDiagnosis.title}</p>
                    <p className="mt-0.5 text-xs opacity-80">{collectionDiagnosis.summary}</p>
                    <p className="mt-1 text-xs opacity-80">Next: {collectionDiagnosis.nextStep}</p>
                  </div>
                ) : null}

                <div className="mt-4 grid gap-4 sm:grid-cols-2">
                  <div>
                    <p className="text-xs font-medium text-slate-500 mb-2">Email setup request</p>
                    <p className="text-xs text-slate-500 mb-3">Use the email request first. Resend instead of creating duplicates.</p>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => (showResendRequest ? resendSetupMutation.mutate() : requestSetupMutation.mutate())}
                        disabled={!canBeginPaymentSetup || requestSetupMutation.isPending || resendSetupMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {requestSetupMutation.isPending || resendSetupMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Send className="h-3.5 w-3.5" />}
                        {setupRequestActionLabel}
                      </button>
                      {latestRequestedCheckoutURL ? (
                        <a href={latestRequestedCheckoutURL} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                          <ExternalLink className="h-3.5 w-3.5" />
                          Open sent link
                        </a>
                      ) : null}
                    </div>
                  </div>

                  <div>
                    <p className="text-xs font-medium text-slate-500 mb-2">Hosted setup link</p>
                    <p className="text-xs text-slate-500 mb-3">Use only when you need to share the link directly.</p>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => beginSetupMutation.mutate()}
                        disabled={!canBeginPaymentSetup || beginSetupMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {beginSetupMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <CreditCard className="h-3.5 w-3.5" />}
                        Generate link
                      </button>
                      {latestCheckoutURL ? (
                        <a href={latestCheckoutURL} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                          <ExternalLink className="h-3.5 w-3.5" />
                          Open link
                        </a>
                      ) : null}
                    </div>
                  </div>
                </div>

                <div className="mt-4 flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={() => refreshMutation.mutate()}
                    disabled={!canWrite || !csrfToken || refreshMutation.isPending}
                    className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {refreshMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                    Refresh payment setup
                  </button>
                  <button
                    type="button"
                    onClick={() => retryMutation.mutate()}
                    disabled={!canWrite || !csrfToken || retryMutation.isPending}
                    className="inline-flex h-8 items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50 px-3 text-xs font-medium text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {retryMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw className="h-3.5 w-3.5" />}
                    Retry billing sync
                  </button>
                  <Link
                    to={`/subscriptions?customer_external_id=${encodeURIComponent(customer.external_id)}`}
                    className="inline-flex h-8 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                  >
                    Open subscriptions
                  </Link>
                </div>

                {beginSetupMutation.isError ? (
                  <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <p className="font-medium">Payment setup link could not be generated</p>
                    <p className="mt-0.5 text-xs">{beginSetupMutation.error instanceof Error ? beginSetupMutation.error.message : "Customer payment setup request failed."}</p>
                  </div>
                ) : null}
                {requestSetupMutation.isError || resendSetupMutation.isError ? (
                  <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <p className="font-medium">Payment setup request could not be sent</p>
                    <p className="mt-0.5 text-xs">
                      {(requestSetupMutation.error instanceof Error && requestSetupMutation.error.message) ||
                        (resendSetupMutation.error instanceof Error && resendSetupMutation.error.message) ||
                        "Customer payment setup email delivery failed."}
                    </p>
                  </div>
                ) : null}
                {readiness.payment_setup.last_request_status === "sent" ? (
                  <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                    <p className="font-medium">Payment setup request sent</p>
                    <p className="mt-0.5 text-xs">
                      Sent to {readiness.payment_setup.last_request_to_email || "the customer"} on {formatExactTimestamp(readiness.payment_setup.last_request_sent_at)}.
                    </p>
                  </div>
                ) : null}
                {readiness.payment_setup.last_request_status === "failed" ? (
                  <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <p className="font-medium">Latest payment setup request failed</p>
                    <p className="mt-0.5 text-xs">{readiness.payment_setup.last_request_error || "Email delivery failed. You can resend or fall back to the hosted link."}</p>
                  </div>
                ) : null}
                {latestCheckoutURL ? (
                  <div className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                    <p className="font-medium">Hosted payment setup link ready</p>
                    <p className="mt-0.5 text-xs">Share this manually, then refresh status once setup is complete.</p>
                  </div>
                ) : null}
                {!canBeginPaymentSetup && readiness.payment_setup_status !== "ready" ? (
                  <p className="mt-3 text-xs text-slate-500">Payment setup can be requested only after the customer is active and the billing profile is ready.</p>
                ) : null}
              </div>

              {/* ---- Billing state ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-3">Billing state</p>
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Billing customer ID</dt>
                    <dd className="mt-0.5 text-sm font-mono text-slate-700">{customer.lago_customer_id || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last sync error</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{readiness.billing_profile.last_sync_error || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last verified</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(readiness.payment_setup.last_verified_at)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Customer status</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatReadinessStatus(customer.status)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Updated</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(customer.updated_at)}</dd>
                  </div>
                </dl>
              </div>
            </div>
          </SectionErrorBoundary>
          )
        ) : null}
      </main>
    </div>
  );
}

function InputField({ label, placeholder, ...props }: { label: string; placeholder: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-1 text-sm text-slate-700">
      <span className="text-xs text-slate-400">{label}</span>
      <input
        {...props}
        placeholder={placeholder}
        className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}

function emptyBillingProfileDraft(): CustomerBillingProfileInput {
  return {
    legal_name: "",
    email: "",
    phone: "",
    billing_address_line1: "",
    billing_address_line2: "",
    billing_city: "",
    billing_state: "",
    billing_postal_code: "",
    billing_country: "",
    currency: "",
    tax_identifier: "",
    tax_codes: [],
    provider_code: "",
  };
}

function billingProfileDraftFromProfile(profile: CustomerBillingProfile | null, fallbackEmail?: string): CustomerBillingProfileInput {
  if (!profile) {
    return {
      ...emptyBillingProfileDraft(),
      email: fallbackEmail || "",
    };
  }
  return {
    legal_name: profile.legal_name || "",
    email: profile.email || fallbackEmail || "",
    phone: profile.phone || "",
    billing_address_line1: profile.billing_address_line1 || "",
    billing_address_line2: profile.billing_address_line2 || "",
    billing_city: profile.billing_city || "",
    billing_state: profile.billing_state || "",
    billing_postal_code: profile.billing_postal_code || "",
    billing_country: profile.billing_country || "",
    currency: profile.currency || "",
    tax_identifier: profile.tax_identifier || "",
    tax_codes: profile.tax_codes || [],
    provider_code: profile.provider_code || "",
  };
}


function parseCodeList(value: string): string[] {
  const seen = new Set<string>();
  return value
    .split(",")
    .map((item) => item.trim().toUpperCase())
    .filter((item) => item.length > 0)
    .filter((item) => {
      if (seen.has(item)) return false;
      seen.add(item);
      return true;
    });
}
