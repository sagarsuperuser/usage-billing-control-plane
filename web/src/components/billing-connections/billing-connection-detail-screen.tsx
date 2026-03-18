"use client";

import Link from "next/link";
import { useState } from "react";
import { ArrowLeft, CreditCard, LoaderCircle, RefreshCcw, ShieldOff } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  disableBillingProviderConnection,
  fetchBillingProviderConnection,
  syncBillingProviderConnection,
  updateBillingProviderConnection,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function readinessTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "connected":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "sync_error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

export function BillingConnectionDetailScreen({ connectionID }: { connectionID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [isEditing, setIsEditing] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [environment, setEnvironment] = useState<"test" | "live">("test");
  const [lagoOrganizationID, setLagoOrganizationID] = useState("");
  const [lagoProviderCode, setLagoProviderCode] = useState("");

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

  const updateMutation = useMutation({
    mutationFn: () =>
      updateBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        connectionID,
        body: {
          display_name: displayName.trim(),
          environment,
          lago_organization_id: lagoOrganizationID.trim() || "",
          lago_provider_code: lagoProviderCode.trim() || "",
        },
      }),
    onSuccess: async () => {
      setIsEditing(false);
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

  const startEditing = () => {
    if (!connection) return;
    setDisplayName(connection.display_name);
    setEnvironment(connection.environment);
    setLagoOrganizationID(connection.lago_organization_id || "");
    setLagoProviderCode(connection.lago_provider_code || "");
    setIsEditing(true);
  };

  const cancelEditing = () => {
    if (connection) {
      setDisplayName(connection.display_name);
      setEnvironment(connection.environment);
      setLagoOrganizationID(connection.lago_organization_id || "");
      setLagoProviderCode(connection.lago_provider_code || "");
    }
    setIsEditing(false);
  };

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
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
          <LoadingPanel label="Loading billing connection detail" />
        ) : connectionQuery.isError || !connection ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing connection</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Connection not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested billing connection could not be loaded.</p>
            <Link
              href="/billing-connections"
              className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to billing connections
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing connection</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{connection.display_name}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{connection.id}</span>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${readinessTone(connection.status)}`}>
                      {formatReadinessStatus(connection.status)}
                    </span>
                    <span className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">
                      {connection.environment}
                    </span>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Link
                    href="/billing-connections"
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
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
              <SummaryStat label="Linked workspaces" value={String(connection.linked_workspace_count)} helper={connection.workspace_ready ? "Ready for assignment" : "Sync before attaching to more workspaces"} />
              <SummaryStat label="Secret" value={connection.secret_configured ? "Configured" : "Missing"} helper="Secret material stays outside the database" />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_420px]">
              <div className="grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Provider sync</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Connection health</h2>
                      <p className="mt-2 text-sm text-slate-600">Track whether Alpha can safely attach this connection to workspaces.</p>
                    </div>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${readinessTone(connection.sync_state)}`}>
                      {formatReadinessStatus(connection.sync_state)}
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3 lg:grid-cols-2">
                    <MetaItem label="Sync state" value={connection.sync_state.replaceAll("_", " ")} />
                    <MetaItem label="Workspace readiness" value={connection.workspace_ready ? "Ready" : "Needs sync"} />
                    <MetaItem label="Connected at" value={connection.connected_at ? formatExactTimestamp(connection.connected_at) : "-"} />
                    <MetaItem label="Last synced at" value={connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "-"} />
                  </div>
                  <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">{connection.sync_summary}</div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Configuration</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Metadata and overrides</h2>
                    </div>
                    {!isEditing ? (
                      <button
                        type="button"
                        onClick={startEditing}
                        className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                      >
                        Edit
                      </button>
                    ) : null}
                  </div>

                  {!isEditing ? (
                    <div className="mt-5 grid gap-3 lg:grid-cols-2">
                      <MetaItem label="Display name" value={connection.display_name} />
                      <MetaItem label="Environment" value={connection.environment} />
                      <MetaItem
                        label="Billing organization override"
                        value={connection.lago_organization_id || "Resolved from platform config on next sync"}
                        mono={Boolean(connection.lago_organization_id)}
                      />
                      <MetaItem label="Provider code override" value={connection.lago_provider_code || "-"} mono={Boolean(connection.lago_provider_code)} />
                    </div>
                  ) : (
                    <div className="mt-5 grid gap-4 lg:grid-cols-2">
                      <InputField label="Connection name" value={displayName} onChange={setDisplayName} placeholder="Stripe Sandbox" />
                      <label className="grid gap-2 text-sm text-slate-700">
                        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Environment</span>
                        <select
                          aria-label="Environment"
                          value={environment}
                          onChange={(event) => setEnvironment(event.target.value as "test" | "live")}
                          className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                        >
                          <option value="test">Test</option>
                          <option value="live">Live</option>
                        </select>
                      </label>
                      <div className="lg:col-span-2 rounded-xl border border-slate-200 bg-slate-50 p-4">
                        <p className="text-sm font-semibold text-slate-950">Internal overrides</p>
                        <p className="mt-2 text-xs leading-relaxed text-slate-600">
                          Alpha should normally resolve the backing billing organization from platform configuration. Use overrides only when you intentionally need a non-default target.
                        </p>
                        <div className="mt-4 grid gap-4 lg:grid-cols-2">
                          <InputField
                            label="Billing organization override"
                            value={lagoOrganizationID}
                            onChange={setLagoOrganizationID}
                            placeholder="4a3951fe-09d8-40ae-8425-6a05aacbd4ea"
                          />
                          <InputField
                            label="Provider code override"
                            value={lagoProviderCode}
                            onChange={setLagoProviderCode}
                            placeholder="alpha_stripe_test_bpc_..."
                          />
                        </div>
                      </div>
                      <div className="lg:col-span-2 flex flex-wrap gap-3">
                        <button
                          type="button"
                          onClick={() => updateMutation.mutate()}
                          disabled={!csrfToken || updateMutation.isPending || !displayName.trim()}
                          className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {updateMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                          Save changes
                        </button>
                        <button
                          type="button"
                          onClick={cancelEditing}
                          disabled={updateMutation.isPending}
                          className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  )}
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Actions</p>
                  <div className="mt-4 grid gap-3">
                    <button
                      type="button"
                      onClick={() => syncMutation.mutate()}
                      disabled={!csrfToken || syncMutation.isPending || disableMutation.isPending || updateMutation.isPending || connection.status === "disabled"}
                      className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {syncMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                      Sync now
                    </button>
                    <button
                      type="button"
                      onClick={() => disableMutation.mutate()}
                      disabled={!csrfToken || disableMutation.isPending || syncMutation.isPending || updateMutation.isPending || connection.status === "disabled"}
                      className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 text-sm font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {disableMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldOff className="h-4 w-4" />}
                      Disable connection
                    </button>
                    <Link
                      href="/workspaces/new"
                      className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      <CreditCard className="h-4 w-4" />
                      Use in workspace setup
                    </Link>
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Metadata</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Provider" value={connection.provider_type} />
                    <MetaItem label="Linked workspaces" value={String(connection.linked_workspace_count)} />
                    <MetaItem label="Secret configured" value={connection.secret_configured ? "Yes" : "No"} />
                  </div>
                </section>
              </aside>
            </div>
          </>
        )}
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

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value}</dd>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (next: string) => void; placeholder?: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}
