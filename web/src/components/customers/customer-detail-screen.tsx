"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, ExternalLink, LoaderCircle, RefreshCw, RotateCcw, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { beginCustomerPaymentSetup, fetchCustomerReadiness, fetchCustomers, refreshCustomerPaymentSetup, requestCustomerPaymentSetup, resendCustomerPaymentSetup, retryCustomerBillingSync } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus } from "@/lib/readiness";
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

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, externalID],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, externalID, limit: 1 }),
    enabled: isAuthenticated && scope === "tenant" && externalID.trim().length > 0,
  });

  const readinessQuery = useQuery({
    queryKey: ["customer-readiness", apiBaseURL, externalID],
    queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID }),
    enabled: isAuthenticated && scope === "tenant" && externalID.trim().length > 0,
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

  const customer = customersQuery.data?.[0] ?? null;
  const readiness = readinessQuery.data ?? null;
  const nextActions = readiness?.missing_steps.map(describeCustomerMissingStep) ?? [];
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

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Tenant" }, { href: "/customers", label: "Customers" }, { label: customer?.display_name || externalID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Customer detail is tenant-scoped. Sign in with a tenant reader, writer, or admin account to inspect readiness and run recovery actions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {customersQuery.isLoading || readinessQuery.isLoading ? (
          <LoadingPanel label="Loading customer detail" />
        ) : customersQuery.isError || readinessQuery.isError || !customer || !readiness ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Customer not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested customer could not be loaded from the tenant APIs.</p>
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
              <SummaryStat label="Billing profile" value={readiness.billing_profile_status} helper={readiness.lago_customer_synced ? "Synced to billing" : "Needs sync"} />
              <SummaryStat label="Payment setup" value={readiness.payment_setup_status} helper={readiness.default_payment_method_verified ? "Verified" : "Awaiting setup"} />
              <SummaryStat label="Open actions" value={String(readiness.missing_steps.length)} helper="Remaining checklist items" />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_420px]">
              <div className="grid gap-5">
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

                <section id="payment-collection" className="scroll-mt-24 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment collection</p>
                  <p className="mt-3 text-sm text-slate-600">
                    Use this customer page as the primary collection path when payment setup is missing or incomplete. Generate the hosted payer setup link here, then refresh verification before retrying collection elsewhere.
                  </p>
                  <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
                    <div className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Customer-directed setup</p>
                      <h3 className="mt-2 text-lg font-semibold text-slate-950">Send one clear setup path</h3>
                      <p className="mt-2 text-sm text-slate-600">
                        Use the email request as the default path. If a request already exists, resend that path instead of showing duplicate send actions.
                      </p>
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
                      <p className="mt-2 text-sm text-slate-600">
                        Generate a direct hosted setup link only when you need to share it manually outside the normal request flow.
                      </p>
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
                        <h3 className="mt-2 text-lg font-semibold text-slate-950">Confirm setup before retrying elsewhere</h3>
                        <p className="mt-2 max-w-2xl text-sm text-slate-600">
                          Refresh payment verification after the payer completes setup. Use billing sync recovery only when the customer mapping or provider state is stale.
                        </p>
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
                      <p className="mt-2">Use this fallback when you need to share the link manually, then refresh verification once setup is complete.</p>
                    </div>
                  ) : null}
                  {!canBeginPaymentSetup && readiness.payment_setup_status !== "ready" ? (
                    <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                      Payment setup can be requested only after the customer is active and the billing profile is ready.
                    </div>
                  ) : null}
                </section>
              </div>

              <aside className="grid gap-5 self-start">
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
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer metadata</p>
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
        )}
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
