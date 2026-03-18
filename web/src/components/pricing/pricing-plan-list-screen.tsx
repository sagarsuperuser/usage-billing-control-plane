"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPlans, fetchPricingMetrics } from "@/lib/api";
import { type Plan, type PricingMetric } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [search, setSearch] = useState("");

  const plansQuery = useQuery({
    queryKey: ["pricing-plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filtered = useMemo(() => {
    const items = plansQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term) || item.currency.toLowerCase().includes(term));
  }, [plansQuery.data, search]);

  const metricsByID = useMemo(() => new Map((metricsQuery.data ?? []).map((metric) => [metric.id, metric])), [metricsQuery.data]);
  const draftCount = (plansQuery.data ?? []).filter((plan) => plan.status === "draft").length;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Plans" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Plans</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Plans define how customers are charged. Keep the first version simple: a base price, a cadence, and one or more linked metrics.
              </p>
            </div>
            <Link href="/pricing/plans/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
              <Plus className="h-4 w-4" />
              New plan
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Tenant session required" body="Plans are tenant-scoped. Sign in with a tenant account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        <section className="grid gap-4 md:grid-cols-3">
          <MetricCard label="Total plans" value={String(plansQuery.data?.length ?? 0)} />
          <MetricCard label="Draft plans" value={String(draftCount)} />
          <MetricCard label="Search results" value={String(filtered.length)} />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Plan inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by name, code, or currency"
              className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
          <div className="mt-5 grid gap-3">
            {plansQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((plan) => <PlanRow key={plan.id} plan={plan} metricsByID={metricsByID} />)}
          </div>
        </section>
      </main>
    </div>
  );
}

function PlanRow({ plan, metricsByID }: { plan: Plan; metricsByID: Map<string, PricingMetric> }) {
  const metricLabel = plan.meter_ids.map((id) => metricsByID.get(id)?.name || id).slice(0, 2).join(", ");

  return (
    <Link
      href={`/pricing/plans/${encodeURIComponent(plan.id)}`}
      className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(4,minmax(0,0.55fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{plan.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{plan.code}</p>
        <p className="mt-2 text-sm text-slate-600">{metricLabel ? `Includes ${metricLabel}` : "No metrics linked yet."}</p>
      </div>
      <StatusCell label="Status" value={plan.status} />
      <StatusCell label="Interval" value={plan.billing_interval} />
      <StatusCell label="Base price" value={`${(plan.base_amount_cents / 100).toFixed(2)} ${plan.currency}`} />
      <StatusCell label="Metrics" value={String(plan.meter_ids.length)} />
      <StatusCell label="Currency" value={plan.currency.toUpperCase()} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-slate-700">
        Open
        <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading plan inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No plans yet.</p>
      <p className="mt-2">Create the first plan after you have at least one metric.</p>
      <div className="mt-4">
        <Link href="/pricing/plans/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">
          Create plan
        </Link>
      </div>
    </div>
  );
}
