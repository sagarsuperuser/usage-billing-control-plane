"use client";

import Link from "next/link";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchAddOns, fetchCoupons, fetchPlans, fetchPricingMetrics, fetchTaxes } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingHomeScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const plansQuery = useQuery({
    queryKey: ["pricing-plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
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
  const taxesQuery = useQuery({
    queryKey: ["pricing-taxes", apiBaseURL],
    queryFn: () => fetchTaxes({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const loading = metricsQuery.isLoading || plansQuery.isLoading || addOnsQuery.isLoading || couponsQuery.isLoading || taxesQuery.isLoading;
  const metricCount = metricsQuery.data?.length ?? 0;
  const planCount = plansQuery.data?.length ?? 0;
  const addOnCount = addOnsQuery.data?.length ?? 0;
  const couponCount = couponsQuery.data?.length ?? 0;
  const taxCount = taxesQuery.data?.length ?? 0;
  const draftPlanCount = (plansQuery.data ?? []).filter((plan) => plan.status === "draft").length;
  const activePlanCount = (plansQuery.data ?? []).filter((plan) => plan.status === "active").length;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Workspace" }, { label: "Pricing" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing console</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Pricing foundation</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">
            Define what gets measured and how customers are charged without leaving Alpha. Start with stable metrics, then compose plans on top of them.
          </p>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Pricing is workspace-scoped. Sign in with a workspace account to define metrics and plans."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-7">
          <MetricCard label="Metrics" value={metricCount} />
          <MetricCard label="Taxes" value={taxCount} />
          <MetricCard label="Add-ons" value={addOnCount} />
          <MetricCard label="Coupons" value={couponCount} />
          <MetricCard label="Plans" value={planCount} />
          <MetricCard label="Draft plans" value={draftPlanCount} />
          <MetricCard label="Active plans" value={activePlanCount} />
        </section>

        {loading ? (
          <section className="rounded-2xl border border-slate-200 bg-white px-4 py-6 text-sm text-slate-600 shadow-sm">
            <div className="flex items-center gap-2">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading pricing inventory
            </div>
          </section>
        ) : (
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)]">
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Operating posture</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Pricing inventory</h2>
                </div>
              </div>
              <div className="mt-5 grid gap-3">
                <InventoryRow label="Metrics" summary={metricCount > 0 ? `${metricCount} configured` : "Create the first metric"} href="/pricing/metrics" action="Open" />
                <InventoryRow label="Taxes" summary={taxCount > 0 ? `${taxCount} reusable rules` : "Create the first tax rule"} href="/pricing/taxes" action="Open" />
                <InventoryRow label="Add-ons" summary={addOnCount > 0 ? `${addOnCount} recurring extras` : "Create the first add-on"} href="/pricing/add-ons" action="Open" />
                <InventoryRow label="Coupons" summary={couponCount > 0 ? `${couponCount} discount rules` : "Create the first coupon"} href="/pricing/coupons" action="Open" />
                <InventoryRow label="Plans" summary={planCount > 0 ? `${activePlanCount} active / ${draftPlanCount} draft` : "Create the first commercial package"} href="/pricing/plans" action="Open" />
              </div>
            </section>

            <section className="grid gap-5 xl:grid-cols-2">
            <DomainPanel
              eyebrow="Metrics"
              title="What gets measured"
              body="Stable usage records that plans can reuse."
              href="/pricing/metrics"
              cta="Open metrics"
              secondaryHref="/pricing/metrics/new"
              secondaryLabel="New metric"
              stats={[
                { label: "Total", value: String(metricCount) },
                { label: "Next step", value: metricCount > 0 ? "Review inventory" : "Create first metric" },
              ]}
            />
            <DomainPanel
              eyebrow="Taxes"
              title="Tax catalog and application"
              body="Reusable tax rules for customer and workspace billing settings."
              href="/pricing/taxes"
              cta="Open taxes"
              secondaryHref="/pricing/taxes/new"
              secondaryLabel="New tax"
              stats={[
                { label: "Total", value: String(taxCount) },
                { label: "Use", value: taxCount > 0 ? "Assign to profiles" : "Create first tax" },
              ]}
            />
            <DomainPanel
              eyebrow="Add-ons"
              title="Package recurring extras"
              body="Reusable fixed-price extras that can be attached to plans."
              href="/pricing/add-ons"
              cta="Open add-ons"
              secondaryHref="/pricing/add-ons/new"
              secondaryLabel="New add-on"
              stats={[
                { label: "Total", value: String(addOnCount) },
                { label: "Use", value: addOnCount > 0 ? "Attach to plans" : "Create first add-on" },
              ]}
            />
            <DomainPanel
              eyebrow="Coupons"
              title="Model commercial relief"
              body="Reusable discount rules for launches, promotions, and negotiated offers."
              href="/pricing/coupons"
              cta="Open coupons"
              secondaryHref="/pricing/coupons/new"
              secondaryLabel="New coupon"
              stats={[
                { label: "Total", value: String(couponCount) },
                { label: "Use", value: couponCount > 0 ? "Attach to plans" : "Create first coupon" },
              ]}
            />
            <DomainPanel
              eyebrow="Plans"
              title="How customers are charged"
              body="Commercial packages built from a base price plus reusable pricing inputs."
              href="/pricing/plans"
              cta="Open plans"
              secondaryHref="/pricing/plans/new"
              secondaryLabel="New plan"
              stats={[
                { label: "Total", value: String(planCount) },
                { label: "Drafts", value: String(draftPlanCount) },
              ]}
            />
            </section>
          </div>
        )}
      </main>
    </div>
  );
}

function DomainPanel({
  eyebrow,
  title,
  body,
  href,
  cta,
  secondaryHref,
  secondaryLabel,
  stats,
}: {
  eyebrow: string;
  title: string;
  body: string;
  href: string;
  cta: string;
  secondaryHref: string;
  secondaryLabel: string;
  stats: Array<{ label: string; value: string }>;
}) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">{eyebrow}</p>
          <h2 className="mt-2 text-xl font-semibold text-slate-950">{title}</h2>
          <p className="mt-3 text-sm leading-relaxed text-slate-600">{body}</p>
        </div>
      </div>
      <div className="mt-5 grid gap-3 sm:grid-cols-2">
        {stats.map((stat) => (
          <div key={stat.label} className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{stat.label}</p>
            <p className="mt-2 text-sm font-semibold text-slate-950">{stat.value}</p>
          </div>
        ))}
      </div>
      <div className="mt-5 flex flex-wrap gap-3">
        <Link href={href} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
          {cta}
          <ArrowRight className="h-4 w-4" />
        </Link>
        <Link href={secondaryHref} className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
          {secondaryLabel}
        </Link>
      </div>
    </section>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InventoryRow({ label, summary, href, action }: { label: string; summary: string; href: string; action: string }) {
  return (
    <Link href={href} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 transition hover:border-slate-300 hover:bg-slate-100 md:grid-cols-[180px_minmax(0,1fr)_auto] md:items-center">
      <p className="text-sm font-semibold text-slate-950">{label}</p>
      <p className="text-sm text-slate-600">{summary}</p>
      <span className="inline-flex items-center gap-2 text-sm font-medium text-slate-700">
        {action}
        <ArrowRight className="h-4 w-4" />
      </span>
    </Link>
  );
}
