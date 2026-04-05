
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";

import { EmptyState } from "@/components/ui/empty-state";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchSubscriptions } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

function statusTone(status: string): string {
  switch (status) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending_payment_setup":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "action_required":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-border bg-surface-secondary text-text-secondary";
  }
}

export function SubscriptionListScreen() {
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);

  const subscriptionsQuery = useQuery({
    queryKey: ["subscriptions", apiBaseURL],
    queryFn: () => fetchSubscriptions({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = subscriptionsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) =>
      [item.display_name, item.code, item.customer_display_name, item.customer_external_id, item.plan_name, item.plan_code].some((value) => value.toLowerCase().includes(term)),
    );
  }, [subscriptionsQuery.data, search]);

  const PAGE_SIZE = 20;
  const paginated = useMemo(() => filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filtered, page]);

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Subscriptions" }]} />


        <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h1 className="text-sm font-semibold text-text-primary">Subscriptions{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  aria-label="Search" placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                />
                <Link to="/subscriptions/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {sessionLoading || subscriptionsQuery.isLoading ? (
              <LoadingState />
            ) : filtered.length === 0 ? (
              <EmptyState title="No subscriptions yet" description="Subscribe a customer to a plan." actionLabel="New subscription" actionHref="/subscriptions/new" />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                    <th className="px-5 py-2.5 font-semibold">Name</th>
                    <th className="px-4 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Plan</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Billing</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-light">
                  {paginated.map((item) => (
                    <tr key={item.id} className="transition hover:bg-surface-secondary">
                      <td className="px-5 py-3">
                        <Link to={`/subscriptions/${encodeURIComponent(item.id)}`} className="block">
                          <p className="font-medium text-text-primary">{item.display_name}</p>
                          <p className="mt-0.5 font-mono text-xs text-text-faint">{item.code}</p>
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text-muted">{item.customer_display_name}</td>
                      <td className="px-4 py-3 text-text-muted">{item.plan_name}</td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(item.status)}`}>
                          {item.status.replaceAll("_", " ")}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-text-muted">
                        {item.billing_interval} · {(item.base_amount_cents / 100).toFixed(2)} {item.currency}
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
    <div className="divide-y divide-border-light">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-32" /><Skeleton className="mt-1 h-3 w-20" /></div>
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-28" />
        </div>
      ))}
    </div>
  );
}
