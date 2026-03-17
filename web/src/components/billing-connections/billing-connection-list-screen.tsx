"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchBillingProviderConnections } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { type BillingProviderConnection } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function statusTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "connected":
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "sync_error":
      return "border-rose-400/40 bg-rose-500/10 text-rose-100";
    case "pending":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
  }
}

export function BillingConnectionListScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
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
      [item.display_name, item.id, item.lago_organization_id, item.lago_provider_code]
        .filter(Boolean)
        .some((value) => value?.toLowerCase().includes(term))
    );
  }, [connectionsQuery.data, search]);

  const summary = useMemo(() => ({
    total: filteredConnections.length,
    connected: filteredConnections.filter((item) => item.status === "connected").length,
    syncErrors: filteredConnections.filter((item) => item.status === "sync_error").length,
    disabled: filteredConnections.filter((item) => item.status === "disabled").length,
  }), [filteredConnections]);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-fuchsia-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Platform Directory</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Billing Connections</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Own Stripe secret storage and Lago provider sync in Alpha. Workspaces link these records instead of carrying raw provider mappings directly.
              </p>
            </div>
            <Link
              href="/billing-connections/new"
              className="inline-flex h-11 items-center gap-2 rounded-xl border border-fuchsia-400/40 bg-fuchsia-500/10 px-4 text-sm font-medium text-fuchsia-100 transition hover:bg-fuchsia-500/20"
            >
              <Plus className="h-4 w-4" />
              New billing connection
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are owned at the platform layer. Sign in with a platform_admin API key to manage them."
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Visible connections" value={summary.total} />
          <MetricCard label="Connected" value={summary.connected} tone="success" />
          <MetricCard label="Sync errors" value={summary.syncErrors} tone="warn" />
          <MetricCard label="Disabled" value={summary.disabled} />
        </section>

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Connection list</p>
              <h2 className="mt-2 text-2xl font-semibold text-white">Browse and inspect</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by name, org, provider code, or ID"
                className="h-11 min-w-[280px] rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="connected">Connected</option>
                <option value="pending">Pending</option>
                <option value="sync_error">Sync error</option>
                <option value="disabled">Disabled</option>
              </select>
            </div>
          </div>

          <div className="mt-5 grid gap-3">
            {connectionsQuery.isLoading ? (
              <LoadingState />
            ) : filteredConnections.length === 0 ? (
              <EmptyState />
            ) : (
              filteredConnections.map((connection) => <ConnectionRow key={connection.id} connection={connection} />)
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

function ConnectionRow({ connection }: { connection: BillingProviderConnection }) {
  return (
    <Link
      href={`/billing-connections/${encodeURIComponent(connection.id)}`}
      className="grid gap-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 transition hover:border-fuchsia-400/40 hover:bg-fuchsia-500/5 lg:grid-cols-[minmax(0,1.1fr)_repeat(3,minmax(0,0.55fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="truncate text-lg font-semibold text-white">{connection.display_name}</h3>
          <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${statusTone(connection.status)}`}>
            {connection.status}
          </span>
        </div>
        <p className="mt-1 break-all font-mono text-xs text-slate-400">{connection.id}</p>
        <p className="mt-2 text-sm text-slate-300">
          {connection.last_sync_error ? connection.last_sync_error : `Last updated ${formatExactTimestamp(connection.updated_at)}`}
        </p>
      </div>
      <StatusCell label="Environment" value={connection.environment} />
      <StatusCell label="Billing org" value={connection.lago_organization_id || "-"} mono />
      <StatusCell label="Provider code" value={connection.lago_provider_code || "-"} mono />
      <StatusCell label="Scope" value={connection.scope} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-fuchsia-100">
        Open
        <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function StatusCell({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className={`mt-2 text-sm font-semibold text-white ${mono ? "break-all font-mono text-xs" : ""}`}>{value}</p>
    </div>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-rose-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-6 text-sm text-slate-300">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading billing connections
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-2xl border border-dashed border-white/10 px-4 py-8 text-sm text-slate-400">
      No billing connections match the current filters.
    </div>
  );
}
