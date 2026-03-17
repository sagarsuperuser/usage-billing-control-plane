"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchSubscriptions } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { type SubscriptionSummary } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function formatSubscriptionPaymentSetupStatus(status: SubscriptionSummary["payment_setup_status"]): string {
  switch (status) {
    case "missing":
      return "Not requested";
    case "pending":
      return "Pending";
    case "ready":
      return "Ready";
    case "error":
      return "Action required";
    default:
      return status;
  }
}

export function SubscriptionListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [search, setSearch] = useState("");
  const subscriptionsQuery = useQuery({
    queryKey: ["subscriptions", apiBaseURL],
    queryFn: () => fetchSubscriptions({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filtered = useMemo(() => {
    const items = subscriptionsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) =>
      [item.display_name, item.code, item.customer_display_name, item.customer_external_id, item.plan_name, item.plan_code].some((value) =>
        value.toLowerCase().includes(term)
      )
    );
  }, [subscriptionsQuery.data, search]);

  const stats = {
    total: subscriptionsQuery.data?.length ?? 0,
    active: (subscriptionsQuery.data ?? []).filter((item) => item.status === "active").length,
    pending: (subscriptionsQuery.data ?? []).filter((item) => item.status === "pending_payment_setup").length,
    actionRequired: (subscriptionsQuery.data ?? []).filter((item) => item.status === "action_required").length,
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Tenant" }, { label: "Subscriptions" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Subscriptions</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Customer subscriptions</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Track what the customer is signing up for, whether payment setup has been requested, and whether the payer has completed billing readiness.
              </p>
            </div>
            <Link href="/subscriptions/new" className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20">
              <Plus className="h-4 w-4" />
              New subscription
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Subscriptions are tenant-scoped. Sign in with a tenant account to create and track customer subscriptions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Subscriptions" value={stats.total} />
          <MetricCard label="Active" value={stats.active} tone="success" />
          <MetricCard label="Pending setup" value={stats.pending} tone="warn" />
          <MetricCard label="Action required" value={stats.actionRequired} tone="danger" />
        </section>

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Subscription inventory</p>
              <h2 className="mt-2 text-2xl font-semibold text-white">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by customer, plan, or code"
              className="h-11 min-w-[260px] rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
            />
          </div>
          <div className="mt-5 grid gap-3">
            {subscriptionsQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((item) => <SubscriptionRow key={item.id} item={item} />)}
          </div>
        </section>
      </main>
    </div>
  );
}

function SubscriptionRow({ item }: { item: SubscriptionSummary }) {
  return (
    <Link href={`/subscriptions/${encodeURIComponent(item.id)}`} className="grid gap-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 transition hover:border-cyan-400/40 hover:bg-cyan-500/5 lg:grid-cols-[minmax(0,1.2fr)_repeat(4,minmax(0,0.52fr))_auto] lg:items-center">
      <div className="min-w-0">
        <h3 className="truncate text-lg font-semibold text-white">{item.display_name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-400">{item.code}</p>
        <p className="mt-2 text-sm text-slate-300">{item.customer_display_name} on {item.plan_name}</p>
      </div>
      <StatusCell label="Lifecycle" value={item.status} />
      <StatusCell label="Payment setup" value={formatSubscriptionPaymentSetupStatus(item.payment_setup_status)} raw />
      <StatusCell label="Plan" value={item.plan_name} raw />
      <StatusCell label="Billing" value={`${item.billing_interval} · ${(item.base_amount_cents / 100).toFixed(2)} ${item.currency}`} raw />
      <StatusCell label="Customer" value={item.customer_external_id} mono raw />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-cyan-100">
        Open <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" | "danger" }) {
  const toneClass =
    tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-amber-100" : tone === "danger" ? "text-rose-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function StatusCell({ label, value, mono, raw }: { label: string; value: string; mono?: boolean; raw?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className={`mt-2 break-all text-sm font-semibold text-white ${mono ? "font-mono" : ""}`}>{raw ? value : formatReadinessStatus(value)}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-6 text-sm text-slate-300">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading subscriptions
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-2xl border border-dashed border-white/10 bg-slate-950/40 px-5 py-8 text-sm text-slate-300">
      <p className="font-semibold text-white">No subscriptions yet.</p>
      <p className="mt-2 text-slate-400">Create the first subscription after you have at least one customer and one plan.</p>
      <div className="mt-4">
        <Link href="/subscriptions/new" className="inline-flex h-10 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-cyan-100 transition hover:bg-cyan-500/20">
          Create subscription
        </Link>
      </div>
    </div>
  );
}
