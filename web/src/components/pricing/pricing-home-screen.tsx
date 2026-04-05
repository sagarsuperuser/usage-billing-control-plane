
import { Link } from "@tanstack/react-router";
import { ArrowRight, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import {
  fetchAddOns,
  fetchCoupons,
  fetchPlans,
  fetchPricingMetrics,
  fetchTaxes,
} from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingHomeScreen() {
  const { apiBaseURL, isAuthenticated } = useUISession();

  const metricsQuery = useQuery({ queryKey: ["pricing-metrics", apiBaseURL], queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated });
  const plansQuery = useQuery({ queryKey: ["plans", apiBaseURL], queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated });
  const addOnsQuery = useQuery({ queryKey: ["add-ons", apiBaseURL], queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated });
  const couponsQuery = useQuery({ queryKey: ["coupons", apiBaseURL], queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated });
  const taxesQuery = useQuery({ queryKey: ["taxes", apiBaseURL], queryFn: () => fetchTaxes({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated });

  const loading = metricsQuery.isLoading || plansQuery.isLoading || addOnsQuery.isLoading || couponsQuery.isLoading || taxesQuery.isLoading;
  const metricCount = metricsQuery.data?.length ?? 0;
  const planCount = plansQuery.data?.length ?? 0;
  const activePlanCount = (plansQuery.data ?? []).filter((p) => p.status === "active").length;
  const addOnCount = addOnsQuery.data?.length ?? 0;
  const couponCount = couponsQuery.data?.length ?? 0;
  const taxCount = taxesQuery.data?.length ?? 0;

  const catalogRows = [
    {
      label: "Metrics",
      count: metricCount,
      status: metricCount > 0 ? `${metricCount} defined` : "None yet",
      href: "/pricing/metrics",
      createHref: "/pricing/metrics/new",
    },
    {
      label: "Plans",
      count: planCount,
      status: planCount > 0 ? `${activePlanCount} active` : "None yet",
      href: "/pricing/plans",
      createHref: "/pricing/plans/new",
    },
    {
      label: "Add-ons",
      count: addOnCount,
      status: addOnCount > 0 ? `${addOnCount} defined` : "None yet",
      href: "/pricing/add-ons",
      createHref: "/pricing/add-ons/new",
    },
    {
      label: "Coupons",
      count: couponCount,
      status: couponCount > 0 ? `${couponCount} defined` : "None yet",
      href: "/pricing/coupons",
      createHref: "/pricing/coupons/new",
    },
    {
      label: "Taxes",
      count: taxCount,
      status: taxCount > 0 ? `${taxCount} defined` : "None yet",
      href: "/pricing/taxes",
      createHref: "/pricing/taxes/new",
    },
  ] as const;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }]} />


        {isAuthenticated ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h1 className="text-sm font-semibold text-text-primary">Pricing catalog</h1>
              <div className="flex items-center gap-2">
                <Link to="/pricing/metrics/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New metric
                </Link>
                <Link to="/pricing/plans/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-border px-3 text-xs font-medium text-text-muted transition hover:bg-surface-secondary">
                  <Plus className="h-3.5 w-3.5" />
                  New plan
                </Link>
              </div>
            </div>

            {loading ? (
              <div className="animate-pulse space-y-3 px-5 py-8">
                <div className="h-4 w-full rounded bg-surface-secondary" />
                <div className="h-4 w-full rounded bg-surface-secondary" />
                <div className="h-4 w-3/4 rounded bg-surface-secondary" />
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                    <th className="px-5 py-2.5 font-semibold">Category</th>
                    <th className="px-4 py-2.5 font-semibold">Records</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-light">
                  {catalogRows.map((row) => (
                    <tr key={row.label} className="transition hover:bg-surface-secondary">
                      <td className="px-5 py-3 font-medium text-text-primary">{row.label}</td>
                      <td className="px-4 py-3 text-text-muted">{row.count}</td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${
                          row.count > 0
                            ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                            : "border-border bg-surface-secondary text-text-muted"
                        }`}>
                          {row.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Link to={row.href} className="inline-flex h-7 items-center gap-1 rounded-md border border-border px-2.5 text-xs text-text-muted transition hover:bg-surface-secondary">
                            Open <ArrowRight className="h-3 w-3" />
                          </Link>
                          <Link to={row.createHref} className="inline-flex h-7 items-center gap-1 rounded-md border border-border px-2.5 text-xs text-text-muted transition hover:bg-surface-secondary">
                            <Plus className="h-3 w-3" /> New
                          </Link>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        ) : null}
      </main>
    </div>
  );
}
