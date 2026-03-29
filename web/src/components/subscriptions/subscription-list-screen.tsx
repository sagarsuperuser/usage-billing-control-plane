"use client";

import Link from "next/link";
import { LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchSubscriptions } from "@/lib/api";
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
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");

  const subscriptionsQuery = useQuery({
    queryKey: ["subscriptions", apiBaseURL],
    queryFn: () => fetchSubscriptions({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = subscriptionsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) =>
      [item.display_name, item.code, item.customer_display_name, item.customer_external_id, item.plan_name, item.plan_code].some((value) => value.toLowerCase().includes(term)),
    );
  }, [subscriptionsQuery.data, search]);

  const stats = {
    total: subscriptionsQuery.data?.length ?? 0,
    active: (subscriptionsQuery.data ?? []).filter((item) => item.status === "active").length,
    pending: (subscriptionsQuery.data ?? []).filter((item) => item.status === "pending_payment_setup").length,
    actionRequired: (subscriptionsQuery.data ?? []).filter((item) => item.status === "action_required").length,
  };

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Workspace" }, { label: "Subscriptions" }]} />

        {isTenantSession ? <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Subscriptions</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Customer subscriptions</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Track what the customer is signing up for, whether payment setup has been requested, and whether the payer has completed billing readiness.
              </p>
            </div>
            {isTenantSession ? (
              <Link href="/subscriptions/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                <Plus className="h-4 w-4" />
                New subscription
              </Link>
            ) : null}
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Subscriptions are workspace-scoped. Sign in with a workspace account to create and track customer subscriptions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <>
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="Subscriptions" value={stats.total} />
              <MetricCard label="Active" value={stats.active} />
              <MetricCard label="Pending setup" value={stats.pending} />
              <MetricCard label="Action required" value={stats.actionRequired} />
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Subscription inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by customer, plan, or code"
              className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
              <div className="mt-5 grid gap-3">
                {subscriptionsQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((item) => <SubscriptionRow key={item.id} item={item} />)}
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}

function SubscriptionRow({ item }: { item: SubscriptionSummary }) {
  return (
    <Link
      href={`/subscriptions/${encodeURIComponent(item.id)}`}
      className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-white lg:grid-cols-[minmax(0,1.5fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,1fr)_110px] lg:items-start"
    >
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{item.display_name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{item.code}</p>
        <p className="mt-2 text-sm text-slate-600">{item.customer_display_name} on {item.plan_name}</p>
      </div>
      <InventoryCell label="Lifecycle" value={item.status} />
      <InventoryCell label="Payment setup" value={formatSubscriptionPaymentSetupStatus(item.payment_setup_status)} />
      <InventoryCell label="Billing" value={`${item.billing_interval} · ${(item.base_amount_cents / 100).toFixed(2)} ${item.currency}`} />
      <div className="flex items-center justify-between gap-3 lg:justify-end">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">Open</p>
      </div>
    </Link>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InventoryCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 break-all text-sm font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading subscriptions
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No subscriptions yet.</p>
      <p className="mt-2">Create the first subscription after you have at least one customer and one plan.</p>
      <div className="mt-4">
        <Link href="/subscriptions/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">
          Create subscription
        </Link>
      </div>
    </div>
  );
}
