"use client";

import Link from "next/link";
import { Plus, Ruler } from "lucide-react";
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
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
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
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Metrics" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Metrics{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <Link href="/pricing/metrics/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {metricsQuery.isLoading ? (
              <LoadingState />
            ) : filtered.length === 0 ? (
              <EmptyState />
            ) : (
              <>
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                      <th className="px-5 py-2.5 font-semibold">Name</th>
                      <th className="px-4 py-2.5 font-semibold">Key</th>
                      <th className="px-4 py-2.5 font-semibold">Unit</th>
                      <th className="px-4 py-2.5 font-semibold">Aggregation</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-100">
                    {paginated.map((metric) => (
                      <tr key={metric.id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link href={`/pricing/metrics/${encodeURIComponent(metric.id)}`} className="block font-medium text-slate-900">
                            {metric.name}
                          </Link>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-slate-500">{metric.key}</td>
                        <td className="px-4 py-3 text-slate-600">{metric.unit}</td>
                        <td className="px-4 py-3 text-slate-600">{metric.aggregation}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination page={page} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
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
          <div className="flex-1"><Skeleton className="h-4 w-32" /></div>
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-3 w-20" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
      <Ruler className="h-8 w-8 text-slate-300" />
      <p className="text-sm font-medium text-slate-700">No metrics yet</p>
      <p className="text-xs text-slate-500">Create the first metric to define what your plans can charge against.</p>
      <Link href="/pricing/metrics/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New metric
      </Link>
    </div>
  );
}
