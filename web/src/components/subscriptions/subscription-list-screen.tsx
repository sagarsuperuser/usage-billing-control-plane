
import { Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchSubscriptions } from "@/lib/api";
import { type SubscriptionSummary } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function formatSubscriptionPaymentSetupStatus(status: SubscriptionSummary["payment_setup_status"]): string {
  switch (status) {
    case "missing":
      return "Not requested";
    case "pending":
      return "Pending";
    case "ready":
      return "Ready";
    case "error":
      return "Action required";
    default:
      return status;
  }
}

function statusTone(status: string): string {
  switch (status) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending_payment_setup":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "action_required":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
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
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Subscriptions" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Subscriptions{filtered.length > 0 ? ` (${filtered.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => { setSearch(event.target.value); setPage(1); }}
                  placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
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
              <EmptyState />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Name</th>
                    <th className="px-4 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Plan</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Billing</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {paginated.map((item) => (
                    <tr key={item.id} className="transition hover:bg-stone-50">
                      <td className="px-5 py-3">
                        <Link to={`/subscriptions/${encodeURIComponent(item.id)}`} className="block">
                          <p className="font-medium text-slate-900">{item.display_name}</p>
                          <p className="mt-0.5 font-mono text-xs text-slate-400">{item.code}</p>
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-slate-600">{item.customer_display_name}</td>
                      <td className="px-4 py-3 text-slate-600">{item.plan_name}</td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(item.status)}`}>
                          {item.status.replaceAll("_", " ")}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-slate-600">
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
    <div className="divide-y divide-stone-100">
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

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No subscriptions</p>
      <p className="text-xs text-slate-500">Create a subscription after you have at least one customer and one plan.</p>
      <Link to="/subscriptions/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New subscription
      </Link>
    </div>
  );
}
