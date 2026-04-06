
import { Link, useNavigate } from "@tanstack/react-router";
import { Plus } from "lucide-react";

import { EmptyState } from "@/components/ui/empty-state";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchAddOns } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

const PAGE_SIZE = 20;

function statusTone(status: string): string {
  switch (status.toLowerCase()) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "draft":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "archived":
      return "border-border bg-surface-secondary text-text-muted";
    default:
      return "border-border bg-surface-secondary text-text-secondary";
  }
}

export function PricingAddOnListScreen() {
  const navigate = useNavigate();
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = addOnsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term) || item.currency.toLowerCase().includes(term));
  }, [addOnsQuery.data, search]);

  const paginated = useMemo(() => filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filtered, page]);

  return (
    <PageContainer>
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Add-ons" }]} />


        <Card>
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h1 className="text-sm font-semibold text-text-primary">Add-ons{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  aria-label="Search" placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                />
                <Link to="/pricing/add-ons/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {sessionLoading || addOnsQuery.isLoading ? (
              <LoadingState />
            ) : filtered.length === 0 ? (
              <EmptyState title="No add-ons yet" description="Create add-on charges for your plans." actionLabel="New add-on" actionHref="/pricing/add-ons/new" />
            ) : (
              <>
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                      <th className="px-5 py-2.5 font-semibold">Name</th>
                      <th className="px-4 py-2.5 font-semibold">Code</th>
                      <th className="px-4 py-2.5 font-semibold">Status</th>
                      <th className="px-4 py-2.5 font-semibold">Amount</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-light">
                    {paginated.map((addOn) => (
                      <tr key={addOn.id} className="cursor-pointer transition hover:bg-surface-secondary" onClick={() => navigate({ to: `/pricing/add-ons/${encodeURIComponent(addOn.id)}` })}>
                        <td className="px-5 py-3 font-medium text-text-primary">
                          {addOn.name}
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-text-muted">{addOn.code}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(addOn.status)}`}>
                            {addOn.status}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-text-muted">{(addOn.amount_cents / 100).toFixed(2)} {addOn.currency.toUpperCase()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination page={page} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
              </>
            )}
          </Card>
    </PageContainer>
  );
}

function LoadingState() {
  return (
    <div className="divide-y divide-border-light">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-32" /></div>
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-20" />
        </div>
      ))}
    </div>
  );
}
