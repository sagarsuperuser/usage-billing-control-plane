"use client";

import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchPricingMetric } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingMetricDetailScreen({ metricID }: { metricID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const query = useQuery({ queryKey: ["pricing-metric", apiBaseURL, metricID], queryFn: () => fetchPricingMetric({ runtimeBaseURL: apiBaseURL, metricID }), enabled: isAuthenticated && scope === "tenant" && metricID.trim().length > 0 });
  const metric = query.data;

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55"><div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" /><div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" /></div>
      <main className="relative mx-auto flex max-w-[1120px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: metric?.name || metricID }]} />
        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Metrics are tenant-scoped. Sign in with a tenant account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}
        {!metric ? <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><h1 className="text-2xl font-semibold text-white">Metric not available</h1></section> : <><section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Metric detail</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">{metric.name}</h1><p className="mt-2 break-all font-mono text-sm text-slate-400">{metric.key}</p></section><section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4"><Stat label="Unit" value={metric.unit} /><Stat label="Aggregation" value={metric.aggregation} /><Stat label="Created" value={new Date(metric.created_at).toLocaleString()} /><Stat label="Updated" value={new Date(metric.updated_at).toLocaleString()} /></section><section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Operator note</p><p className="mt-3 text-sm text-slate-300">Alpha created the underlying pricing draft automatically. Plans can now reuse this metric without exposing the lower-level billing-engine objects in the primary workflow.</p></section></>}
      </main>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) { return <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p><p className="mt-2 text-sm font-semibold text-white">{value}</p></div>; }
