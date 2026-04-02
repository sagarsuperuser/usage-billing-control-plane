"use client";

import Link from "next/link";
import { ArrowRight, LoaderCircle, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchAddOns, fetchCoupons, fetchPlans, fetchPricingMetrics, fetchTaxes } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingHomeScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const requiresTenantSession = isAuthenticated && !isTenantSession;

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const plansQuery = useQuery({
    queryKey: ["pricing-plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
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

  const taxesQuery = useQuery({
    queryKey: ["pricing-taxes", apiBaseURL],
    queryFn: () => fetchTaxes({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const loading = metricsQuery.isLoading || plansQuery.isLoading || addOnsQuery.isLoading || couponsQuery.isLoading || taxesQuery.isLoading;
  const metricCount = metricsQuery.data?.length ?? 0;
  const planCount = plansQuery.data?.length ?? 0;
  const addOnCount = addOnsQuery.data?.length ?? 0;
  const couponCount = couponsQuery.data?.length ?? 0;
  const taxCount = taxesQuery.data?.length ?? 0;
  const draftPlanCount = (plansQuery.data ?? []).filter((plan) => plan.status === "draft").length;
  const activePlanCount = (plansQuery.data ?? []).filter((plan) => plan.status === "active").length;
  const activeTaxCount = (taxesQuery.data ?? []).filter((tax) => tax.status === "active").length;
  const catalogCount = metricCount + taxCount + addOnCount + couponCount + planCount;

  const catalogRows = [
    {
      label: "Metrics",
      itemLabel: "metric",
      count: metricCount,
      summary: metricCount > 0 ? `${metricCount} reusable usage definitions` : "No metric definitions yet",
      posture: metricCount > 0 ? "Ready for plan design" : "Required before plans can price usage",
      href: "/pricing/metrics",
      createHref: "/pricing/metrics/new",
      createLabel: "New metric",
    },
    {
      label: "Plans",
      itemLabel: "plan",
      count: planCount,
      summary: planCount > 0 ? `${activePlanCount} active / ${draftPlanCount} draft` : "No plans yet",
      posture: metricCount > 0 ? (planCount > 0 ? "Review activation posture" : "Ready to create first plan") : "Blocked on metrics",
      href: "/pricing/plans",
      createHref: "/pricing/plans/new",
      createLabel: "New plan",
    },
    {
      label: "Add-ons",
      itemLabel: "add-on",
      count: addOnCount,
      summary: addOnCount > 0 ? `${addOnCount} reusable recurring extras` : "No reusable extras yet",
      posture: addOnCount > 0 ? "Attach through plan packages" : "Optional catalog extension",
      href: "/pricing/add-ons",
      createHref: "/pricing/add-ons/new",
      createLabel: "New add-on",
    },
    {
      label: "Coupons",
      itemLabel: "coupon",
      count: couponCount,
      summary: couponCount > 0 ? `${couponCount} commercial relief rules` : "No discount rules yet",
      posture: couponCount > 0 ? "Apply through plans or follow-up" : "Optional launch or retention tool",
      href: "/pricing/coupons",
      createHref: "/pricing/coupons/new",
      createLabel: "New coupon",
    },
    {
      label: "Taxes",
      itemLabel: "tax",
      count: taxCount,
      summary: taxCount > 0 ? `${activeTaxCount} active / ${taxCount - activeTaxCount} inactive` : "No reusable tax rules yet",
      posture: taxCount > 0 ? "Assign through billing settings" : "Optional until tax handling is needed",
      href: "/pricing/taxes",
      createHref: "/pricing/taxes/new",
      createLabel: "New tax",
    },
  ] as const;

  const setupQueue = catalogRows.filter((row) => row.count === 0);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Workspace" }, { label: "Pricing" }]} />

        {isTenantSession ? <section className="rounded-2xl border border-slate-200 bg-white px-6 py-5 shadow-sm">
          <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
            <div className="max-w-3xl">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Pricing catalog</h1>
              <p className="mt-3 text-sm text-slate-600">
                Define metrics and plans once, then reuse them across customers. Metrics define what gets charged; plans set the price and cadence.
              </p>
            </div>
            {isTenantSession ? (
              <div className="flex flex-wrap gap-3">
                <Link href="/pricing/metrics/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-4 w-4" />
                  New metric
                </Link>
                <Link href="/pricing/plans/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                  <Plus className="h-4 w-4" />
                  New plan
                </Link>
              </div>
            ) : null}
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {requiresTenantSession ? (
          <ScopeNotice
            title="Workspace session required"
            body="Pricing is workspace-scoped. Sign in with a workspace account to define metrics and plans."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {!isAuthenticated || !isTenantSession ? null : (
          <>
            <section className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <SummaryCell label="Catalog records" value={catalogCount} hint="Metrics, plans, add-ons, coupons, and taxes" />
              <SummaryCell label="Active plans" value={activePlanCount} hint={planCount > 0 ? `${draftPlanCount} draft` : "No active plans yet"} />
              <SummaryCell label="Reusable rules" value={metricCount + addOnCount + couponCount + taxCount} hint="Reusable inputs before customer assignment" />
              <SummaryCell label="Immediate gaps" value={setupQueue.length} hint={setupQueue.length > 0 ? "Domains still missing a first record" : "Core pricing inventory is present"} />
            </section>

            {loading ? (
              <section className="rounded-2xl border border-slate-200 bg-white px-4 py-6 text-sm text-slate-600 shadow-sm">
                <div className="flex items-center gap-2">
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                  Loading pricing inventory
                </div>
              </section>
            ) : (
              <div className="grid gap-5 xl:grid-cols-[minmax(0,1.6fr)_360px]">
                <section className="rounded-2xl border border-slate-200 bg-white shadow-sm">
                  <div className="border-b border-slate-200 px-6 py-4">
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <h2 className="text-xl font-semibold text-slate-950">Pricing setup</h2>
                        <p className="mt-1 text-sm text-slate-600">Configure metrics, plans, add-ons, coupons, and taxes for your workspace.</p>
                      </div>
                    </div>
                  </div>

                  <div className="hidden grid-cols-[180px_110px_minmax(0,1fr)_auto] gap-4 border-b border-slate-200 px-6 py-3 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500 lg:grid">
                    <span>Domain</span>
                    <span>Count</span>
                    <span>Status</span>
                    <span>Actions</span>
                  </div>

                  <div className="divide-y divide-slate-200">
                    {catalogRows.map((row) => (
                      <CatalogRow key={row.label} {...row} />
                    ))}
                  </div>
                </section>

                <div className="grid gap-5">
                  <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Operating model</p>
                    <h2 className="mt-2 text-xl font-semibold text-slate-950">Commercial setup sequence</h2>
                    <div className="mt-5 grid gap-3">
                      <SequenceStep number="1" title="Define metrics" body="Create stable usage records first so plans are built on reusable measurement rules." />
                      <SequenceStep number="2" title="Package plans" body="Assemble customer-facing plans from base price, metrics, add-ons, and coupons." />
                      <SequenceStep number="3" title="Add optional rules" body="Attach taxes, add-ons, and coupons only where they change commercial behavior clearly." />
                    </div>
                  </section>

                  <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Immediate queue</p>
                    <h2 className="mt-2 text-xl font-semibold text-slate-950">Setup gaps</h2>
                    <div className="mt-5 grid gap-3">
                      {setupQueue.length > 0 ? (
                        setupQueue.map((row) => (
                          <Link key={row.label} href={row.createHref} className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 transition hover:border-slate-300 hover:bg-slate-100">
                            <div className="flex items-center justify-between gap-3">
                              <div>
                                <p className="text-sm font-semibold text-slate-950">Create first {row.itemLabel}</p>
                                <p className="mt-1 text-sm text-slate-600">{row.posture}</p>
                              </div>
                              <ArrowRight className="h-4 w-4 text-slate-500" />
                            </div>
                          </Link>
                        ))
                      ) : (
                        <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-900">
                          Core pricing inventory is in place. Use the catalog table to review counts, open records, and create additional variants only when commercial scope changes.
                        </div>
                      )}
                    </div>
                  </section>
                </div>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function SummaryCell({ label, value, hint }: { label: string; value: number; hint: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <div className="mt-2 flex items-end justify-between gap-3">
        <p className="text-2xl font-semibold text-slate-950">{value}</p>
        <p className="max-w-[180px] text-right text-xs leading-relaxed text-slate-500">{hint}</p>
      </div>
    </div>
  );
}

function CatalogRow({
  label,
  count,
  summary,
  posture,
  href,
  createHref,
  createLabel,
}: {
  label: string;
  itemLabel: string;
  count: number;
  summary: string;
  posture: string;
  href: string;
  createHref: string;
  createLabel: string;
}) {
  return (
    <div className="grid gap-4 px-6 py-5 lg:grid-cols-[180px_110px_minmax(0,1fr)_auto] lg:items-center">
      <div>
        <p className="text-sm font-semibold text-slate-950">{label}</p>
        <p className="mt-1 text-sm text-slate-500 lg:hidden">{summary}</p>
      </div>
      <div>
        <p className="text-sm font-semibold text-slate-950">{count}</p>
        <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-500">records</p>
      </div>
      <div>
        <p className="text-sm text-slate-700">{summary}</p>
        <p className="mt-1 text-sm text-slate-500">{posture}</p>
      </div>
      <div className="flex flex-wrap gap-2 lg:justify-end">
        <Link href={href} className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-700 transition hover:bg-slate-100">
          Open
          <ArrowRight className="h-4 w-4" />
        </Link>
        <Link href={createHref} className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
          <Plus className="h-4 w-4" />
          {createLabel}
        </Link>
      </div>
    </div>
  );
}

function SequenceStep({ number, title, body }: { number: string; title: string; body: string }) {
  return (
    <div className="grid grid-cols-[32px_minmax(0,1fr)] gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
      <div className="flex h-8 w-8 items-center justify-center rounded-full border border-slate-300 bg-white text-xs font-semibold text-slate-700">{number}</div>
      <div>
        <p className="text-sm font-semibold text-slate-950">{title}</p>
        <p className="mt-1 text-sm text-slate-600">{body}</p>
      </div>
    </div>
  );
}
