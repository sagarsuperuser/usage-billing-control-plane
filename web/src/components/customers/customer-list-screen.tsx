"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchCustomerReadiness, fetchCustomers } from "@/lib/api";
import { describeCustomerMissingStep, formatReadinessStatus } from "@/lib/readiness";
import { type Customer, type CustomerReadiness } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function tone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
    case "active":
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

export function CustomerListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, statusFilter],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, status: statusFilter || undefined, limit: 100 }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filteredCustomers = useMemo(() => {
    const customers = customersQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return customers;
    return customers.filter((customer) =>
      customer.external_id.toLowerCase().includes(term) || customer.display_name.toLowerCase().includes(term)
    );
  }, [search, customersQuery.data]);

  const readinessQueries = useQueries({
    queries: filteredCustomers.map((customer) => ({
      queryKey: ["customer-readiness", apiBaseURL, customer.external_id],
      queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID: customer.external_id }),
      enabled: isAuthenticated && scope === "tenant",
    })),
  });

  const readinessByCustomer = useMemo(() => {
    const map = new Map<string, CustomerReadiness>();
    readinessQueries.forEach((query, index) => {
      const customer = filteredCustomers[index];
      if (customer && query.data) map.set(customer.external_id, query.data);
    });
    return map;
  }, [filteredCustomers, readinessQueries]);

  const summary = useMemo(() => {
    const readiness = filteredCustomers.flatMap((customer) => {
      const item = readinessByCustomer.get(customer.external_id);
      return item ? [item] : [];
    });
    return {
      total: filteredCustomers.length,
      ready: readiness.filter((item) => item.status === "ready").length,
      pendingPayment: readiness.filter((item) => item.payment_setup_status !== "ready").length,
      syncErrors: readiness.filter((item) => item.billing_profile_status === "sync_error").length,
    };
  }, [filteredCustomers, readinessByCustomer]);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#164e63_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Tenant" }, { label: "Customers" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Tenant Directory</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Customers</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Browse customer billing readiness, payment setup state, and recovery needs. Creation now lives in a dedicated customer setup flow.
              </p>
            </div>
            <Link
              href="/customers/new"
              className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
            >
              <Plus className="h-4 w-4" />
              New customer
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Customer directory is a tenant-scoped view. Sign in with a tenant reader, writer, or admin API key to browse customer readiness."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Visible customers" value={summary.total} />
          <MetricCard label="Billing ready" value={summary.ready} tone="success" />
          <MetricCard label="Waiting on payment" value={summary.pendingPayment} tone="warn" />
          <MetricCard label="Sync errors" value={summary.syncErrors} tone="error" />
        </section>

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Customer list</p>
              <h2 className="mt-2 text-2xl font-semibold text-white">Browse and inspect</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by name or customer ID"
                className="h-11 min-w-[260px] rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="active">Active</option>
                <option value="suspended">Suspended</option>
                <option value="archived">Archived</option>
              </select>
            </div>
          </div>

          <div className="mt-5 grid gap-3">
            {customersQuery.isLoading ? (
              <LoadingState />
            ) : filteredCustomers.length === 0 ? (
              <EmptyState />
            ) : (
              filteredCustomers.map((customer) => (
                <CustomerRow key={customer.external_id} customer={customer} readiness={readinessByCustomer.get(customer.external_id)} />
              ))
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

function CustomerRow({ customer, readiness }: { customer: Customer; readiness?: CustomerReadiness }) {
  const nextStep = readiness?.missing_steps[0];
  return (
    <Link
      href={`/customers/${encodeURIComponent(customer.external_id)}`}
      className="grid gap-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 transition hover:border-cyan-400/40 hover:bg-cyan-500/5 lg:grid-cols-[minmax(0,1.1fr)_repeat(3,minmax(0,0.55fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="truncate text-lg font-semibold text-white">{customer.display_name}</h3>
          <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${tone(customer.status)}`}>
            {customer.status}
          </span>
        </div>
        <p className="mt-1 break-all font-mono text-xs text-slate-400">{customer.external_id}</p>
        <p className="mt-2 text-sm text-slate-300">
          {nextStep ? `Next action: ${describeCustomerMissingStep(nextStep)}` : "Customer is ready for billing operations."}
        </p>
      </div>
      <StatusCell label="Overall" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
      <StatusCell label="Profile" value={readiness ? formatReadinessStatus(readiness.billing_profile_status) : "Loading"} />
      <StatusCell label="Payments" value={readiness ? formatReadinessStatus(readiness.payment_setup_status) : "Loading"} />
      <StatusCell label="Billing sync" value={customer.lago_customer_id ? "Synced" : "Missing"} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-cyan-100">
        Open
        <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 text-sm font-semibold text-white">{value}</p>
    </div>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" | "error" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-amber-100" : tone === "error" ? "text-rose-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-6 text-sm text-slate-300">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading customer inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-2xl border border-dashed border-white/10 bg-slate-950/40 px-5 py-8 text-sm text-slate-300">
      <p className="font-semibold text-white">No customers match the current filters.</p>
      <p className="mt-2 text-slate-400">Clear filters or create a new customer to start the tenant billing journey.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/customers/new" className="inline-flex h-10 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-cyan-100 transition hover:bg-cyan-500/20">Create customer</Link>
      </div>
    </div>
  );
}
