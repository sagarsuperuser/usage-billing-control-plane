
import { Link } from "@tanstack/react-router";
import { CreditCard, ExternalLink, RefreshCw, RotateCcw, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { FormField } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { LoadingSkeleton } from "@/components/ui/loading-skeleton";
import { PageContainer } from "@/components/ui/page-container";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { StatusChip } from "@/components/ui/status-chip";
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
import { statusTone } from "@/lib/badge";
import { customerCollectionDiagnosisToneClass, diagnoseCustomerCollection } from "@/lib/customer-collection-diagnosis";
import { showError, showSuccess } from "@/lib/toast";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { type CustomerBillingProfile, type CustomerBillingProfileInput } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

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
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, isLoading: _sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [profileFlash, setProfileFlash] = useState<string | null>(null);
  const [editingProfile, setEditingProfile] = useState(false);
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
      showSuccess("Billing profile saved");
      setEditingProfile(false);
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

  const profileStatusDot = readiness?.billing_profile_status === "ready" ? "bg-emerald-500" : readiness?.billing_profile_status === "sync_error" ? "bg-rose-500" : "bg-amber-500";
  const paymentDot = readiness?.default_payment_method_verified ? "bg-emerald-500" : readiness?.payment_setup_status === "error" ? "bg-rose-500" : "bg-slate-300";
  const syncDot = readiness?.billing_profile?.last_sync_error ? "bg-rose-500" : readiness?.billing_profile?.last_synced_at ? "bg-emerald-500" : "bg-slate-300";

  const formatAddress = (bp: CustomerBillingProfile | null) => {
    if (!bp) return null;
    const parts = [bp.billing_address_line1, bp.billing_address_line2, bp.billing_city, bp.billing_state, bp.billing_postal_code, bp.billing_country].filter(Boolean);
    return parts.length > 0 ? parts.join(", ") : null;
  };

  return (
    <PageContainer>
        <AppBreadcrumbs items={[{ href: "/customers", label: "Customers" }, { label: customer?.display_name || externalID }]} />

        {isTenantSession ? (
          customersQuery.isLoading || readinessQuery.isLoading ? (
            <LoadingSkeleton variant="card" />
          ) : customersQuery.isError || readinessQuery.isError || !customer || !readiness ? (
            <section className="rounded-lg border border-border bg-surface shadow-sm p-5">
              <p className="text-sm font-semibold text-text-primary">Customer not available</p>
              <p className="mt-1 text-sm text-text-muted">The requested customer could not be loaded.</p>
            </section>
          ) : (
          <SectionErrorBoundary>
            {/* ── Header ─────────────────────────────────────────── */}
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3 min-w-0">
                <h1 className="text-lg font-semibold text-text-primary truncate">{customer.display_name}</h1>
                <span className="font-mono text-xs text-text-faint">{customer.external_id}</span>
                <StatusChip tone={statusTone(readiness.status)}>{formatReadinessStatus(readiness.status)}</StatusChip>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Link to={`/invoices?customer_external_id=${encodeURIComponent(customer.external_id)}`} className="inline-flex h-8 items-center rounded-lg border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                  View invoices
                </Link>
                <Link to={`/subscriptions/new`} className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3 text-xs font-semibold text-white shadow-sm transition hover:bg-blue-700">
                  Create subscription
                </Link>
              </div>
            </div>

            {/* ── Status summary (3 cards) ────────────────────────── */}
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-lg border border-border bg-surface p-4">
                <p className="text-xs font-medium text-text-muted">Billing profile</p>
                <div className="mt-2 flex items-center gap-2">
                  <span className={`inline-block h-2 w-2 rounded-full ${profileStatusDot}`} />
                  <span className="text-sm font-medium text-text-primary">{formatReadinessStatus(readiness.billing_profile_status)}</span>
                </div>
              </div>
              <div className="rounded-lg border border-border bg-surface p-4">
                <p className="text-xs font-medium text-text-muted">Payment method</p>
                <div className="mt-2 flex items-center gap-2">
                  <span className={`inline-block h-2 w-2 rounded-full ${paymentDot}`} />
                  <span className="text-sm font-medium text-text-primary">{readiness.default_payment_method_verified ? "Verified" : formatReadinessStatus(readiness.payment_setup_status)}</span>
                </div>
                {canBeginPaymentSetup ? (
                  <Button variant="primary" size="xs" className="mt-2" onClick={() => (showResendRequest ? resendSetupMutation.mutate() : requestSetupMutation.mutate())} loading={requestSetupMutation.isPending || resendSetupMutation.isPending}>
                    {!(requestSetupMutation.isPending || resendSetupMutation.isPending) ? <Send className="h-2.5 w-2.5" /> : null}
                    {showResendRequest ? "Resend link" : "Send setup link"}
                  </Button>
                ) : null}
              </div>
              <div className="rounded-lg border border-border bg-surface p-4">
                <p className="text-xs font-medium text-text-muted">Stripe sync</p>
                <div className="mt-2 flex items-center gap-2">
                  <span className={`inline-block h-2 w-2 rounded-full ${syncDot}`} />
                  <span className="text-sm font-medium text-text-primary">{readiness.billing_profile?.last_sync_error ? "Error" : readiness.billing_profile?.last_synced_at ? "Connected" : "Pending"}</span>
                </div>
                {readiness.billing_profile?.last_sync_error ? (
                  <Button variant="secondary" size="xs" className="mt-2" onClick={() => retryMutation.mutate()} loading={retryMutation.isPending} disabled={!canWrite || !csrfToken}>
                    {!retryMutation.isPending ? <RotateCcw className="h-2.5 w-2.5" /> : null}
                    Retry sync
                  </Button>
                ) : null}
              </div>
            </div>

            {/* ── Next step banner (only if not ready) ───────────── */}
            {collectionDiagnosis && collectionDiagnosis.tone !== "healthy" ? (
              <div className={`flex items-center gap-4 rounded-lg border px-5 py-4 ${collectionDiagnosis.tone === "danger" ? "border-rose-200 bg-rose-50 dark:border-rose-900 dark:bg-rose-950" : "border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950"}`}>
                <div className="flex-1 min-w-0">
                  <p className={`text-sm font-semibold ${collectionDiagnosis.tone === "danger" ? "text-rose-800 dark:text-rose-200" : "text-amber-800 dark:text-amber-200"}`}>{collectionDiagnosis.title}</p>
                  <p className={`mt-0.5 text-xs ${collectionDiagnosis.tone === "danger" ? "text-rose-700 dark:text-rose-300" : "text-amber-700 dark:text-amber-300"}`}>{collectionDiagnosis.nextStep}</p>
                </div>
              </div>
            ) : null}

            {/* ── Billing details (read-only / edit toggle) ──────── */}
            <div className="rounded-lg border border-border bg-surface shadow-sm">
              <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
                <p className="text-sm font-semibold text-text-primary">Billing details</p>
                <Button variant="secondary" size="xs" onClick={() => setEditingProfile(!editingProfile)}>
                  {editingProfile ? "Cancel" : "Edit"}
                </Button>
              </div>

              {editingProfile ? (
                <div className="p-5">
                  <div className="grid gap-3 md:grid-cols-2">
                    <FormField label="Legal name"><Input placeholder="Acme Billing LLC" {...register("legal_name")} /></FormField>
                    <FormField label="Billing email"><Input placeholder="billing@acme.test" {...register("email")} /></FormField>
                    <FormField label="Phone"><Input placeholder="+1 415 555 0100" {...register("phone")} /></FormField>
                    <FormField label="Tax identifier"><Input placeholder="VAT / GST / EIN" {...register("tax_identifier")} /></FormField>
                    <FormField label="Tax codes"><Input placeholder="GST_IN, VAT_DE" {...register("tax_codes_raw")} /></FormField>
                    <FormField label="Address line 1"><Input placeholder="1 Billing Street" {...register("billing_address_line1")} /></FormField>
                    <FormField label="Address line 2"><Input placeholder="Suite 200" {...register("billing_address_line2")} /></FormField>
                    <FormField label="City"><Input placeholder="Bengaluru" {...register("billing_city")} /></FormField>
                    <FormField label="State"><Input placeholder="Karnataka" {...register("billing_state")} /></FormField>
                    <FormField label="Postal code"><Input placeholder="560001" {...register("billing_postal_code")} /></FormField>
                    <FormField label="Country"><Input placeholder="IN" {...register("billing_country")} /></FormField>
                    <FormField label="Currency"><Input placeholder="USD" {...register("currency")} /></FormField>
                  </div>
                  <div className="mt-4 flex gap-2">
                    <Button variant="primary" onClick={handleProfileSubmit((data) => { setProfileFlash(null); billingProfileMutation.mutate(data); })} disabled={!canWrite || !csrfToken || !billingProfileDirty} loading={billingProfileMutation.isPending}>
                      Save
                    </Button>
                    <Button variant="secondary" onClick={() => setEditingProfile(false)}>
                      Cancel
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="p-5">
                  <dl className="grid gap-3 sm:grid-cols-2">
                    {billingProfile?.legal_name ? <div><dt className="text-xs text-text-faint">Legal name</dt><dd className="mt-0.5 text-sm text-text-secondary">{billingProfile.legal_name}</dd></div> : null}
                    {billingProfile?.email ? <div><dt className="text-xs text-text-faint">Email</dt><dd className="mt-0.5 text-sm text-text-secondary">{billingProfile.email}</dd></div> : null}
                    {billingProfile?.phone ? <div><dt className="text-xs text-text-faint">Phone</dt><dd className="mt-0.5 text-sm text-text-secondary">{billingProfile.phone}</dd></div> : null}
                    {formatAddress(billingProfile) ? <div><dt className="text-xs text-text-faint">Address</dt><dd className="mt-0.5 text-sm text-text-secondary">{formatAddress(billingProfile)}</dd></div> : null}
                    {billingProfile?.currency ? <div><dt className="text-xs text-text-faint">Currency</dt><dd className="mt-0.5 text-sm text-text-secondary">{billingProfile.currency}</dd></div> : null}
                    {billingProfile?.tax_identifier ? <div><dt className="text-xs text-text-faint">Tax ID</dt><dd className="mt-0.5 text-sm text-text-secondary">{billingProfile.tax_identifier}</dd></div> : null}
                  </dl>
                  {!billingProfile?.legal_name && !billingProfile?.email ? (
                    <p className="text-sm text-text-faint italic">No billing details yet. Click Edit to add.</p>
                  ) : null}
                </div>
              )}
            </div>

            {/* ── Payment setup (only if billing profile ready) ──── */}
            {readiness.billing_profile_status === "ready" ? (
              <div className="rounded-lg border border-border bg-surface shadow-sm">
                <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
                  <p className="text-sm font-semibold text-text-primary">Payment setup</p>
                  <div className="flex items-center gap-2">
                    <Button variant="secondary" size="xs" onClick={() => refreshMutation.mutate()} loading={refreshMutation.isPending} disabled={!canWrite || !csrfToken}>
                      {!refreshMutation.isPending ? <RefreshCw className="h-2.5 w-2.5" /> : null}
                      Refresh
                    </Button>
                  </div>
                </div>
                <div className="p-5">
                  {readiness.default_payment_method_verified ? (
                    <div className="flex items-center gap-2">
                      <span className="inline-block h-2 w-2 rounded-full bg-emerald-500" />
                      <span className="text-sm font-medium text-text-primary">Payment method verified</span>
                      {readiness.payment_setup.last_verified_at ? (
                        <span className="text-xs text-text-faint">Last verified {formatExactTimestamp(readiness.payment_setup.last_verified_at)}</span>
                      ) : null}
                    </div>
                  ) : (
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="inline-block h-2 w-2 rounded-full bg-slate-300" />
                        <span className="text-sm text-text-muted">{readiness.payment_setup_status === "pending" ? "Awaiting customer setup" : "Not configured"}</span>
                      </div>
                      {canBeginPaymentSetup ? (
                        <div className="mt-3 flex flex-wrap gap-2">
                          <Button variant="primary" size="sm" onClick={() => (showResendRequest ? resendSetupMutation.mutate() : requestSetupMutation.mutate())} loading={requestSetupMutation.isPending || resendSetupMutation.isPending}>
                            {!(requestSetupMutation.isPending || resendSetupMutation.isPending) ? <Send className="h-3 w-3" /> : null}
                            {showResendRequest ? "Resend setup email" : "Send setup email"}
                          </Button>
                          <Button variant="secondary" size="sm" onClick={() => beginSetupMutation.mutate()} loading={beginSetupMutation.isPending}>
                            {!beginSetupMutation.isPending ? <ExternalLink className="h-3 w-3" /> : null}
                            Generate link
                          </Button>
                          {(latestCheckoutURL || latestRequestedCheckoutURL) ? (
                            <a href={latestCheckoutURL || latestRequestedCheckoutURL} target="_blank" rel="noreferrer" className="inline-flex h-7 items-center gap-1.5 rounded-md border border-border bg-surface px-2.5 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                              <ExternalLink className="h-3 w-3" />
                              Open link
                            </a>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  )}
                </div>
              </div>
            ) : null}
          </SectionErrorBoundary>
          )
        ) : null}
    </PageContainer>
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
