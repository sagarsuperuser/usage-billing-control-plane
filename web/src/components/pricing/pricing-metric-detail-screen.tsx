"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPricingMetric } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingMetricDetailScreen({ metricID }: { metricID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const query = useQuery({
    queryKey: ["pricing-metric", apiBaseURL, metricID],
    queryFn: () => fetchPricingMetric({ runtimeBaseURL: apiBaseURL, metricID }),
    enabled: isTenantSession && metricID.trim().length > 0,
  });

  const metric = query.data ?? null;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: metric?.name || metricID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Workspace session required" body="Metrics are workspace-scoped. Sign in with a workspace account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        {isTenantSession ? query.isLoading ? (
          <LoadingPanel label="Loading metric detail" />
        ) : !metric ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing metric</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Metric not available</h1>
            <Link href="/pricing/metrics" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
              <ArrowLeft className="h-4 w-4" />
              Back to metrics
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing metric</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{metric.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{metric.key}</p>
                </div>
                <Link href="/pricing/metrics" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                  <ArrowLeft className="h-4 w-4" />
                  Back to metrics
                </Link>
              </div>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <div className="grid gap-5">
                <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <Stat label="Unit" value={metric.unit} />
                  <Stat label="Aggregation" value={metric.aggregation} />
                  <Stat label="Created" value={new Date(metric.created_at).toLocaleString()} />
                  <Stat label="Updated" value={new Date(metric.updated_at).toLocaleString()} />
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial record</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Usage definition</h2>
                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    <InfoCell label="Metric name" value={metric.name} />
                    <InfoCell label="Metric code" value={metric.key} mono />
                    <InfoCell label="Usage unit" value={metric.unit} />
                    <InfoCell label="Commercial aggregation" value={metric.aggregation} />
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <GuidanceCard title="Operator posture" body="Use this record when plans need a stable usage input. Keep the key and aggregation unchanged once commercial reporting depends on them." />
                <GuidanceCard title="Next action" body="Attach the metric to plans that price this usage record. Metric detail is for inspection, not day-to-day operator changes." />
              </aside>
            </div>
          </>
        ) : null}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InfoCell({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 text-sm font-semibold text-slate-950 ${mono ? "break-all font-mono" : ""}`}>{value}</p>
    </div>
  );
}

function GuidanceCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}
