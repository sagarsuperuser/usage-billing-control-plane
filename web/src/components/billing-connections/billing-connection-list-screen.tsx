"use client";

import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";
import { ArrowRight, CreditCard, LoaderCircle, Plus, Workflow } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchBillingProviderConnections } from "@/lib/api";
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
      return "border-slate-200 bg-slate-100 text-slate-700";
    default:
      return "border-stone-200 bg-stone-100 text-slate-700";
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
      [item.display_name, item.id, item.provider_type, item.environment]
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
    <div className="min-h-screen bg-[linear-gradient(180deg,#dfeee3_0%,#f4ede3_18%,#f7f3eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Billing Connections" }]} />

        <section className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700/70">Platform directory</p>
            <h1 className="mt-3 text-4xl font-semibold tracking-tight text-slate-950 md:text-5xl">Billing Connections</h1>
            <p className="mt-4 max-w-3xl text-base leading-7 text-slate-600">
              Treat billing credentials like reusable platform assets. Alpha owns the secret, sync state, and operational visibility. Workspaces only attach to the resulting connection.
            </p>
            <div className="mt-6 grid gap-3 md:grid-cols-4">
              <MetricCard label="Visible" value={summary.total} />
              <MetricCard label="Connected" value={summary.connected} tone="success" />
              <MetricCard label="Sync errors" value={summary.syncErrors} tone="warn" />
              <MetricCard label="Linked workspaces" value={summary.linkedWorkspaces} />
            </div>
          </div>

          <div className="rounded-[32px] border border-emerald-900/10 bg-[linear-gradient(160deg,#0f3b2f_0%,#1a5a44_100%)] p-6 text-emerald-50 shadow-[0_25px_70px_rgba(15,23,42,0.12)]">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-100/70">Operator pattern</p>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight">Create once. Reuse deliberately.</h2>
            <p className="mt-4 text-sm leading-7 text-emerald-50/80">
              A billing connection is not the workspace itself. It is the reusable provider credential and sync record that workspace billing attaches to.
            </p>
            <div className="mt-6 flex flex-wrap gap-3">
              <Link
                href="/billing-connections/new"
                className="inline-flex items-center gap-2 rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-emerald-900 transition hover:bg-emerald-50"
              >
                <Plus className="h-4 w-4" />
                New billing connection
              </Link>
              <Link
                href="/workspaces"
                className="inline-flex items-center gap-2 rounded-2xl border border-white/12 bg-white/8 px-4 py-3 text-sm font-semibold text-emerald-50 transition hover:bg-white/12"
              >
                Open workspaces
              </Link>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are owned at the platform layer. Sign in with a platform account to manage them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        <section className="grid gap-4 xl:grid-cols-[0.72fr_1.28fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Operational guide</p>
            <h2 className="mt-3 text-2xl font-semibold tracking-tight text-slate-950">What this surface should answer</h2>
            <div className="mt-5 grid gap-3">
              <GuideCard
                icon={<CreditCard className="h-5 w-5 text-emerald-700" />}
                title="Which credentials are safe to reuse?"
                body="Use connected status and sync summary as the first filter before attaching a connection to more workspaces."
              />
              <GuideCard
                icon={<Workflow className="h-5 w-5 text-emerald-700" />}
                title="Where are we carrying operational risk?"
                body="Sync errors and disabled connections should read like issues, not like hidden backend metadata."
              />
            </div>
          </div>

          <section className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Connection catalogue</p>
                <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Browse by health and reuse</h2>
              </div>
              <div className="flex flex-col gap-3 sm:flex-row">
                <input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search by name, provider, environment, or ID"
                  className="h-11 min-w-[280px] rounded-2xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition placeholder:text-slate-500 focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  className="h-11 rounded-2xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="connected">Connected</option>
                  <option value="pending">Pending</option>
                  <option value="sync_error">Sync error</option>
                  <option value="disabled">Disabled</option>
                </select>
              </div>
            </div>

            <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {connectionsQuery.isLoading ? (
                <LoadingState />
              ) : filteredConnections.length === 0 ? (
                <EmptyState />
              ) : (
                filteredConnections.map((connection) => <ConnectionCard key={connection.id} connection={connection} />)
              )}
            </div>
          </section>
        </section>
      </main>
    </div>
  );
}

function ConnectionCard({ connection }: { connection: BillingProviderConnection }) {
  return (
    <Link
      href={`/billing-connections/${encodeURIComponent(connection.id)}`}
      className="rounded-[28px] border border-stone-200 bg-[#fbfaf6] p-5 transition hover:-translate-y-0.5 hover:border-emerald-200 hover:bg-white"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">{connection.provider_type}</p>
          <h3 className="mt-2 text-xl font-semibold tracking-tight text-slate-950">{connection.display_name}</h3>
        </div>
        <span className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${statusTone(connection.status)}`}>
          {connection.status}
        </span>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <DetailPill label="Environment" value={connection.environment} />
        <DetailPill label="Scope" value={connection.scope} />
        <DetailPill label="Linked workspaces" value={String(connection.linked_workspace_count)} />
        <DetailPill label="Workspace ready" value={connection.workspace_ready ? "Yes" : "No"} />
      </div>

      <p className="mt-4 line-clamp-3 text-sm leading-6 text-slate-600">{connection.sync_summary}</p>
      <p className="mt-4 break-all font-mono text-[11px] text-slate-500">{connection.id}</p>

      <div className="mt-5 inline-flex items-center gap-2 text-sm font-semibold text-emerald-700">
        Open connection
        <ArrowRight className="h-4 w-4" />
      </div>
    </Link>
  );
}

function DetailPill({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-3 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-900">{value}</p>
    </div>
  );
}

function GuideCard({ icon, title, body }: { icon: ReactNode; title: string; body: string }) {
  return (
    <div className="rounded-[24px] border border-stone-200 bg-stone-50/85 p-4">
      <span className="inline-flex rounded-2xl border border-emerald-200 bg-emerald-50 p-3">{icon}</span>
      <h3 className="mt-4 text-lg font-semibold tracking-tight text-slate-950">{title}</h3>
      <p className="mt-2 text-sm leading-6 text-slate-600">{body}</p>
    </div>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" }) {
  const toneClass = tone === "success" ? "text-emerald-700" : tone === "warn" ? "text-rose-700" : "text-slate-950";
  return (
    <div className="rounded-[24px] border border-stone-200 bg-stone-50/85 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className={`mt-2 text-3xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="md:col-span-2 xl:col-span-3 flex items-center gap-2 rounded-[24px] border border-stone-200 bg-stone-50/85 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading billing connections
    </div>
  );
}

function EmptyState() {
  return (
    <div className="md:col-span-2 xl:col-span-3 rounded-[24px] border border-dashed border-stone-300 bg-stone-50/70 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No billing connections match the current filters.</p>
      <p className="mt-2">Create a Stripe connection in Alpha before assigning billing to new workspaces.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/billing-connections/new" className="inline-flex h-10 items-center rounded-2xl bg-emerald-700 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-emerald-800">Create billing connection</Link>
      </div>
    </div>
  );
}
