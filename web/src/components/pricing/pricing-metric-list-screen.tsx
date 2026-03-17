"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
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
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>
      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Metrics" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Pricing</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Metrics</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">Metrics define what gets measured commercially. Keep them simple and stable so plans can reuse them later.</p>
            </div>
            <Link href="/pricing/metrics/new" className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20">
              <Plus className="h-4 w-4" />
              New metric
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Tenant session required" body="Pricing metrics are tenant-scoped. Sign in with a tenant account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Metric inventory</p>
              <h2 className="mt-2 text-2xl font-semibold text-white">Browse and inspect</h2>
            </div>
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search by name, code, or unit" className="h-11 min-w-[260px] rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2" />
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
    <Link href={`/pricing/metrics/${encodeURIComponent(metric.id)}`} className="grid gap-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 transition hover:border-cyan-400/40 hover:bg-cyan-500/5 lg:grid-cols-[minmax(0,1.1fr)_repeat(3,minmax(0,0.55fr))_auto] lg:items-center">
      <div className="min-w-0">
        <h3 className="truncate text-lg font-semibold text-white">{metric.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-400">{metric.key}</p>
        <p className="mt-2 text-sm text-slate-300">Tracks {metric.unit} using {metric.aggregation} aggregation.</p>
      </div>
      <StatusCell label="Unit" value={metric.unit} />
      <StatusCell label="Aggregation" value={metric.aggregation} />
      <StatusCell label="Created" value={new Date(metric.created_at).toLocaleDateString()} />
      <StatusCell label="Updated" value={new Date(metric.updated_at).toLocaleDateString()} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-cyan-100">Open <ChevronRight className="h-4 w-4" /></span>
    </Link>
  );
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3"><p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p><p className="mt-2 text-sm font-semibold text-white">{value}</p></div>;
}

function LoadingState() { return <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-6 text-sm text-slate-300"><LoaderCircle className="h-4 w-4 animate-spin" />Loading metric inventory</div>; }
function EmptyState() { return <div className="rounded-2xl border border-dashed border-white/10 bg-slate-950/40 px-5 py-8 text-sm text-slate-300"><p className="font-semibold text-white">No metrics yet.</p><p className="mt-2 text-slate-400">Create the first metric to define what your plans can charge against.</p><div className="mt-4"><Link href="/pricing/metrics/new" className="inline-flex h-10 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-cyan-100 transition hover:bg-cyan-500/20">Create metric</Link></div></div>; }
