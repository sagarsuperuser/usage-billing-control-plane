"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchCustomerReadiness, fetchCustomers } from "@/lib/api";
import { customerCollectionDiagnosisToneClass, diagnoseCustomerCollection } from "@/lib/customer-collection-diagnosis";
import { describeCustomerMissingStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { type Customer, type CustomerReadiness } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function tone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
    case "active":
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
    return customers.filter((customer) => customer.external_id.toLowerCase().includes(term) || customer.display_name.toLowerCase().includes(term));
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
      pendingPayment: readiness.filter((item) => diagnoseCustomerCollection(item).code === "awaiting_customer_setup" || diagnoseCustomerCollection(item).code === "setup_request_failed" || diagnoseCustomerCollection(item).code === "collection_missing").length,
      syncErrors: readiness.filter((item) => diagnoseCustomerCollection(item).code === "billing_sync_error").length,
    };
  }, [filteredCustomers, readinessByCustomer]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Tenant" }, { label: "Customers" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customers</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Customer directory</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Browse billing readiness, payment setup state, and recovery needs. Customer creation stays in the dedicated setup flow.
              </p>
            </div>
            <Link href="/customers/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
              <Plus className="h-4 w-4" />
              New customer
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Customer directory is tenant-scoped. Sign in with a tenant reader, writer, or admin account to browse customer readiness."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Visible customers" value={summary.total} />
          <MetricCard label="Billing ready" value={summary.ready} />
          <MetricCard label="Collection blocked" value={summary.pendingPayment} />
          <MetricCard label="Sync recovery" value={summary.syncErrors} />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by name or customer ID"
                className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
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
  const nextStep = normalizeMissingSteps(readiness?.missing_steps)[0];
  const diagnosis = readiness ? diagnoseCustomerCollection(readiness) : null;
  return (
    <Link
      href={`/customers/${encodeURIComponent(customer.external_id)}`}
      className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(4,minmax(0,0.52fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="truncate text-base font-semibold text-slate-950">{customer.display_name}</h3>
          <span className={`rounded-full border px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(customer.status)}`}>
            {customer.status}
          </span>
          {diagnosis ? (
            <span
              className={`rounded-full border px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${customerCollectionDiagnosisToneClass(
                diagnosis.tone,
              )}`}
            >
              {diagnosis.title}
            </span>
          ) : null}
        </div>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{customer.external_id}</p>
        <p className="mt-2 text-sm text-slate-600">
          {diagnosis?.summary || (nextStep ? `Next action: ${describeCustomerMissingStep(nextStep)}` : "No immediate customer blocker is shown.")}
        </p>
      </div>
      <StatusCell label="Overall" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
      <StatusCell label="Profile" value={readiness ? formatReadinessStatus(readiness.billing_profile_status) : "Loading"} />
      <StatusCell label="Payments" value={readiness ? formatReadinessStatus(readiness.payment_setup_status) : "Loading"} />
      <StatusCell
        label="Collection"
        value={
          diagnosis
            ? diagnosis.tone === "healthy"
              ? "Ready"
              : diagnosis.tone === "warning"
                ? "Pending"
                : "Blocked"
            : "Review"
        }
      />
      <StatusCell label="Status" value={customer.status} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-slate-700">
        Open
        <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading customer inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No customers match the current filters.</p>
      <p className="mt-2">Clear filters or create a new customer to start the tenant billing journey.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/customers/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">Create customer</Link>
      </div>
    </div>
  );
}
