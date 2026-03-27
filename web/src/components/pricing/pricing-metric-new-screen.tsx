"use client";

import Link from "next/link";
import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1120px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Metrics are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create metric</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Define the usage record plans will price against. Keep the key stable and the aggregation obvious.</p>
        </section>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_280px]">
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="grid gap-5">
              <div className="grid gap-3 lg:grid-cols-3">
                <OperatorCard title="Operator input" body="Use a stable code and a unit that operators will still recognize six months from now." />
                <OperatorCard title="Aggregation rule" body="Sum is the safest default for most metered event flows. Only use count or max when the billing rule is explicit." />
                <OperatorCard title="After create" body="Open the metric record to inspect the generated pricing draft and attach it to plans." />
              </div>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Usage definition</p>
                <h2 className="mt-2 text-lg font-semibold text-slate-950">Metric basics</h2>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <Field label="Metric name" value={name} onChange={setName} placeholder="API Calls" testID="pricing-metric-name" />
                  <Field label="Metric code" value={key} onChange={setKey} placeholder="api_calls" testID="pricing-metric-code" />
                  <Field label="Unit" value={unit} onChange={setUnit} placeholder="request" testID="pricing-metric-unit" />
                  <SelectField label="Aggregation" value={aggregation} onChange={setAggregation} options={["sum", "count", "max"]} />
                  <Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-metric-currency" />
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                <div className="mt-3 grid gap-2 md:grid-cols-2">
                  <ChecklistLine done={name.trim().length > 0} text="Metric name is set" />
                  <ChecklistLine done={key.trim().length > 0} text="Metric code is set" />
                  <ChecklistLine done={unit.trim().length > 0} text="Usage unit is set" />
                  <ChecklistLine done={Boolean(csrfToken)} text="Writable workspace session present" />
                </div>
              </section>

              {error ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

              <div className="flex flex-wrap gap-3">
                <button data-testid="pricing-metric-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create metric
                </button>
                <Link href="/pricing/metrics" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
              </div>
            </div>
          </section>

          <aside className="grid gap-5 self-start">
            <InfoCard title="Design rule" body="Keep metrics simple and stable so plans can reuse them safely later." />
            <InfoCard title="Operator guidance" body="This screen defines the usage record only. Use metric detail after submit to inspect the generated pricing draft." />
          </aside>
        </div>
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function Field({ label, value, onChange, placeholder, testID }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; testID: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input data-testid={testID} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" />
    </label>
  );
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2">
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
    </label>
  );
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-white px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}
