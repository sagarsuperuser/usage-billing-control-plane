"use client";

import Link from "next/link";
import { Plus, Receipt } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchTaxes } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

const PAGE_SIZE = 20;

function statusTone(status: string): string {
  switch (status.toLowerCase()) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "draft":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "archived":
      return "border-slate-200 bg-slate-50 text-slate-500";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

export function PricingTaxListScreen() {
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const taxesQuery = useQuery({
    queryKey: ["pricing-taxes", apiBaseURL],
    queryFn: () => fetchTaxes({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = taxesQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term));
  }, [taxesQuery.data, search]);

  const paginated = useMemo(() => filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filtered, page]);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Taxes" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Taxes{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <Link href="/pricing/taxes/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {sessionLoading || taxesQuery.isLoading ? (
              <LoadingState />
            ) : filtered.length === 0 ? (
              <EmptyState />
            ) : (
              <>
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                      <th className="px-5 py-2.5 font-semibold">Name</th>
                      <th className="px-4 py-2.5 font-semibold">Code</th>
                      <th className="px-4 py-2.5 font-semibold">Rate</th>
                      <th className="px-4 py-2.5 font-semibold">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-100">
                    {paginated.map((tax) => (
                      <tr key={tax.id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link href={`/pricing/taxes/${encodeURIComponent(tax.id)}`} className="block font-medium text-slate-900">
                            {tax.name}
                          </Link>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-slate-500">{tax.code}</td>
                        <td className="px-4 py-3 text-slate-600">{tax.rate.toFixed(2)}%</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(tax.status)}`}>
                            {tax.status}
                          </span>
                        </td>
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
    <div className="divide-y divide-stone-100">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-32" /></div>
          <Skeleton className="h-3 w-20" />
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
      <Receipt className="h-8 w-8 text-slate-300" />
      <p className="text-sm font-medium text-slate-700">No taxes yet</p>
      <p className="text-xs text-slate-500">Create the first tax, then assign it to customer profiles or workspace billing settings.</p>
      <Link href="/pricing/taxes/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New tax
      </Link>
    </div>
  );
}
