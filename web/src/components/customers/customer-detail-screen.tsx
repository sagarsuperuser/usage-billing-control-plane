"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, LoaderCircle, RefreshCw, RotateCcw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchCustomerReadiness, fetchCustomers, refreshCustomerPaymentSetup, retryCustomerBillingSync } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function tone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "pending":
    case "incomplete":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    case "sync_error":
    case "error":
      return "border-rose-400/40 bg-rose-500/10 text-rose-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
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
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#164e63_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1240px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Tenant" }, { href: "/customers", label: "Customers" }, { label: customer?.display_name || externalID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Customer detail is a tenant-scoped view. Sign in with a tenant reader, writer, or admin API key to inspect readiness and run recovery actions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {customersQuery.isLoading || readinessQuery.isLoading ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 text-sm text-slate-300 backdrop-blur-xl">
            <div className="flex items-center gap-2">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading customer detail
            </div>
          </section>
        ) : customersQuery.isError || readinessQuery.isError || !customer || !readiness ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Customer detail</p>
            <h1 className="mt-2 text-3xl font-semibold text-white">Customer not available</h1>
            <p className="mt-3 text-sm text-slate-300">The requested customer could not be loaded from the tenant APIs.</p>
            <Link href="/customers" className="mt-5 inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
              <ArrowLeft className="h-4 w-4" />
              Back to customers
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Customer detail</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-white md:text-4xl">{customer.display_name}</h1>
                  <p className="mt-2 break-all font-mono text-sm text-slate-400">{customer.external_id}</p>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <span className={`rounded-full px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] ${tone(readiness.status)}`}>
                    {formatReadinessStatus(readiness.status)}
                  </span>
                  <Link href="/customers" className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
                    <ArrowLeft className="h-4 w-4" />
                    Back to customers
                  </Link>
                  <Link href="/customers/new" className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20">
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

            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Readiness</p>
                <h2 className="mt-2 text-2xl font-semibold text-white">What still needs action</h2>
                <div className="mt-5 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                  <p className="text-sm font-semibold text-white">Next actions</p>
                  <div className="mt-3 grid gap-2">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Customer is ready for payment operations." />}
                  </div>
                </div>

                <div className="mt-4 grid gap-3 xl:grid-cols-3">
                  <StatusCard title="Billing profile" value={readiness.billing_profile_status} />
                  <StatusCard title="Payment setup" value={readiness.payment_setup_status} />
                  <StatusCard title="Overall readiness" value={readiness.status} />
                </div>

                <div className="mt-4 rounded-2xl border border-white/10 bg-slate-900/60 p-4 text-sm text-slate-200">
                  <p className="font-semibold text-white">Recovery actions</p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    <button
                      type="button"
                      onClick={() => retryMutation.mutate()}
                      disabled={!canWrite || !csrfToken || retryMutation.isPending}
                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-400/40 bg-amber-500/10 px-3 text-sm text-amber-100 transition hover:bg-amber-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
                      Retry billing sync
                    </button>
                    <button
                      type="button"
                      onClick={() => refreshMutation.mutate()}
                      disabled={!canWrite || !csrfToken || refreshMutation.isPending}
                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {refreshMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                      Refresh payment setup
                    </button>
                  </div>
                </div>
              </section>

              <aside className="flex flex-col gap-4">
                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Billing state</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Billing customer ID" value={customer.lago_customer_id || "-"} mono />
                    <MetaItem label="Last billing sync error" value={readiness.billing_profile.last_sync_error || "-"} />
                    <MetaItem label="Last synced" value={formatExactTimestamp(readiness.billing_profile.last_synced_at)} />
                    <MetaItem label="Last verified" value={formatExactTimestamp(readiness.payment_setup.last_verified_at)} />
                  </dl>
                </section>

                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Customer metadata</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Email" value={customer.email || "-"} />
                    <MetaItem label="Created" value={formatExactTimestamp(customer.created_at)} />
                    <MetaItem label="Updated" value={formatExactTimestamp(customer.updated_at)} />
                    <MetaItem label="Customer status" value={formatReadinessStatus(customer.status)} />
                  </dl>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="min-w-0 rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function StatusCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
      <p className="text-sm font-semibold text-white">{title}</p>
      <span className={`mt-3 inline-flex rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em] ${tone(value)}`}>
        {formatReadinessStatus(value)}
      </span>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/10 bg-white/5 px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-500/20 text-emerald-100" : "bg-amber-500/20 text-amber-100"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-200">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3">
      <dt className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
