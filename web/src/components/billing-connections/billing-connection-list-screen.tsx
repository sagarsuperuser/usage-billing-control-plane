"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchBillingProviderConnections } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { type BillingProviderConnection } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function statusTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "connected":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "sync_error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "disabled":
      return "border-stone-300 bg-stone-100 text-slate-700";
    default:
      return "border-stone-300 bg-stone-100 text-slate-700";
  }
}

export function BillingConnectionListScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");

  const connectionsQuery = useQuery({
    queryKey: ["billing-provider-connections", apiBaseURL, statusFilter],
    queryFn: () =>
      fetchBillingProviderConnections({
        runtimeBaseURL: apiBaseURL,
        status: statusFilter || undefined,
        limit: 100,
      }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const filteredConnections = useMemo(() => {
    const items = connectionsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) =>
      [item.display_name, item.provider_type, item.environment]
        .filter(Boolean)
        .some((value) => value?.toLowerCase().includes(term))
    );
  }, [connectionsQuery.data, search]);

  const summary = useMemo(
    () => ({
      total: filteredConnections.length,
      connected: filteredConnections.filter((item) => item.status === "connected").length,
      syncErrors: filteredConnections.filter((item) => item.status === "sync_error").length,
      disabled: filteredConnections.filter((item) => item.status === "disabled").length,
      linkedWorkspaces: filteredConnections.reduce((sum, item) => sum + item.linked_workspace_count, 0),
    }),
    [filteredConnections]
  );

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#eef4ef_0%,#f6f2eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Billing Connections" }]} />

        {canViewPlatformSurface ? <section className="rounded-3xl border border-stone-200 bg-white/92 shadow-[0_18px_50px_rgba(15,23,42,0.06)]">
          <div className="flex flex-col gap-5 p-5 lg:flex-row lg:items-start lg:justify-between lg:p-6">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-500">Billing Connections</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Billing connections</h1>
              <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
                Manage Stripe connections here. Verify the connection first, then assign it to a workspace.
              </p>
              <div className="mt-5 grid gap-3 lg:grid-cols-3">
                <OperatorLine title="Inventory rule" body="Use this list as the platform credential inventory. Open detail only when the row shows a real provider or assignment issue." />
                <OperatorLine title="Verification rule" body="Separate provider verification from workspace provisioning. A healthy Stripe check is not the same as a completed workspace handoff." />
                <OperatorLine title="Assignment rule" body="Treat linked workspaces as dependency count. The more workspaces attached, the stricter the change control for that connection." />
              </div>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/workspaces"
                className="inline-flex items-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm font-medium text-slate-700 transition hover:bg-white"
              >
                Open workspaces
              </Link>
              <Link
                href="/billing-connections/new"
                className="inline-flex items-center gap-2 rounded-xl bg-emerald-700 px-4 py-3 text-sm font-semibold text-white transition hover:bg-emerald-800"
              >
                <Plus className="h-4 w-4" />
                New billing connection
              </Link>
            </div>
          </div>
          <div className="grid gap-3 border-t border-stone-200 px-5 py-4 sm:grid-cols-2 xl:grid-cols-5 lg:px-6">
            <MetricCard label="Visible" value={summary.total} />
            <MetricCard label="Connected" value={summary.connected} tone="success" />
            <MetricCard label="Needs attention" value={summary.syncErrors} tone="danger" />
            <MetricCard label="Disabled" value={summary.disabled} />
            <MetricCard label="Linked workspaces" value={summary.linkedWorkspaces} />
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are platform-owned. Sign in with a platform account to manage them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {canViewPlatformSurface ? <section className="rounded-3xl border border-stone-200 bg-white/92 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.06)] lg:p-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Directory</p>
              <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Connection inventory</h2>
              <p className="mt-2 text-sm text-slate-600">Read provider health, assignment readiness, and dependency count from the row before opening detail.</p>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by name, provider, or environment"
                className="h-11 min-w-[280px] rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition placeholder:text-slate-500 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-11 rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="connected">Connected</option>
                <option value="pending">Pending</option>
                <option value="sync_error">Needs attention</option>
                <option value="disabled">Disabled</option>
              </select>
            </div>
          </div>

          <div className="mt-5 divide-y divide-stone-200">
            {connectionsQuery.isLoading ? (
              <LoadingState />
            ) : filteredConnections.length === 0 ? (
              <EmptyState />
            ) : (
              filteredConnections.map((connection) => <ConnectionRow key={connection.id} connection={connection} />)
            )}
          </div>
        </section> : null}
      </main>
    </div>
  );
}

function ConnectionRow({ connection }: { connection: BillingProviderConnection }) {
  return (
    <Link
      href={`/billing-connections/${encodeURIComponent(connection.id)}`}
      className="grid gap-4 py-4 first:pt-0 last:pb-0 lg:grid-cols-[minmax(0,1.3fr)_repeat(4,minmax(0,0.5fr))] lg:items-start"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-base font-semibold text-slate-950">{connection.display_name}</p>
          <span className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] ${statusTone(connection.status)}`}>
            {connection.status}
          </span>
        </div>
        <p className="mt-2 text-sm leading-6 text-slate-600">{connection.sync_summary}</p>
      </div>
      <StatCell label="Provider" value={connection.provider_type} />
      <StatCell label="Environment" value={connection.environment} />
      <StatCell label="Linked workspaces" value={String(connection.linked_workspace_count)} />
      <StatCell label="Assignment" value={connection.workspace_ready ? "Ready" : "Blocked"} />
      <StatCell label="Last checked" value={connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "Not checked"} />
    </Link>
  );
}

function StatCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-900">{value}</p>
    </div>
  );
}

function MetricCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "success" | "danger";
}) {
  const toneClass = tone === "success" ? "text-emerald-700" : tone === "danger" ? "text-rose-700" : "text-slate-950";
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 text-3xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
    </div>
  );
}

function OperatorLine({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="divide-y divide-stone-200">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="grid gap-4 py-4 first:pt-0 last:pb-0 lg:grid-cols-[minmax(0,1.3fr)_repeat(4,minmax(0,0.5fr))] lg:items-start">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <Skeleton className="h-5 w-40" />
              <Skeleton className="h-5 w-20 rounded-full" />
            </div>
            <Skeleton className="mt-2 h-4 w-64" />
          </div>
          <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
            <Skeleton className="h-3 w-14" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
            <Skeleton className="h-3 w-24" />
            <Skeleton className="mt-2 h-4 w-8" />
          </div>
          <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="mt-2 h-4 w-24" />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="py-6 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No billing connections match the current filters.</p>
      <p className="mt-2">Create a billing connection before assigning billing to a workspace.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/billing-connections/new" className="inline-flex h-10 items-center rounded-xl bg-emerald-700 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-emerald-800">Create billing connection</Link>
      </div>
    </div>
  );
}
