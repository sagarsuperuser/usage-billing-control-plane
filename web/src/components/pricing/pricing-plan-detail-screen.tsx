"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchPlan, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanDetailScreen({ planID }: { planID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const planQuery = useQuery({ queryKey: ["pricing-plan", apiBaseURL, planID], queryFn: () => fetchPlan({ runtimeBaseURL: apiBaseURL, planID }), enabled: isAuthenticated && scope === "tenant" && planID.trim().length > 0 });
  const metricsQuery = useQuery({ queryKey: ["pricing-metrics", apiBaseURL], queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated && scope === "tenant" });
  const plan = planQuery.data;
  const linkedMetrics = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((metricsQuery.data ?? []).map((metric) => [metric.id, metric]));
    return plan.meter_ids.map((id) => byID.get(id)).filter(Boolean);
  }, [metricsQuery.data, plan]);

  return <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100"><div className="pointer-events-none absolute inset-0 opacity-55"><div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" /><div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" /></div><main className="relative mx-auto flex max-w-[1120px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10"><ControlPlaneNav /><AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: plan?.name || planID }]} />{!isAuthenticated ? <LoginRedirectNotice /> : null}{isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Plans are tenant-scoped. Sign in with a tenant account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}{!plan ? <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><h1 className="text-2xl font-semibold text-white">Plan not available</h1></section> : <><section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Plan detail</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">{plan.name}</h1><p className="mt-2 break-all font-mono text-sm text-slate-400">{plan.code}</p><p className="mt-3 text-sm text-slate-300">{plan.description || "No description provided."}</p></section><section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4"><Stat label="Status" value={plan.status} /><Stat label="Interval" value={plan.billing_interval} /><Stat label="Base price" value={`${(plan.base_amount_cents / 100).toFixed(2)} ${plan.currency}`} /><Stat label="Metrics" value={String(plan.meter_ids.length)} /></section><section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Linked metrics</p><div className="mt-4 grid gap-3">{linkedMetrics.length === 0 ? <p className="text-sm text-slate-400">No linked metrics were found for this plan.</p> : linkedMetrics.map((metric) => metric ? <div key={metric.id} className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-4"><p className="text-sm font-semibold text-white">{metric.name}</p><p className="mt-1 font-mono text-xs text-slate-400">{metric.key}</p><p className="mt-2 text-sm text-slate-300">{metric.aggregation} aggregation • {metric.unit}</p></div> : null)}</div></section></>}</main></div>;
}

function Stat({ label, value }: { label: string; value: string }) { return <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p><p className="mt-2 text-sm font-semibold text-white">{value}</p></div>; }
