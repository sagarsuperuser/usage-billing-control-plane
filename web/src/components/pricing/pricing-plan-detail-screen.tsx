"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchAddOns, fetchCoupons, fetchPlan, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanDetailScreen({ planID }: { planID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();

  const planQuery = useQuery({
    queryKey: ["pricing-plan", apiBaseURL, planID],
    queryFn: () => fetchPlan({ runtimeBaseURL: apiBaseURL, planID }),
    enabled: isAuthenticated && scope === "tenant" && planID.trim().length > 0,
  });

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });
  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });
  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: plan?.name || planID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Tenant session required" body="Plans are tenant-scoped. Sign in with a tenant account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        {planQuery.isLoading ? (
          <LoadingPanel label="Loading plan detail" />
        ) : !plan ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing plan</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Plan not available</h1>
            <Link href="/pricing/plans" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
              <ArrowLeft className="h-4 w-4" />
              Back to plans
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing plan</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{plan.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{plan.code}</p>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">{plan.description || "No description provided."}</p>
                </div>
                <Link href="/pricing/plans" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                  <ArrowLeft className="h-4 w-4" />
                  Back to plans
                </Link>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-6">
              <Stat label="Status" value={plan.status} />
              <Stat label="Interval" value={plan.billing_interval} />
              <Stat label="Base price" value={`${(plan.base_amount_cents / 100).toFixed(2)} ${plan.currency}`} />
              <Stat label="Metrics" value={String(plan.meter_ids.length)} />
              <Stat label="Add-ons" value={String((plan.add_on_ids ?? []).length)} />
              <Stat label="Coupons" value={String((plan.coupon_ids ?? []).length)} />
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Linked metrics</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Commercial inputs</h2>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">{linkedMetrics.length} linked metric(s)</div>
              </div>
              <div className="mt-5 grid gap-3">
                {linkedMetrics.length === 0 ? (
                  <EmptyPanel message="No linked metrics were found for this plan." />
                ) : (
                  linkedMetrics.map((metric) =>
                    metric ? (
                      <div key={metric.id} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1fr)_140px_140px] lg:items-center">
                        <div className="min-w-0">
                          <p className="text-sm font-semibold text-slate-950">{metric.name}</p>
                          <p className="mt-1 break-all font-mono text-xs text-slate-500">{metric.key}</p>
                        </div>
                        <InfoCell label="Aggregation" value={metric.aggregation} />
                        <InfoCell label="Unit" value={metric.unit} />
                      </div>
                    ) : null,
                  )
                )}
              </div>
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Attached coupons</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Commercial relief</h2>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">{linkedCoupons.length} attached coupon(s)</div>
              </div>
              <div className="mt-5 grid gap-3">
                {linkedCoupons.length === 0 ? (
                  <EmptyPanel message="No coupons are attached to this plan." />
                ) : (
                  linkedCoupons.map((coupon) =>
                    coupon ? (
                      <div key={coupon.id} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1fr)_180px_140px] lg:items-center">
                        <div className="min-w-0">
                          <p className="text-sm font-semibold text-slate-950">{coupon.name}</p>
                          <p className="mt-1 break-all font-mono text-xs text-slate-500">{coupon.code}</p>
                        </div>
                        <InfoCell label="Discount" value={coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`} />
                        <InfoCell label="Status" value={coupon.status} />
                      </div>
                    ) : null,
                  )
                )}
              </div>
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Attached add-ons</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Recurring extras</h2>
                </div>
                <div className="rounded-lg border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">{linkedAddOns.length} attached add-on(s)</div>
              </div>
              <div className="mt-5 grid gap-3">
                {linkedAddOns.length === 0 ? (
                  <EmptyPanel message="No add-ons are attached to this plan." />
                ) : (
                  linkedAddOns.map((addOn) =>
                    addOn ? (
                      <div key={addOn.id} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1fr)_160px_140px] lg:items-center">
                        <div className="min-w-0">
                          <p className="text-sm font-semibold text-slate-950">{addOn.name}</p>
                          <p className="mt-1 break-all font-mono text-xs text-slate-500">{addOn.code}</p>
                        </div>
                        <InfoCell label="Amount" value={`${(addOn.amount_cents / 100).toFixed(2)} ${addOn.currency}`} />
                        <InfoCell label="Interval" value={addOn.billing_interval} />
                      </div>
                    ) : null,
                  )
                )}
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InfoCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function EmptyPanel({ message }: { message: string }) {
  return <p className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-600">{message}</p>;
}
