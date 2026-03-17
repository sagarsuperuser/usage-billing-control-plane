"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, LoaderCircle, RefreshCcw, ShieldOff } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { disableBillingProviderConnection, fetchBillingProviderConnection, syncBillingProviderConnection } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function readinessTone(status?: string): string {
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

export function BillingConnectionDetailScreen({ connectionID }: { connectionID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();

  const connectionQuery = useQuery({
    queryKey: ["billing-provider-connection", apiBaseURL, connectionID],
    queryFn: () => fetchBillingProviderConnection({ runtimeBaseURL: apiBaseURL, connectionID }),
    enabled: isAuthenticated && isPlatformAdmin && connectionID.trim().length > 0,
  });

  const syncMutation = useMutation({
    mutationFn: () => syncBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }),
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] }),
      ]);
    },
  });

  const disableMutation = useMutation({
    mutationFn: () => disableBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }),
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] }),
      ]);
    },
  });

  const connection = connectionQuery.data ?? null;

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-fuchsia-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1240px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/billing-connections", label: "Billing Connections" }, { label: connection?.display_name || connectionID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are managed at the platform layer. Sign in with a platform account to inspect them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {connectionQuery.isLoading ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 text-sm text-slate-300 backdrop-blur-xl">
            <div className="flex items-center gap-2">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading billing connection detail
            </div>
          </section>
        ) : connectionQuery.isError || !connection ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Billing connection detail</p>
            <h1 className="mt-2 text-3xl font-semibold text-white">Connection not available</h1>
            <p className="mt-3 text-sm text-slate-300">The requested billing connection could not be loaded.</p>
            <Link
              href="/billing-connections"
              className="mt-5 inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to billing connections
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Billing connection detail</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-white md:text-4xl">{connection.display_name}</h1>
                  <p className="mt-2 break-all font-mono text-sm text-slate-400">{connection.id}</p>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <span className={`rounded-full px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] ${readinessTone(connection.status)}`}>
                    {formatReadinessStatus(connection.status)}
                  </span>
                  <Link
                    href="/billing-connections"
                    className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Back to billing connections
                  </Link>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Status" value={formatReadinessStatus(connection.status)} helper={connection.sync_summary} />
              <SummaryStat label="Environment" value={connection.environment} helper={`Provider: ${connection.provider_type}`} />
              <SummaryStat label="Linked workspaces" value={String(connection.linked_workspace_count)} helper={connection.workspace_ready ? "Ready for workspace assignment." : "Sync this connection before assigning it to new workspaces."} />
              <SummaryStat label="Secret" value={connection.secret_configured ? "Configured" : "Missing"} helper="Secret material stays outside the database." />
            </section>

            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Provider sync</p>
                <h2 className="mt-2 text-2xl font-semibold text-white">Connection health</h2>
                <div className="mt-5 grid gap-3 md:grid-cols-2">
                  <MetaItem label="Sync state" value={connection.sync_state.replaceAll("_", " ")} />
                  <MetaItem label="Workspace readiness" value={connection.workspace_ready ? "Ready" : "Needs sync"} />
                  <MetaItem label="Connected at" value={connection.connected_at ? formatExactTimestamp(connection.connected_at) : "-"} />
                  <MetaItem label="Last synced at" value={connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "-"} />
                </div>
                <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 text-sm text-slate-200">{connection.sync_summary}</div>
                <details className="mt-4 rounded-2xl border border-white/10 bg-slate-950/40 p-4">
                  <summary className="cursor-pointer list-none text-sm font-semibold text-white">Advanced provider mapping</summary>
                  <div className="mt-4 grid gap-3 md:grid-cols-2">
                    <MetaItem label="Billing organization reference" value={connection.lago_organization_id || "-"} mono />
                    <MetaItem label="Provider code" value={connection.lago_provider_code || "-"} mono />
                  </div>
                </details>
              </section>

              <aside className="flex flex-col gap-4">
                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Actions</p>
                  <div className="mt-4 grid gap-3">
                    <button
                      type="button"
                      onClick={() => syncMutation.mutate()}
                      disabled={!csrfToken || syncMutation.isPending || disableMutation.isPending || connection.status === "disabled"}
                      className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {syncMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                      Sync now
                    </button>
                    <button
                      type="button"
                      onClick={() => disableMutation.mutate()}
                      disabled={!csrfToken || disableMutation.isPending || syncMutation.isPending || connection.status === "disabled"}
                      className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-rose-400/40 bg-rose-500/10 px-4 text-sm font-medium text-rose-100 transition hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {disableMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldOff className="h-4 w-4" />}
                      Disable connection
                    </button>
                    <Link
                      href="/workspaces/new"
                      className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                    >
                      <CreditCard className="h-4 w-4" />
                      Use in workspace setup
                    </Link>
                  </div>
                </section>

                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Metadata</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Created" value={formatExactTimestamp(connection.created_at)} />
                    <MetaItem label="Updated" value={formatExactTimestamp(connection.updated_at)} />
                    <MetaItem label="Created by" value={connection.created_by_id || connection.created_by_type} />
                  </dl>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="min-w-0 rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{value}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3">
      <dt className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value}</dd>
    </div>
  );
}
