"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { AlertTriangle, CheckCircle2, Clock3, LoaderCircle, RefreshCcw, ShieldOff, XCircle } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import {
  disableBillingProviderConnection,
  fetchBillingProviderConnection,
  refreshBillingProviderConnection,
  rotateBillingProviderConnectionSecret,
  updateBillingProviderConnection,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { formatReadinessStatus } from "@/lib/readiness";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type HealthCheckTone = "good" | "warn" | "bad" | "neutral";

type ConnectionHealthCheck = {
  label: string;
  status: string;
  tone: HealthCheckTone;
};

type ConnectionVerificationDiagnosis = {
  code: string;
  title: string;
  summary: string;
  nextStep: string;
  workspaceImpact: string;
  tone: HealthCheckTone;
};

type ConnectionShape = {
  status: string;
  sync_state: string;
  sync_summary: string;
  check_ready: boolean;
  check_blocker_code?: string;
  check_blocker_summary?: string;
  linked_workspace_count: number;
  workspace_ready: boolean;
  secret_configured: boolean;
  lago_organization_id?: string;
  lago_provider_code?: string;
  last_synced_at?: string;
  last_sync_error?: string;
  connected_at?: string;
  display_name: string;
  environment: string;
  provider_type: string;
  created_at: string;
  updated_at: string;
  disabled_at?: string;
};

/* ------------------------------------------------------------------ */
/*  Pure helpers (outside component)                                    */
/* ------------------------------------------------------------------ */

function statusBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "connected":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "sync_error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

function toneBadgeClass(tone: HealthCheckTone): string {
  switch (tone) {
    case "good":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "warn":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "bad":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

function toneBannerClass(tone: HealthCheckTone): string {
  switch (tone) {
    case "good":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "warn":
      return "border-amber-200 bg-amber-50 text-amber-800";
    case "bad":
      return "border-rose-200 bg-rose-50 text-rose-800";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

function HealthCheckIcon({ tone }: { tone: HealthCheckTone }) {
  switch (tone) {
    case "good":
      return <CheckCircle2 className="h-3.5 w-3.5" />;
    case "warn":
      return <Clock3 className="h-3.5 w-3.5" />;
    case "bad":
      return <XCircle className="h-3.5 w-3.5" />;
    default:
      return <AlertTriangle className="h-3.5 w-3.5" />;
  }
}

function buildVerificationDiagnosis(connection: ConnectionShape): ConnectionVerificationDiagnosis {
  const workspaceImpact =
    connection.linked_workspace_count > 0
      ? `${connection.linked_workspace_count} linked workspace${connection.linked_workspace_count === 1 ? "" : "s"} depend on this path.`
      : "No workspaces depend on this connection yet.";

  if (connection.status === "disabled") {
    return { code: "disabled", title: "Connection is disabled", summary: "This connection is out of service.", nextStep: "Move workspaces to another active connection before replacing it.", workspaceImpact, tone: "bad" };
  }
  if (!connection.secret_configured) {
    return { code: "missing_secret", title: "Secret is missing", summary: "Alpha cannot check this connection without a stored Stripe secret.", nextStep: "Store the secret, then refresh the connection.", workspaceImpact, tone: "bad" };
  }
  if (!connection.check_ready) {
    return { code: "check_blocked", title: "Check unavailable", summary: connection.check_blocker_summary || "Alpha cannot run a connection check yet.", nextStep: "Add the missing secret, then run the first connection check.", workspaceImpact, tone: "warn" };
  }
  if (connection.sync_state === "failed") {
    return { code: "verification_failed", title: "Needs attention", summary: connection.last_sync_error || connection.sync_summary || "Alpha could not confirm that Stripe is ready.", nextStep: "Correct the issue, then refresh the connection.", workspaceImpact, tone: "bad" };
  }
  if (connection.sync_state === "pending") {
    return { code: "verification_pending", title: "Check required", summary: "A change was made, but Alpha has not completed a fresh connection check yet.", nextStep: "Run another connection check before assigning this connection to a workspace.", workspaceImpact, tone: "warn" };
  }
  if (connection.sync_state === "never_synced") {
    return { code: "never_synced", title: "Check required", summary: "The secret is stored, but Alpha has not checked this Stripe connection yet.", nextStep: "Run the first connection check.", workspaceImpact, tone: "warn" };
  }
  return {
    code: "verified",
    title: "Connected",
    summary: connection.last_synced_at ? `Last checked ${formatExactTimestamp(connection.last_synced_at)}.` : "This Stripe connection is healthy.",
    nextStep: connection.linked_workspace_count > 0 ? "Healthy. Ready for current and additional workspace assignments." : "Healthy. Ready for the next workspace assignment.",
    workspaceImpact,
    tone: "good",
  };
}

function buildConnectionHealthChecks(connection: ConnectionShape): ConnectionHealthCheck[] {
  const checks: ConnectionHealthCheck[] = [];
  checks.push(connection.secret_configured ? { label: "Secret material", status: "Stored", tone: "good" } : { label: "Secret material", status: "Missing", tone: "bad" });
  checks.push(connection.check_ready ? { label: "Connection check", status: "Available", tone: "good" } : { label: "Connection check", status: "Blocked", tone: "warn" });
  if (connection.status === "disabled") {
    checks.push({ label: "Workspace assignment", status: "Blocked", tone: "bad" });
  } else if (connection.workspace_ready) {
    checks.push({ label: "Workspace assignment", status: "Ready", tone: "good" });
  } else {
    checks.push({ label: "Workspace assignment", status: connection.linked_workspace_count > 0 ? "At risk" : "Blocked", tone: connection.linked_workspace_count > 0 ? "bad" : "warn" });
  }
  return checks;
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function BillingConnectionDetailScreen({ connectionID }: { connectionID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;
  const [isEditing, setIsEditing] = useState(false);
  const [rotatedStripeSecretKey, setRotatedStripeSecretKey] = useState("");
  const { register, handleSubmit, reset: resetEdit } = useForm({
    resolver: zodResolver(z.object({ display_name: z.string().min(1), environment: z.enum(["test", "live"]) })),
    defaultValues: { display_name: "", environment: "test" as const },
  });

  const connectionQuery = useQuery({
    queryKey: ["billing-provider-connection", apiBaseURL, connectionID],
    queryFn: () => fetchBillingProviderConnection({ runtimeBaseURL: apiBaseURL, connectionID }),
    enabled: isAuthenticated && isPlatformAdmin && connectionID.trim().length > 0,
  });

  const refreshMutation = useMutation({
    mutationFn: () => refreshBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID }),
    onSuccess: async () => { showSuccess("Connection check complete"); await Promise.all([queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }), queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] })]); },
    onError: (err: Error) => { showError("Check failed", err.message || "Could not refresh connection status."); },
  });

  const updateMutation = useMutation({
    mutationFn: (data: { display_name: string; environment: "test" | "live" }) =>
      updateBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID, body: { display_name: data.display_name.trim(), environment: data.environment } }),
    onSuccess: async () => { showSuccess("Connection settings saved"); setIsEditing(false); await Promise.all([queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }), queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] })]); },
    onError: (err: Error) => { showError("Update failed", err.message || "Could not update connection settings."); },
  });

  const disableMutation = useMutation({
    mutationFn: () => disableBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID }),
    onSuccess: async () => { showSuccess("Connection disabled"); await Promise.all([queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }), queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] })]); },
    onError: (err: Error) => { showError("Disable failed", err.message || "Could not disable connection."); },
  });

  const rotateSecretMutation = useMutation({
    mutationFn: () => rotateBillingProviderConnectionSecret({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID, stripeSecretKey: rotatedStripeSecretKey.trim() }),
    onSuccess: async () => { showSuccess("Secret rotated", "The new Stripe secret key is now active."); setRotatedStripeSecretKey(""); await Promise.all([queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }), queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] })]); },
    onError: (err: Error) => { showError("Rotation failed", err.message || "Could not rotate the Stripe secret key."); },
  });

  const connection = connectionQuery.data ?? null;
  const healthChecks = connection ? buildConnectionHealthChecks(connection as ConnectionShape) : [];
  const diagnosis = connection ? buildVerificationDiagnosis(connection as ConnectionShape) : null;
  const anyPending = refreshMutation.isPending || rotateSecretMutation.isPending || disableMutation.isPending || updateMutation.isPending;
  const refreshDisabled = !connection || !csrfToken || anyPending || connection.status === "disabled" || !connection.check_ready;

  const startEditing = () => {
    if (!connection) return;
    resetEdit({ display_name: connection.display_name, environment: connection.environment as "test" | "live" });
    setIsEditing(true);
  };

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Billing Connections" }, { label: connection?.display_name || connectionID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice title="Platform session required" body="Billing connections are managed at the platform layer. Sign in with a platform account to inspect them." actionHref="/customers" actionLabel="Open tenant home" />
        ) : null}

        {canViewPlatformSurface ? (
          connectionQuery.isLoading ? (
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading billing connection detail
              </div>
            </section>
          ) : connectionQuery.isError || !connection ? (
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <p className="text-sm font-semibold text-slate-900">Connection not available</p>
              <p className="mt-1 text-sm text-slate-500">The requested billing connection could not be loaded.</p>
            </section>
          ) : (
            <SectionErrorBoundary>
              <div className="rounded-lg border border-slate-200 bg-white shadow-sm divide-y divide-slate-200">
                {/* ---- Header ---- */}
                <div className="px-5 py-4">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex items-center gap-3 min-w-0">
                      <h1 className="text-base font-semibold text-slate-900 truncate">{connection.display_name}</h1>
                      <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${statusBadgeClass(connection.status)}`}>
                        {formatReadinessStatus(connection.status)}
                      </span>
                      <span className="shrink-0 rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-slate-600">
                        {connection.environment}
                      </span>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <button type="button" onClick={() => refreshMutation.mutate()} disabled={refreshDisabled} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40">
                        {refreshMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCcw className="h-3.5 w-3.5" />}
                        Refresh
                      </button>
                      {!isEditing && (
                        <button type="button" onClick={startEditing} className="inline-flex h-8 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                          Edit
                        </button>
                      )}
                      <ConfirmDialog title="Disable this connection?" description="This connection will stop processing payments for linked workspaces." confirmLabel="Disable connection" onConfirm={async () => { await disableMutation.mutateAsync(); }}>
                        {(open) => (
                          <button type="button" onClick={open} disabled={!csrfToken || anyPending || connection.status === "disabled"} className="inline-flex h-8 items-center gap-1.5 rounded-md border border-rose-200 bg-rose-50 px-3 text-xs font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-40">
                            <ShieldOff className="h-3.5 w-3.5" />
                            Disable
                          </button>
                        )}
                      </ConfirmDialog>
                    </div>
                  </div>
                  <p className="mt-1.5 text-xs text-slate-500">
                    {connection.linked_workspace_count} workspace{connection.linked_workspace_count === 1 ? "" : "s"}
                    {connection.last_synced_at ? <> &middot; Last checked {formatExactTimestamp(connection.last_synced_at)}</> : null}
                  </p>
                </div>

                {/* ---- Details grid ---- */}
                <div className="px-5 py-4">
                  <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs text-slate-400">Provider</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{connection.provider_type}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Check state</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{connection.sync_state.replaceAll("_", " ")}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Secret</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{connection.secret_configured ? "Configured" : "Missing"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Assignment readiness</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{connection.workspace_ready ? "Ready" : "Blocked"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Created</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(connection.created_at)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Last checked</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "-"}</dd>
                    </div>
                  </dl>
                </div>

                {/* ---- Diagnosis banner ---- */}
                {diagnosis && diagnosis.code !== "verified" ? (
                  <div className="px-5 py-0">
                    <div className={`rounded-md border px-4 py-3 my-4 text-sm ${toneBannerClass(diagnosis.tone)}`}>
                      <p className="font-medium">{diagnosis.title}</p>
                      <p className="mt-0.5 text-xs opacity-80">{diagnosis.summary}</p>
                    </div>
                    {connection.last_sync_error ? (
                      <div className="rounded-md border border-rose-200 bg-rose-50 px-4 py-3 mb-4 text-sm text-rose-800">
                        <p className="font-medium">Latest error</p>
                        <p className="mt-0.5 text-xs">{connection.last_sync_error}</p>
                      </div>
                    ) : null}
                  </div>
                ) : null}

                {/* ---- Health checks ---- */}
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-slate-400 mb-3">Health checks</p>
                  <div className="flex flex-wrap gap-4">
                    {healthChecks.map((check) => (
                      <div key={check.label} className="flex items-center gap-2 text-sm">
                        <span className="text-slate-600">{check.label}</span>
                        <span className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${toneBadgeClass(check.tone)}`}>
                          <HealthCheckIcon tone={check.tone} />
                          {check.status}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>

                {/* ---- Secret rotation ---- */}
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-slate-400 mb-3">Rotate Stripe secret</p>
                  <div className="flex items-end gap-3">
                    <label className="flex-1 min-w-0">
                      <span className="sr-only">New Stripe secret key</span>
                      <input
                        type="password"
                        value={rotatedStripeSecretKey}
                        onChange={(e) => setRotatedStripeSecretKey(e.target.value)}
                        placeholder="sk_test_..."
                        className="h-9 w-full rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                      />
                    </label>
                    <button
                      type="button"
                      onClick={() => rotateSecretMutation.mutate()}
                      disabled={!csrfToken || anyPending || connection.status === "disabled" || !rotatedStripeSecretKey.trim()}
                      className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      {rotateSecretMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCcw className="h-3.5 w-3.5" />}
                      Rotate
                    </button>
                  </div>
                </div>

                {/* ---- Edit section ---- */}
                {isEditing ? (
                  <div className="px-5 py-4">
                    <p className="text-xs font-medium text-slate-400 mb-3">Edit connection</p>
                    <div className="flex flex-wrap items-end gap-3">
                      <label className="grid gap-1.5 text-sm min-w-[200px] flex-1">
                        <span className="text-xs text-slate-500">Connection name</span>
                        <input
                          {...register("display_name")}
                          placeholder="Stripe Sandbox"
                          className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                        />
                      </label>
                      <label className="grid gap-1.5 text-sm">
                        <span className="text-xs text-slate-500">Environment</span>
                        <select
                          {...register("environment")}
                          className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                        >
                          <option value="test">Test</option>
                          <option value="live">Live</option>
                        </select>
                      </label>
                      <button
                        type="button"
                        onClick={handleSubmit((data) => updateMutation.mutate(data))}
                        disabled={!csrfToken || updateMutation.isPending}
                        className="inline-flex h-9 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-4 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {updateMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                        Save
                      </button>
                      <button
                        type="button"
                        onClick={() => setIsEditing(false)}
                        disabled={updateMutation.isPending}
                        className="inline-flex h-9 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : null}
              </div>
            </SectionErrorBoundary>
          )
        ) : null}
      </main>
    </div>
  );
}
