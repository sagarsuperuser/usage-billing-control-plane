"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { createPricingMetric } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingMetricNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [unit, setUnit] = useState("request");
  const [aggregation, setAggregation] = useState("sum");
  const [currency, setCurrency] = useState("USD");
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () => createPricingMetric({ runtimeBaseURL: apiBaseURL, csrfToken, body: { name, key, unit, aggregation, currency } }),
    onSuccess: (metric) => router.push(`/pricing/metrics/${encodeURIComponent(metric.id)}`),
    onError: (err: Error) => setError(err.message),
  });

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55"><div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" /><div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" /></div>
      <main className="relative mx-auto flex max-w-[960px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: "New metric" }]} />
        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Metrics are tenant-scoped. Sign in with a tenant account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}
        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Pricing</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">Create metric</h1>
          <p className="mt-3 text-sm text-slate-300">Define what gets measured. Alpha will create the supporting pricing draft behind the scenes so this stays simple.</p>
          <div className="mt-6 grid gap-4 md:grid-cols-2">
            <Field label="Metric name" value={name} onChange={setName} placeholder="API Calls" testID="pricing-metric-name" />
            <Field label="Metric code" value={key} onChange={setKey} placeholder="api_calls" testID="pricing-metric-code" />
            <Field label="Unit" value={unit} onChange={setUnit} placeholder="request" testID="pricing-metric-unit" />
            <div>
              <label className="text-xs uppercase tracking-[0.16em] text-slate-400">Aggregation</label>
              <select value={aggregation} onChange={(event) => setAggregation(event.target.value)} className="mt-2 h-12 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 text-sm text-white outline-none ring-cyan-400 transition focus:ring-2">
                <option value="sum">Sum</option>
                <option value="count">Count</option>
                <option value="max">Max</option>
              </select>
            </div>
            <Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-metric-currency" />
          </div>
          {error ? <p className="mt-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-100">{error}</p> : null}
          <div className="mt-6 flex flex-wrap gap-3">
            <button data-testid="pricing-metric-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending} className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50">
              {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
              Create metric
            </button>
          </div>
        </section>
      </main>
    </div>
  );
}

function Field({ label, value, onChange, placeholder, testID }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; testID: string }) {
  return <div><label className="text-xs uppercase tracking-[0.16em] text-slate-400">{label}</label><input data-testid={testID} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="mt-2 h-12 w-full rounded-2xl border border-white/10 bg-slate-950/60 px-4 text-sm text-white outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2" /></div>;
}
