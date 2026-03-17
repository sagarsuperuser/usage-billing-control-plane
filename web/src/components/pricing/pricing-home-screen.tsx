"use client";

import Link from "next/link";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchPlans, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingHomeScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const plansQuery = useQuery({
    queryKey: ["pricing-plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const loading = metricsQuery.isLoading || plansQuery.isLoading;
  const metricCount = metricsQuery.data?.length ?? 0;
  const planCount = plansQuery.data?.length ?? 0;

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Tenant" }, { label: "Pricing" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Pricing</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Pricing foundation</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
            Define what gets measured and how customers are charged without leaving Alpha. Keep the first version simple: metrics first, then plans.
          </p>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Pricing is tenant-scoped. Sign in with a tenant account to define metrics and plans."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Metrics" value={metricCount} />
          <MetricCard label="Plans" value={planCount} />
          <MetricCard label="Draft plans" value={(plansQuery.data ?? []).filter((plan) => plan.status === "draft").length} tone="warn" />
          <MetricCard label="Active plans" value={(plansQuery.data ?? []).filter((plan) => plan.status === "active").length} tone="success" />
        </section>

        {loading ? (
          <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-6 text-sm text-slate-300">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading pricing inventory
          </div>
        ) : (
          <section className="grid gap-6 lg:grid-cols-2">
            <DomainCard
              eyebrow="Metrics"
              title="What gets measured"
              body="Create usage metrics that describe the commercial signal you want to track, such as API calls or seats."
              href="/pricing/metrics"
              cta="Open metrics"
              secondaryHref="/pricing/metrics/new"
              secondaryLabel="New metric"
            />
            <DomainCard
              eyebrow="Plans"
              title="How customers are charged"
              body="Create plans that combine a base price with one or more metrics. Keep the first version opinionated and easy to review."
              href="/pricing/plans"
              cta="Open plans"
              secondaryHref="/pricing/plans/new"
              secondaryLabel="New plan"
            />
          </section>
        )}
      </main>
    </div>
  );
}

function DomainCard({ eyebrow, title, body, href, cta, secondaryHref, secondaryLabel }: { eyebrow: string; title: string; body: string; href: string; cta: string; secondaryHref: string; secondaryLabel: string }) {
  return (
    <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">{eyebrow}</p>
      <h2 className="mt-2 text-2xl font-semibold text-white">{title}</h2>
      <p className="mt-3 text-sm leading-relaxed text-slate-300">{body}</p>
      <div className="mt-5 flex flex-wrap gap-3">
        <Link href={href} className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20">
          {cta}
          <ArrowRight className="h-4 w-4" />
        </Link>
        <Link href={secondaryHref} className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
          {secondaryLabel}
        </Link>
      </div>
    </div>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-amber-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}
