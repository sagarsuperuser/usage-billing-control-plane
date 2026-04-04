"use client";

import Link from "next/link";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchCustomerReadiness, fetchCustomers } from "@/lib/api";
import { customerCollectionDiagnosisToneClass, diagnoseCustomerCollection } from "@/lib/customer-collection-diagnosis";
import { formatReadinessStatus } from "@/lib/readiness";
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
  const [page, setPage] = useState(1);

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

  const PAGE_SIZE = 20;
  const paginatedCustomers = useMemo(() => filteredCustomers.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filteredCustomers, page]);

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
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Customers" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Customer directory is workspace-scoped. Sign in with a workspace reader, writer, or admin account to view customers."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <div className="flex items-center gap-3">
                <h1 className="text-sm font-semibold text-slate-900">Customers{filteredCustomers.length > 0 ? ` (${filteredCustomers.length})` : ""}</h1>
              </div>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => { setStatusFilter(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="active">Active</option>
                  <option value="suspended">Suspended</option>
                  <option value="archived">Archived</option>
                </select>
                <Link href="/customers/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {customersQuery.isLoading ? (
              <LoadingState />
            ) : filteredCustomers.length === 0 ? (
              <EmptyState />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Profile</th>
                    <th className="px-4 py-2.5 font-semibold">Collection</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {paginatedCustomers.map((customer) => {
                    const readiness = readinessByCustomer.get(customer.external_id);
                    const diagnosis = readiness ? diagnoseCustomerCollection(readiness) : null;
                    return (
                      <tr key={customer.external_id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link href={`/customers/${encodeURIComponent(customer.external_id)}`} className="block">
                            <p className="font-medium text-slate-900">{customer.display_name}</p>
                            <p className="mt-0.5 font-mono text-xs text-slate-400">{customer.external_id}</p>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${tone(customer.status)}`}>
                            {customer.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-600">
                          {readiness ? formatReadinessStatus(readiness.billing_profile_status) : "—"}
                        </td>
                        <td className="px-4 py-3">
                          {diagnosis ? (
                            <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${customerCollectionDiagnosisToneClass(diagnosis.tone)}`}>
                              {diagnosis.tone === "healthy" ? "Ready" : diagnosis.tone === "warning" ? "Pending" : "Blocked"}
                            </span>
                          ) : <span className="text-slate-400">—</span>}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <Pagination page={page} pageSize={PAGE_SIZE} total={filteredCustomers.length} onPageChange={setPage} />
              </>
            )}
          </div>
        ) : null}
      </main>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="divide-y divide-stone-100">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-32" /><Skeleton className="mt-1 h-3 w-20" /></div>
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-4 w-14 rounded-full" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No customers</p>
      <p className="text-xs text-slate-500">Create a customer to get started.</p>
      <Link href="/customers/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New customer
      </Link>
    </div>
  );
}
