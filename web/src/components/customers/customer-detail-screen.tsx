"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, ExternalLink, LoaderCircle, RefreshCw, RotateCcw, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
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

export function CustomerDetailScreen({ externalID }: { externalID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [profileDraftState, setProfileDraftState] = useState<{
    sourceKey: string;
    value: CustomerBillingProfileInput;
    flash: string | null;
  }>({
    sourceKey: "",
    value: emptyBillingProfileDraft(),
    flash: null,
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
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
  });

  const refreshMutation = useMutation({
    mutationFn: () => refreshCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
  });
  const beginSetupMutation = useMutation({
    mutationFn: () => beginCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
  });
  const requestSetupMutation = useMutation({
    mutationFn: () => requestCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
  });
  const resendSetupMutation = useMutation({
    mutationFn: () => resendCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID }),
    onSuccess: async () => {
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch()]);
    },
  });
  const billingProfileMutation = useMutation({
    mutationFn: () =>
      updateCustomerBillingProfile({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        externalID,
        body: profileDraft,
      }),
    onSuccess: async () => {
      updateProfileFlash("Billing profile saved.");
      await Promise.all([customersQuery.refetch(), readinessQuery.refetch(), billingProfileQuery.refetch()]);
    },
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
  const profileDraft =
    profileDraftState.sourceKey === profileSourceKey ? profileDraftState.value : profileBaseline;
  const profileFlash =
    profileDraftState.sourceKey === profileSourceKey ? profileDraftState.flash : null;
  const billingProfileDirty = serializeBillingProfileDraft(profileDraft) !== serializeBillingProfileDraft(profileBaseline);
  const billingProfileReady = requiredBillingProfileFields(profileDraft).every(Boolean);

  const updateProfileDraft = (
    next:
      | CustomerBillingProfileInput
      | ((current: CustomerBillingProfileInput) => CustomerBillingProfileInput),
  ) => {
    setProfileDraftState((current) => {
      const currentValue = current.sourceKey === profileSourceKey ? current.value : profileBaseline;
      return {
        sourceKey: profileSourceKey,
        value: typeof next === "function" ? next(currentValue) : next,
        flash: null,
      };
    });
  };

  const updateProfileFlash = (message: string | null) => {
    setProfileDraftState((current) => ({
      sourceKey: profileSourceKey,
      value: current.sourceKey === profileSourceKey ? current.value : profileBaseline,
      flash: message,
    }));
  };

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { href: "/customers", label: "Customers" }, { label: customer?.display_name || externalID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Customer detail is workspace-scoped. Sign in with a workspace reader, writer, or admin account to inspect readiness and run recovery actions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          customersQuery.isLoading || readinessQuery.isLoading ? (
            <LoadingPanel label="Loading customer detail" />
          ) : customersQuery.isError || readinessQuery.isError || billingProfileQuery.isError || !customer || !readiness ? (
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
              <h1 className="mt-2 text-2xl font-semibold text-slate-950">Customer not available</h1>
              <p className="mt-3 text-sm text-slate-600">The requested customer could not be loaded from the workspace APIs.</p>
              <Link href="/customers" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                <ArrowLeft className="h-4 w-4" />
                Back to customers
              </Link>
            </section>
          ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{customer.display_name}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{customer.external_id}</span>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(readiness.status)}`}>
                      {formatReadinessStatus(readiness.status)}
                    </span>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Link href="/customers" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                    <ArrowLeft className="h-4 w-4" />
                    Back to customers
                  </Link>
                  <Link
                    href={`/invoices?customer_external_id=${encodeURIComponent(customer.external_id)}`}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm text-slate-700 transition hover:bg-slate-50"
                  >
                    View invoices
                  </Link>
                  <Link href="/customers/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                    <CreditCard className="h-4 w-4" />
                    New customer
                  </Link>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Customer" value={readiness.customer_active ? "ready" : "pending"} helper={readiness.customer_active ? "Active" : "Needs attention"} />
              <SummaryStat label="Billing profile" value={readiness.billing_profile_status} helper={readiness.lago_customer_synced ? "Billing ready" : "Needs attention"} />
              <SummaryStat label="Payment setup" value={readiness.payment_setup_status} helper={readiness.default_payment_method_verified ? "Verified" : "Awaiting setup"} />
              <SummaryStat label="Open actions" value={String(readinessMissingSteps.length)} helper="Remaining checklist items" />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
              <div className="min-w-0 grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Readiness</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">What still needs action</h2>
                    </div>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(readiness.status)}`}>
                      {formatReadinessStatus(readiness.status)}
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Customer is ready for payment operations." />}
                  </div>
                  <div className="mt-5 grid gap-3 lg:grid-cols-3">
                    <StatusCard title="Billing profile" value={readiness.billing_profile_status} />
                    <StatusCard title="Payment setup" value={readiness.payment_setup_status} />
                    <StatusCard title="Overall readiness" value={readiness.status} />
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing profile</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Commercial and tax settings</h2>
                      <p className="mt-2 max-w-3xl text-sm text-slate-600">
                        Keep legal identity, billing address, tax details, currency, and billing connection current here.
                      </p>
                    </div>
                    <span className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(readiness.billing_profile_status)}`}>
                      {formatReadinessStatus(readiness.billing_profile_status)}
                    </span>
                  </div>

                  <div className="mt-5 grid gap-4 md:grid-cols-2">
                    <InputField label="Legal name" value={profileDraft.legal_name || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, legal_name: value }))} placeholder="Acme Billing LLC" />
                    <InputField label="Billing email" value={profileDraft.email || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, email: value }))} placeholder="billing@acme.test" />
                    <InputField label="Phone" value={profileDraft.phone || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, phone: value }))} placeholder="+1 415 555 0100" />
                    <InputField label="Tax identifier" value={profileDraft.tax_identifier || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, tax_identifier: value }))} placeholder="VAT / GST / EIN" />
                    <InputField
                      label="Tax codes"
                      value={(profileDraft.tax_codes || []).join(", ")}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, tax_codes: parseCodeList(value) }))}
                      placeholder="GST_IN, VAT_DE"
                    />
                    <InputField
                      label="Billing address line 1"
                      value={profileDraft.billing_address_line1 || ""}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_address_line1: value }))}
                      placeholder="1 Billing Street"
                    />
                    <InputField
                      label="Billing address line 2"
                      value={profileDraft.billing_address_line2 || ""}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_address_line2: value }))}
                      placeholder="Suite 200"
                    />
                    <InputField label="Billing city" value={profileDraft.billing_city || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_city: value }))} placeholder="Bengaluru" />
                    <InputField label="Billing state" value={profileDraft.billing_state || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_state: value }))} placeholder="Karnataka" />
                    <InputField
                      label="Billing postal code"
                      value={profileDraft.billing_postal_code || ""}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_postal_code: value }))}
                      placeholder="560001"
                    />
                    <InputField
                      label="Billing country"
                      value={profileDraft.billing_country || ""}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, billing_country: value }))}
                      placeholder="IN"
                    />
                    <InputField label="Currency" value={profileDraft.currency || ""} onChange={(value) => updateProfileDraft((current) => ({ ...current, currency: value }))} placeholder="USD" />
                    <InputField
                      label="Billing connection code"
                      value={profileDraft.provider_code || ""}
                      onChange={(value) => updateProfileDraft((current) => ({ ...current, provider_code: value }))}
                      placeholder="stripe_default"
                    />
                  </div>

                  <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Required fields</p>
                    <div className="mt-3 grid gap-2 md:grid-cols-2">
                      <ChecklistLine done={Boolean((profileDraft.legal_name || "").trim())} text="Legal name is set" />
                      <ChecklistLine done={Boolean((profileDraft.email || "").trim())} text="Billing email is set" />
                      <ChecklistLine done={Boolean((profileDraft.billing_address_line1 || "").trim())} text="Billing address is set" />
                      <ChecklistLine done={Boolean((profileDraft.billing_city || "").trim())} text="Billing city is set" />
                      <ChecklistLine done={Boolean((profileDraft.billing_postal_code || "").trim())} text="Billing postal code is set" />
                      <ChecklistLine done={Boolean((profileDraft.billing_country || "").trim())} text="Billing country is set" />
                      <ChecklistLine done={Boolean((profileDraft.currency || "").trim())} text="Currency is set" />
                      <ChecklistLine done={Boolean((profileDraft.tax_codes || []).length)} text="Tax codes are optional and ready when assigned" />
                      <ChecklistLine done={Boolean((profileDraft.tax_identifier || "").trim())} text="Tax identifier is optional and ready when present" />
                    </div>
                  </div>

                  <div className="mt-5 flex flex-wrap gap-3">
                    <button
                      type="button"
                      onClick={() => {
                        updateProfileFlash(null);
                        billingProfileMutation.mutate();
                      }}
                      disabled={!canWrite || !csrfToken || billingProfileMutation.isPending || !billingProfileDirty}
                      className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {billingProfileMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                      Save billing profile
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        updateProfileDraft(profileBaseline);
                      }}
                      disabled={!billingProfileDirty || billingProfileMutation.isPending}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      Reset changes
                    </button>
                  </div>

                  {profileFlash ? (
                    <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-800">
                      <p className="font-semibold text-emerald-900">{profileFlash}</p>
                      <p className="mt-2">{billingProfileReady ? "Required billing fields are complete for payment setup and invoice sync." : "Save succeeded, but the profile still needs the required readiness fields listed above."}</p>
                    </div>
                  ) : null}
                  {billingProfileMutation.isError ? (
                    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Billing profile could not be saved</p>
                      <p className="mt-2">{billingProfileMutation.error instanceof Error ? billingProfileMutation.error.message : "Saving the billing profile failed."}</p>
                    </div>
                  ) : null}
                  {!billingProfileReady ? (
                    <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                      Payment setup is blocked until the required billing fields are complete.
                    </div>
                  ) : null}
                </section>

                <section id="payment-collection" className="scroll-mt-24 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment collection</p>
                  <p className="mt-3 text-sm text-slate-600">Send the setup path here, then refresh verification.</p>
                  <div className="mt-5 rounded-2xl border border-slate-200 bg-slate-50 p-5">
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Collection status</p>
                        <h3 className="mt-2 text-lg font-semibold text-slate-950">{collectionDiagnosis?.title || "Collection status unavailable"}</h3>
                        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">
                          {collectionDiagnosis?.summary || "Collection readiness could not be derived from the current customer state."}
                        </p>
                      </div>
                      <span
                        className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${customerCollectionDiagnosisToneClass(
                          collectionDiagnosis?.tone || "warning",
                        )}`}
                      >
                        {collectionDiagnosis?.title || "Collection status unavailable"}
                      </span>
                    </div>
                    <div className="mt-4 rounded-xl border border-slate-200 bg-white px-4 py-4 text-sm text-slate-700">
                      <p className="font-semibold text-slate-950">Next step</p>
                      <p className="mt-2">{collectionDiagnosis?.nextStep || "Reload the customer detail and retry if the collection diagnosis still does not render."}</p>
                    </div>
                  </div>
                  <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
                    <div className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Customer-directed setup</p>
                      <h3 className="mt-2 text-lg font-semibold text-slate-950">Email setup request</h3>
                      <p className="mt-2 text-sm text-slate-600">Use the email request first. Resend instead of creating duplicates.</p>
                      <div className="mt-4 flex flex-wrap gap-3">
                        <button
                          type="button"
                          onClick={() => (showResendRequest ? resendSetupMutation.mutate() : requestSetupMutation.mutate())}
                          disabled={!canBeginPaymentSetup || requestSetupMutation.isPending || resendSetupMutation.isPending}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {requestSetupMutation.isPending || resendSetupMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                          {setupRequestActionLabel}
                        </button>
                        {latestRequestedCheckoutURL ? (
                          <a
                            href={latestRequestedCheckoutURL}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                          >
                            <ExternalLink className="h-4 w-4" />
                            Open latest sent link
                          </a>
                        ) : null}
                      </div>
                    </div>

                    <div className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Manual fallback</p>
                      <h3 className="mt-2 text-lg font-semibold text-slate-950">Hosted setup link</h3>
                      <p className="mt-2 text-sm text-slate-600">Use this only when you need to share the setup link directly.</p>
                      <div className="mt-4 flex flex-wrap gap-3">
                        <button
                          type="button"
                          onClick={() => beginSetupMutation.mutate()}
                          disabled={!canBeginPaymentSetup || beginSetupMutation.isPending}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {beginSetupMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                          Generate hosted setup link
                        </button>
                        {latestCheckoutURL ? (
                          <a
                            href={latestCheckoutURL}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                          >
                            <ExternalLink className="h-4 w-4" />
                            Open latest setup link
                          </a>
                        ) : null}
                      </div>
                    </div>
                  </div>

                  <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Verification and recovery</p>
                        <h3 className="mt-2 text-lg font-semibold text-slate-950">Verify before retrying</h3>
                        <p className="mt-2 max-w-2xl text-sm text-slate-600">Refresh after setup completes. Retry sync only when billing state is stale.</p>
                      </div>
                      <div className="flex flex-wrap gap-3">
                        <button
                          type="button"
                          onClick={() => refreshMutation.mutate()}
                          disabled={!canWrite || !csrfToken || refreshMutation.isPending}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {refreshMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                          Refresh payment setup
                        </button>
                        <button
                          type="button"
                          onClick={() => retryMutation.mutate()}
                          disabled={!canWrite || !csrfToken || retryMutation.isPending}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 text-sm font-medium text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
                          Retry billing sync
                        </button>
                        <Link
                          href={`/subscriptions?customer_external_id=${encodeURIComponent(customer.external_id)}`}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm text-slate-700 transition hover:bg-slate-50"
                        >
                          Open subscriptions
                        </Link>
                      </div>
                    </div>
                  </div>
                  {beginSetupMutation.isError ? (
                    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Payment setup link could not be generated</p>
                      <p className="mt-2">{beginSetupMutation.error instanceof Error ? beginSetupMutation.error.message : "Customer payment setup request failed."}</p>
                    </div>
                  ) : null}
                  {requestSetupMutation.isError || resendSetupMutation.isError ? (
                    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Payment setup request could not be sent</p>
                      <p className="mt-2">
                        {(requestSetupMutation.error instanceof Error && requestSetupMutation.error.message) ||
                          (resendSetupMutation.error instanceof Error && resendSetupMutation.error.message) ||
                          "Customer payment setup email delivery failed."}
                      </p>
                    </div>
                  ) : null}
                  {readiness.payment_setup.last_request_status === "sent" ? (
                    <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-800">
                      <p className="font-semibold text-emerald-900">Payment setup request sent</p>
                      <p className="mt-2">
                        Sent to {readiness.payment_setup.last_request_to_email || "the customer"} on {formatExactTimestamp(readiness.payment_setup.last_request_sent_at)}.
                      </p>
                    </div>
                  ) : null}
                  {readiness.payment_setup.last_request_status === "failed" ? (
                    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Latest payment setup request failed</p>
                      <p className="mt-2">{readiness.payment_setup.last_request_error || "Email delivery failed. You can resend or fall back to the hosted link."}</p>
                    </div>
                  ) : null}
                  {latestCheckoutURL ? (
                    <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-800">
                      <p className="font-semibold text-emerald-900">Hosted payment setup link ready</p>
                      <p className="mt-2">Share this manually, then refresh status once setup is complete.</p>
                    </div>
                  ) : null}
                  {!canBeginPaymentSetup && readiness.payment_setup_status !== "ready" ? (
                    <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                      Payment setup can be requested only after the customer is active and the billing profile is ready.
                    </div>
                  ) : null}
                </section>
              </div>

              <aside className="min-w-0 grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing state</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Billing customer ID" value={customer.lago_customer_id || "-"} mono />
                    <MetaItem label="Last billing sync error" value={readiness.billing_profile.last_sync_error || "-"} />
                    <MetaItem label="Last synced" value={formatExactTimestamp(readiness.billing_profile.last_synced_at)} />
                    <MetaItem label="Last verified" value={formatExactTimestamp(readiness.payment_setup.last_verified_at)} />
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer record</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Email" value={customer.email || "-"} />
                    <MetaItem label="Created" value={formatExactTimestamp(customer.created_at)} />
                    <MetaItem label="Updated" value={formatExactTimestamp(customer.updated_at)} />
                    <MetaItem label="Customer status" value={formatReadinessStatus(customer.status)} />
                  </div>
                </section>
              </aside>
            </div>
          </>
          )
        ) : null}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function StatusCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <span className={`mt-3 inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(value)}`}>
        {formatReadinessStatus(value)}
      </span>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
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

function normalizeBillingProfileDraft(input: CustomerBillingProfileInput): CustomerBillingProfileInput {
  return {
    legal_name: (input.legal_name || "").trim(),
    email: (input.email || "").trim(),
    phone: (input.phone || "").trim(),
    billing_address_line1: (input.billing_address_line1 || "").trim(),
    billing_address_line2: (input.billing_address_line2 || "").trim(),
    billing_city: (input.billing_city || "").trim(),
    billing_state: (input.billing_state || "").trim(),
    billing_postal_code: (input.billing_postal_code || "").trim(),
    billing_country: (input.billing_country || "").trim(),
    currency: (input.currency || "").trim(),
    tax_identifier: (input.tax_identifier || "").trim(),
    tax_codes: parseCodeList((input.tax_codes || []).join(",")),
    provider_code: (input.provider_code || "").trim(),
  };
}

function serializeBillingProfileDraft(input: CustomerBillingProfileInput): string {
  return JSON.stringify(normalizeBillingProfileDraft(input));
}

function requiredBillingProfileFields(input: CustomerBillingProfileInput): boolean[] {
  const normalized = normalizeBillingProfileDraft(input);
  return [
    normalized.legal_name !== "",
    normalized.email !== "",
    normalized.billing_address_line1 !== "",
    normalized.billing_city !== "",
    normalized.billing_postal_code !== "",
    normalized.billing_country !== "",
    normalized.currency !== "",
  ];
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
