"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { Skeleton } from "@/components/ui/skeleton";
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
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, statusFilter],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, status: statusFilter || undefined, limit: 100 }),
    enabled: isTenantSession,
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
      enabled: isTenantSession,
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
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { label: "Customers" }]} />

        {isTenantSession ? <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customers</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Customers</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                View billing readiness, payment setup status, and recovery needs across all customers.
              </p>
            </div>
            {isTenantSession ? (
              <Link href="/customers/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                <Plus className="h-4 w-4" />
                New customer
              </Link>
            ) : null}
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Customer directory is workspace-scoped. Sign in with a workspace reader, writer, or admin account to browse customer readiness."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <>
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
          </>
        ) : null}
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
      className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-white lg:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_110px] lg:items-start"
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
      <InventoryCell label="Readiness" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
      <InventoryCell label="Profile" value={readiness ? formatReadinessStatus(readiness.billing_profile_status) : "Loading"} />
      <InventoryCell
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
      <div className="flex items-center justify-between gap-3 lg:justify-end">
        <p className="text-[11px] text-slate-400">View →</p>
      </div>
    </Link>
  );
}

function InventoryCell({ label, value }: { label: string; value: string }) {
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
    <div className="grid gap-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_110px] lg:items-start">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <Skeleton className="h-5 w-36" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
            <Skeleton className="mt-2 h-3 w-28" />
            <Skeleton className="mt-2 h-4 w-56" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-14" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-12" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="flex items-center justify-end">
            <Skeleton className="h-3.5 w-8" />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No customers match the current filters.</p>
      <p className="mt-2">Create a customer to get started.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/customers/new" className="inline-flex h-9 items-center rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">New customer</Link>
      </div>
    </div>
  );
}
