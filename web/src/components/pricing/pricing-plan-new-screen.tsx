"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { createPlan, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [billingInterval, setBillingInterval] = useState("monthly");
  const [status, setStatus] = useState("draft");
  const [baseAmount, setBaseAmount] = useState("49");
  const [selectedMetricIDs, setSelectedMetricIDs] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const metricsQuery = useQuery({ queryKey: ["pricing-metrics", apiBaseURL], queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }), enabled: isAuthenticated && scope === "tenant" });

  const mutation = useMutation({
    mutationFn: () => createPlan({ runtimeBaseURL: apiBaseURL, csrfToken, body: { name, code, description, currency, billing_interval: billingInterval, status, base_amount_cents: Math.round(Number(baseAmount || 0) * 100), meter_ids: selectedMetricIDs } }),
    onSuccess: (plan) => router.push(`/pricing/plans/${encodeURIComponent(plan.id)}`),
    onError: (err: Error) => setError(err.message),
  });

  const toggleMetric = (metricID: string) => {
    setSelectedMetricIDs((current) => current.includes(metricID) ? current.filter((id) => id !== metricID) : [...current, metricID]);
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55"><div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" /><div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" /></div>
      <main className="relative mx-auto flex max-w-[980px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: "New plan" }]} />
        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Plans are tenant-scoped. Sign in with a tenant account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}
        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"><p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Pricing</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">Create plan</h1><p className="mt-3 text-sm text-slate-300">Keep the first version opinionated: one base price, one billing cadence, and explicit linked metrics.</p><div className="mt-6 grid gap-4 md:grid-cols-2"><Field label="Plan name" value={name} onChange={setName} placeholder="Growth" testID="pricing-plan-name" /><Field label="Plan code" value={code} onChange={setCode} placeholder="growth" testID="pricing-plan-code" /><Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-plan-currency" /><Field label="Base price" value={baseAmount} onChange={setBaseAmount} placeholder="49" testID="pricing-plan-base-price" /><div><label className="text-xs uppercase tracking-[0.16em] text-slate-400">Billing interval</label><select value={billingInterval} onChange={(event) => setBillingInterval(event.target.value)} className="mt-2 h-12 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 text-sm text-white outline-none ring-cyan-400 transition focus:ring-2"><option value="monthly">Monthly</option><option value="yearly">Yearly</option></select></div><div><label className="text-xs uppercase tracking-[0.16em] text-slate-400">Status</label><select value={status} onChange={(event) => setStatus(event.target.value)} className="mt-2 h-12 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 text-sm text-white outline-none ring-cyan-400 transition focus:ring-2"><option value="draft">Draft</option><option value="active">Active</option><option value="archived">Archived</option></select></div><div className="md:col-span-2"><label className="text-xs uppercase tracking-[0.16em] text-slate-400">Description</label><textarea data-testid="pricing-plan-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Best for teams moving from pilot to growth." className="mt-2 min-h-[120px] w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 py-3 text-sm text-white outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2" /></div></div><section className="mt-6 rounded-2xl border border-white/10 bg-slate-950/45 p-4"><p className="text-xs uppercase tracking-[0.16em] text-slate-400">Linked metrics</p><div className="mt-3 grid gap-3">{metricsQuery.isLoading ? <div className="flex items-center gap-2 text-sm text-slate-300"><LoaderCircle className="h-4 w-4 animate-spin" />Loading metrics</div> : (metricsQuery.data ?? []).length === 0 ? <p className="text-sm text-slate-400">Create at least one metric before creating a plan.</p> : metricsQuery.data?.map((metric) => <label key={metric.id} className="flex items-center gap-3 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-slate-200"><input data-testid={`pricing-plan-metric-${metric.id}`} type="checkbox" checked={selectedMetricIDs.includes(metric.id)} onChange={() => toggleMetric(metric.id)} className="h-4 w-4 rounded border-white/20 bg-slate-950/70" /><span className="font-semibold text-white">{metric.name}</span><span className="font-mono text-xs text-slate-400">{metric.key}</span><span className="text-xs uppercase tracking-[0.14em] text-slate-400">{metric.aggregation}</span></label>)}</div></section>{error ? <p className="mt-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-100">{error}</p> : null}<div className="mt-6 flex flex-wrap gap-3"><button data-testid="pricing-plan-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending || selectedMetricIDs.length === 0} className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50">{mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}Create plan</button></div></section>
      </main>
    </div>
  );
}

function Field({ label, value, onChange, placeholder, testID }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; testID: string }) { return <div><label className="text-xs uppercase tracking-[0.16em] text-slate-400">{label}</label><input data-testid={testID} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="mt-2 h-12 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 text-sm text-white outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2" /></div>; }
