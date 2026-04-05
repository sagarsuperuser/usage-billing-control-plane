
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";

import { EmptyState } from "@/components/ui/empty-state";
import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusChip } from "@/components/ui/status-chip";
import { fetchCustomerReadiness, fetchCustomers } from "@/lib/api";
import { statusTone, diagnosisTone } from "@/lib/badge";
import { diagnoseCustomerCollection } from "@/lib/customer-collection-diagnosis";
import { formatReadinessStatus } from "@/lib/readiness";
import { type CustomerReadiness } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function CustomerListScreen() {
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
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

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Customers" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <div className="flex items-center gap-3">
                <h1 className="text-sm font-semibold text-text-primary">Customers{filteredCustomers.length > 0 ? ` (${filteredCustomers.length})` : ""}</h1>
              </div>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  aria-label="Search" placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => { setStatusFilter(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="active">Active</option>
                  <option value="suspended">Suspended</option>
                  <option value="archived">Archived</option>
                </select>
                <Link to="/customers/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {sessionLoading || customersQuery.isLoading ? (
              <LoadingState />
            ) : filteredCustomers.length === 0 ? (
              <EmptyState title="No customers yet" description="Create a customer to start billing." actionLabel="New customer" actionHref="/customers/new" />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                    <th className="px-5 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Profile</th>
                    <th className="px-4 py-2.5 font-semibold">Collection</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-light">
                  {paginatedCustomers.map((customer) => {
                    const readiness = readinessByCustomer.get(customer.external_id);
                    const diagnosis = readiness ? diagnoseCustomerCollection(readiness) : null;
                    return (
                      <tr key={customer.external_id} className="transition hover:bg-surface-secondary">
                        <td className="px-5 py-3">
                          <Link to={`/customers/${encodeURIComponent(customer.external_id)}`} className="block">
                            <p className="font-medium text-text-primary">{customer.display_name}</p>
                            <p className="mt-0.5 font-mono text-xs text-text-faint">{customer.external_id}</p>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <StatusChip tone={statusTone(customer.status)}>{customer.status}</StatusChip>
                        </td>
                        <td className="px-4 py-3 text-text-muted">
                          {readiness ? formatReadinessStatus(readiness.billing_profile_status) : "—"}
                        </td>
                        <td className="px-4 py-3">
                          {diagnosis ? (
                            <StatusChip tone={diagnosisTone(diagnosis.tone)}>
                              {diagnosis.tone === "healthy" ? "Ready" : diagnosis.tone === "warning" ? "Pending" : "Blocked"}
                            </StatusChip>
                          ) : <span className="text-text-faint">—</span>}
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
      </main>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="divide-y divide-border-light">
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
