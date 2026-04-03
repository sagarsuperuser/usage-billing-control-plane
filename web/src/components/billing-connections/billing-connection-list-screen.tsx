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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1180px] flex-col gap-5 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Billing connections" }]} />

        {canViewPlatformSurface ? (
          <div className="overflow-hidden rounded-xl border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-slate-900">Billing connections</h1>
                <p className="mt-0.5 text-xs text-slate-500">Platform-owned Stripe connections. Verify before assigning to a workspace.</p>
              </div>
              <div className="flex items-center gap-2">
                <Link
                  href="/workspaces"
                  className="inline-flex h-8 items-center rounded-lg border border-stone-200 px-3 text-sm text-slate-600 transition hover:bg-stone-100"
                >
                  Workspaces
                </Link>
                <Link
                  href="/billing-connections/new"
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800"
                >
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            <div className="grid gap-px border-b border-stone-200 bg-stone-200 sm:grid-cols-5">
              <MetricCell label="Total" value={summary.total} />
              <MetricCell label="Connected" value={summary.connected} tone="success" />
              <MetricCell label="Needs attention" value={summary.syncErrors} tone="danger" />
              <MetricCell label="Disabled" value={summary.disabled} />
              <MetricCell label="Linked workspaces" value={summary.linkedWorkspaces} />
            </div>
          </div>
        ) : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are platform-owned. Sign in with a platform account to manage them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {canViewPlatformSurface ? (
          <div className="overflow-hidden rounded-xl border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-3">
              <div className="flex gap-2">
                <input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search by name, provider, or environment"
                  className="h-8 min-w-[260px] rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="connected">Connected</option>
                  <option value="pending">Pending</option>
                  <option value="sync_error">Needs attention</option>
                  <option value="disabled">Disabled</option>
                </select>
              </div>
            </div>

            <div className="divide-y divide-stone-100">
              {connectionsQuery.isLoading ? (
                <LoadingState />
              ) : filteredConnections.length === 0 ? (
                <EmptyState />
              ) : (
                filteredConnections.map((connection) => <ConnectionRow key={connection.id} connection={connection} />)
              )}
            </div>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function ConnectionRow({ connection }: { connection: BillingProviderConnection }) {
  return (
    <Link
      href={`/billing-connections/${encodeURIComponent(connection.id)}`}
      className="flex items-center gap-4 px-6 py-4 transition hover:bg-stone-50"
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-slate-900">{connection.display_name}</p>
          <span className={`rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(connection.status)}`}>
            {connection.status}
          </span>
        </div>
        <p className="mt-0.5 truncate text-xs text-slate-500">{connection.sync_summary}</p>
      </div>
      <div className="hidden shrink-0 items-center gap-6 text-right sm:flex">
        <StatCell label="Provider" value={connection.provider_type} />
        <StatCell label="Environment" value={connection.environment} />
        <StatCell label="Workspaces" value={String(connection.linked_workspace_count)} />
        <StatCell label="Assignment" value={connection.workspace_ready ? "Ready" : "Blocked"} />
        <StatCell label="Last checked" value={connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "—"} />
      </div>
    </Link>
  );
}

function StatCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="text-right">
      <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-slate-400">{label}</p>
      <p className="mt-0.5 text-xs font-medium text-slate-700">{value}</p>
    </div>
  );
}

function MetricCell({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "success" | "danger";
}) {
  const toneClass = tone === "success" ? "text-emerald-700" : tone === "danger" ? "text-rose-700" : "text-slate-900";
  return (
    <div className="bg-white px-5 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-400">{label}</p>
      <p className={`mt-1 text-xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <>
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-6 py-4">
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <Skeleton className="h-4 w-36" />
              <Skeleton className="h-4 w-16 rounded-full" />
            </div>
            <Skeleton className="mt-1.5 h-3 w-64" />
          </div>
          <div className="hidden shrink-0 items-center gap-6 sm:flex">
            {Array.from({ length: 5 }).map((__, j) => (
              <div key={j} className="text-right">
                <Skeleton className="h-2.5 w-12" />
                <Skeleton className="mt-1 h-3 w-16" />
              </div>
            ))}
          </div>
        </div>
      ))}
    </>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No billing connections</p>
      <p className="text-xs text-slate-500">Create a connection before assigning billing to a workspace.</p>
      <Link href="/billing-connections/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New billing connection
      </Link>
    </div>
  );
}
