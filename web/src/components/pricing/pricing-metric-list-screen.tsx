
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";

import { EmptyState } from "@/components/ui/empty-state";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

const PAGE_SIZE = 20;

export function PricingMetricListScreen() {
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = metricsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.key.toLowerCase().includes(term) || item.name.toLowerCase().includes(term) || item.unit.toLowerCase().includes(term));
  }, [metricsQuery.data, search]);

  const paginated = useMemo(() => filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filtered, page]);

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Metrics" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <h1 className="text-sm font-semibold text-text-primary">Metrics{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
            <div className="flex items-center gap-2">
              <input
                value={search}
                onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                aria-label="Search" placeholder="Search..."
                className="h-8 w-48 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              />
              <Link to="/pricing/metrics/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                <Plus className="h-3.5 w-3.5" />
                New
              </Link>
            </div>
          </div>
          {sessionLoading || metricsQuery.isLoading ? (
            <LoadingState />
          ) : filtered.length === 0 ? (
            <EmptyState title="No metrics yet" description="Define what your plans charge against." actionLabel="New metric" actionHref="/pricing/metrics/new" />
          ) : (
              <>
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                      <th className="px-5 py-2.5 font-semibold">Name</th>
                      <th className="px-4 py-2.5 font-semibold">Key</th>
                      <th className="px-4 py-2.5 font-semibold">Unit</th>
                      <th className="px-4 py-2.5 font-semibold">Aggregation</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-light">
                    {paginated.map((metric) => (
                      <tr key={metric.id} className="transition hover:bg-surface-secondary">
                        <td className="px-5 py-3">
                          <Link to={`/pricing/metrics/${encodeURIComponent(metric.id)}`} className="block font-medium text-text-primary">
                            {metric.name}
                          </Link>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-text-muted">{metric.key}</td>
                        <td className="px-4 py-3 text-text-muted">{metric.unit}</td>
                        <td className="px-4 py-3 text-text-muted">{metric.aggregation}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination page={page} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
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
          <div className="flex-1"><Skeleton className="h-4 w-32" /></div>
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-20" />
        </div>
      ))}
    </div>
  );
}
