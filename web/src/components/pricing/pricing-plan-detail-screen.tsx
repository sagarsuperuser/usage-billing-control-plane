"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchAddOns, fetchCoupons, fetchPlan, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

function statusBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "draft":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "archived":
      return "border-slate-200 bg-slate-100 text-slate-500";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

export function PricingPlanDetailScreen({ planID }: { planID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const planQuery = useQuery({
    queryKey: ["pricing-plan", apiBaseURL, planID],
    queryFn: () => fetchPlan({ runtimeBaseURL: apiBaseURL, planID }),
    enabled: isTenantSession && planID.trim().length > 0,
  });

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const plan = planQuery.data ?? null;
  const linkedMetrics = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((metricsQuery.data ?? []).map((metric) => [metric.id, metric]));
    return plan.meter_ids.map((id) => byID.get(id)).filter(Boolean);
  }, [metricsQuery.data, plan]);
  const linkedAddOns = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((addOnsQuery.data ?? []).map((item) => [item.id, item]));
    return (plan.add_on_ids ?? []).map((id) => byID.get(id)).filter(Boolean);
  }, [addOnsQuery.data, plan]);
  const linkedCoupons = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((couponsQuery.data ?? []).map((item) => [item.id, item]));
    return (plan.coupon_ids ?? []).map((id) => byID.get(id)).filter(Boolean);
  }, [couponsQuery.data, plan]);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: plan?.name || planID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? planQuery.isLoading ? (
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center gap-2 text-sm text-slate-500">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading plan detail
            </div>
          </section>
        ) : !plan ? (
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <p className="text-sm font-semibold text-slate-900">Plan not available</p>
            <p className="mt-1 text-sm text-slate-500">The requested plan could not be loaded.</p>
            <Link href="/pricing/plans" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to plans
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-slate-200 bg-white shadow-sm divide-y divide-slate-200">
              {/* Header */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900 truncate">{plan.name}</h1>
                    <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${statusBadgeClass(plan.status)}`}>
                      {plan.status}
                    </span>
                  </div>
                  <Link href="/pricing/plans" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to plans
                  </Link>
                </div>
                {plan.description ? <p className="mt-1.5 text-xs text-slate-500">{plan.description}</p> : null}
              </div>

              {/* Details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Code</dt>
                    <dd className="mt-0.5 text-sm text-slate-700 font-mono">{plan.code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Currency</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{plan.currency}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Interval</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{plan.billing_interval}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Base price</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{(plan.base_amount_cents / 100).toFixed(2)} {plan.currency}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Status</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{plan.status}</dd>
                  </div>
                </dl>
              </div>

              {/* Linked metrics */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-2">Linked metrics ({linkedMetrics.length})</p>
                {linkedMetrics.length === 0 ? (
                  <p className="text-sm text-slate-500">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedMetrics.map((metric) =>
                      metric ? (
                        <li key={metric.id} className="text-sm text-slate-700">
                          {metric.name} <span className="font-mono text-xs text-slate-400">{metric.key}</span> &middot; {metric.aggregation} &middot; {metric.unit}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>

              {/* Linked add-ons */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-2">Linked add-ons ({linkedAddOns.length})</p>
                {linkedAddOns.length === 0 ? (
                  <p className="text-sm text-slate-500">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedAddOns.map((addOn) =>
                      addOn ? (
                        <li key={addOn.id} className="text-sm text-slate-700">
                          {addOn.name} <span className="font-mono text-xs text-slate-400">{addOn.code}</span> &middot; {(addOn.amount_cents / 100).toFixed(2)} {addOn.currency} &middot; {addOn.billing_interval}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>

              {/* Linked coupons */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-2">Linked coupons ({linkedCoupons.length})</p>
                {linkedCoupons.length === 0 ? (
                  <p className="text-sm text-slate-500">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedCoupons.map((coupon) =>
                      coupon ? (
                        <li key={coupon.id} className="text-sm text-slate-700">
                          {coupon.name} <span className="font-mono text-xs text-slate-400">{coupon.code}</span> &middot; {coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>
            </div>
          </SectionErrorBoundary>
        ) : null}
      </main>
    </div>
  );
}
