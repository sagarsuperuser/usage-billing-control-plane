"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPricingMetrics } from "@/lib/api";
import { type PricingMetric } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingMetricListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [search, setSearch] = useState("");

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filtered = useMemo(() => {
    const items = metricsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.key.toLowerCase().includes(term) || item.name.toLowerCase().includes(term) || item.unit.toLowerCase().includes(term));
  }, [metricsQuery.data, search]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Metrics" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Metrics</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Metrics define what gets measured commercially. Keep them simple and stable so plans can reuse them later.
              </p>
            </div>
            <Link href="/pricing/metrics/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
              <Plus className="h-4 w-4" />
              New metric
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Workspace session required" body="Pricing metrics are workspace-scoped. Sign in with a workspace account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        <section className="grid gap-4 md:grid-cols-3">
          <MetricCard label="Total metrics" value={String(metricsQuery.data?.length ?? 0)} />
          <MetricCard label="Distinct units" value={String(new Set((metricsQuery.data ?? []).map((item) => item.unit)).size)} />
          <MetricCard label="Search results" value={String(filtered.length)} />
        </section>

        <section className="grid gap-3 xl:grid-cols-3">
          <OperatorCard title="Commercial rule" body="Keep metric keys stable so plans and reporting can rely on them." />
          <OperatorCard title="Review posture" body="Use this list for inventory review. Open detail when an operator needs aggregation or usage context." />
          <OperatorCard title="Next action" body="Create new metrics only when the commercial signal is genuinely different from existing usage records." />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Metric inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by name, code, or unit"
              className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
          <div className="mt-5 grid gap-3">
            {metricsQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((metric) => <MetricRow key={metric.id} metric={metric} />)}
          </div>
        </section>
      </main>
    </div>
  );
}

function MetricRow({ metric }: { metric: PricingMetric }) {
  return (
    <Link
      href={`/pricing/metrics/${encodeURIComponent(metric.id)}`}
      className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(4,minmax(0,0.55fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{metric.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{metric.key}</p>
        <p className="mt-2 text-sm text-slate-600">Tracks {metric.unit} using {metric.aggregation} aggregation.</p>
      </div>
      <StatusCell label="Unit" value={metric.unit} />
      <StatusCell label="Aggregation" value={metric.aggregation} />
      <StatusCell label="Created" value={new Date(metric.created_at).toLocaleDateString()} />
      <StatusCell label="Updated" value={new Date(metric.updated_at).toLocaleDateString()} />
      <StatusCell label="Key" value={metric.key} />
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
      <p className="mt-2 truncate text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading metric inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No metrics yet.</p>
      <p className="mt-2">Create the first metric to define what your plans can charge against.</p>
      <div className="mt-4">
        <Link href="/pricing/metrics/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">
          Create metric
        </Link>
      </div>
    </div>
  );
}
