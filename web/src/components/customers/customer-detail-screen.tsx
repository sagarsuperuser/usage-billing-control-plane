"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, LoaderCircle, RefreshCw, RotateCcw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchCustomerReadiness, fetchCustomers, refreshCustomerPaymentSetup, retryCustomerBillingSync } from "@/lib/api";
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

  const customer = customersQuery.data?.[0] ?? null;
  const readiness = readinessQuery.data ?? null;
  const nextActions = readiness?.missing_steps.map(describeCustomerMissingStep) ?? [];

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

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Recovery actions</p>
                  <p className="mt-3 text-sm text-slate-600">Retry billing synchronization or refresh payment-method verification when a customer is blocked.</p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    <button
                      type="button"
                      onClick={() => retryMutation.mutate()}
                      disabled={!canWrite || !csrfToken || retryMutation.isPending}
                      className="inline-flex h-10 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 text-sm font-medium text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
                      Retry billing sync
                    </button>
                    <button
                      type="button"
                      onClick={() => refreshMutation.mutate()}
                      disabled={!canWrite || !csrfToken || refreshMutation.isPending}
                      className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {refreshMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                      Refresh payment setup
                    </button>
                  </div>
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
