"use client";

import Link from "next/link";
import { useState } from "react";
import { AlertTriangle, ArrowLeft, CheckCircle2, Clock3, CreditCard, LoaderCircle, RefreshCcw, ShieldOff, XCircle } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  disableBillingProviderConnection,
  fetchBillingProviderConnection,
  refreshBillingProviderConnection,
  rotateBillingProviderConnectionSecret,
  updateBillingProviderConnection,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

type HealthCheckTone = "good" | "warn" | "bad" | "neutral";

type ConnectionHealthCheck = {
  label: string;
  status: string;
  detail: string;
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

function healthCheckTone(tone: HealthCheckTone): string {
  switch (tone) {
    case "good":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "warn":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "bad":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

function HealthCheckIcon({ tone }: { tone: HealthCheckTone }) {
  switch (tone) {
    case "good":
      return <CheckCircle2 className="h-4 w-4" />;
    case "warn":
      return <Clock3 className="h-4 w-4" />;
    case "bad":
      return <XCircle className="h-4 w-4" />;
    default:
      return <AlertTriangle className="h-4 w-4" />;
  }
}

function buildVerificationDiagnosis(connection: {
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
}): ConnectionVerificationDiagnosis {
  const workspaceImpact =
    connection.linked_workspace_count > 0
      ? `${connection.linked_workspace_count} linked workspace${connection.linked_workspace_count === 1 ? "" : "s"} depend on this path.`
      : "No workspaces depend on this connection yet.";

  if (connection.status === "disabled") {
    return {
      code: "disabled",
      title: "Connection is disabled",
      summary: "This connection is out of service.",
      nextStep: "Move workspaces to another active connection before replacing it.",
      workspaceImpact,
      tone: "bad",
    };
  }

  if (!connection.secret_configured) {
    return {
      code: "missing_secret",
      title: "Secret is missing",
      summary: "Alpha cannot check this connection without a stored Stripe secret.",
      nextStep: "Store the secret, then refresh the connection.",
      workspaceImpact,
      tone: "bad",
    };
  }

  if (!connection.check_ready) {
    return {
      code: "billing_setup_required",
      title: "Billing setup required",
      summary: connection.check_blocker_summary || "Complete billing setup before Alpha can run a full connection check.",
      nextStep: "Complete billing setup, then run the first connection check.",
      workspaceImpact,
      tone: "warn",
    };
  }

  if (connection.sync_state === "failed") {
    return {
      code: "verification_failed",
      title: "Needs attention",
      summary: connection.last_sync_error || connection.sync_summary || "Alpha could not confirm that Stripe is ready.",
      nextStep: "Correct the issue, then refresh the connection.",
      workspaceImpact,
      tone: "bad",
    };
  }

  if (connection.sync_state === "pending") {
    if (connection.last_synced_at && !connection.workspace_ready) {
      return {
        code: "billing_setup_pending",
        title: "Check completed",
        summary: connection.sync_summary || "Stripe was checked, but billing setup is still incomplete for workspace use.",
        nextStep: "Complete billing setup before using this connection with workspaces.",
        workspaceImpact,
        tone: "warn",
      };
    }
    return {
      code: "verification_pending",
      title: "Check required",
      summary: "A change was made, but Alpha has not completed a fresh connection check yet.",
      nextStep: "Run another connection check before using this connection for billing.",
      workspaceImpact,
      tone: "warn",
    };
  }

  if (connection.sync_state === "never_synced") {
    return {
      code: "never_synced",
      title: "Check required",
      summary: "The secret is stored, but Alpha has not checked this Stripe connection yet.",
      nextStep: "Run the first connection check.",
      workspaceImpact,
      tone: "warn",
    };
  }

  return {
    code: "verified",
    title: "Connected",
    summary: connection.last_synced_at
      ? `Last checked ${formatExactTimestamp(connection.last_synced_at)}.`
      : "This Stripe connection is healthy.",
    nextStep:
      connection.linked_workspace_count > 0
        ? "Healthy. Ready for current and additional workspace assignments."
        : "Healthy. Ready for the next workspace assignment.",
    workspaceImpact,
    tone: "good",
  };
}

function buildConnectionHealthChecks(connection: {
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
}): ConnectionHealthCheck[] {
  const checks: ConnectionHealthCheck[] = [];

  checks.push(
    connection.secret_configured
      ? {
          label: "Secret material",
          status: "Stored",
          detail: "Alpha has the Stripe secret and can check the connection.",
          tone: "good",
        }
      : {
          label: "Secret material",
          status: "Missing",
          detail: "No Stripe secret is stored, so Alpha cannot check this connection.",
          tone: "bad",
        },
  );

  checks.push(
    connection.check_ready
      ? {
          label: "Connection check",
          status: "Available",
          detail: "Alpha can run a full Stripe and billing readiness check for this connection.",
          tone: "good",
        }
      : {
          label: "Connection check",
          status: "Blocked",
          detail: connection.check_blocker_summary || "Complete billing setup before Alpha can run a full connection check.",
          tone: "warn",
        },
  );

  if (connection.status === "disabled") {
    checks.push({
      label: "Workspace assignment",
      status: "Blocked",
      detail: "Disabled connections cannot be used for new workspace handoff.",
      tone: "bad",
    });
  } else if (connection.workspace_ready) {
    checks.push({
      label: "Workspace assignment",
      status: "Ready",
      detail:
        connection.linked_workspace_count > 0
          ? `Currently supporting ${connection.linked_workspace_count} workspace${connection.linked_workspace_count === 1 ? "" : "s"}.`
          : "Ready for the first workspace assignment.",
      tone: "good",
    });
  } else {
    checks.push({
      label: "Workspace assignment",
      status: connection.linked_workspace_count > 0 ? "At risk" : "Blocked",
      detail:
        connection.linked_workspace_count > 0
          ? `There are ${connection.linked_workspace_count} linked workspace${connection.linked_workspace_count === 1 ? "" : "s"}, but the connection is not currently ready.`
          : connection.check_ready
            ? "Run a successful connection check before attaching new workspaces."
            : "Complete billing setup before attaching new workspaces.",
      tone: connection.linked_workspace_count > 0 ? "bad" : "warn",
    });
  }

  return checks;
}

export function BillingConnectionDetailScreen({ connectionID }: { connectionID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [isEditing, setIsEditing] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [environment, setEnvironment] = useState<"test" | "live">("test");
  const [rotatedStripeSecretKey, setRotatedStripeSecretKey] = useState("");

  const connectionQuery = useQuery({
    queryKey: ["billing-provider-connection", apiBaseURL, connectionID],
    queryFn: () => fetchBillingProviderConnection({ runtimeBaseURL: apiBaseURL, connectionID }),
    enabled: isAuthenticated && isPlatformAdmin && connectionID.trim().length > 0,
  });

  const refreshMutation = useMutation({
    mutationFn: () => refreshBillingProviderConnection({ runtimeBaseURL: apiBaseURL, csrfToken, connectionID }),
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

  const rotateSecretMutation = useMutation({
    mutationFn: () =>
      rotateBillingProviderConnectionSecret({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        connectionID,
        stripeSecretKey: rotatedStripeSecretKey.trim(),
      }),
    onSuccess: async () => {
      setRotatedStripeSecretKey("");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connection", apiBaseURL, connectionID] }),
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] }),
      ]);
    },
  });

  const connection = connectionQuery.data ?? null;
  const healthChecks = connection ? buildConnectionHealthChecks(connection) : [];
  const verificationDiagnosis = connection ? buildVerificationDiagnosis(connection) : null;
  const refreshActionLabel = !connection
    ? "Check connection"
    : !connection.check_ready
      ? "Billing setup required"
      : connection.sync_state === "never_synced"
        ? "Run first check"
        : "Refresh connection status";
  const refreshActionHelper = !connection
    ? ""
    : !connection.check_ready
      ? connection.check_blocker_summary || "Complete billing setup before Alpha can run a full connection check."
      : connection.sync_state === "never_synced"
        ? "Checks Stripe and completes the first billing readiness check."
        : "Rechecks Stripe and refreshes billing readiness.";
  const refreshActionDisabled = !connection || !csrfToken || refreshMutation.isPending || rotateSecretMutation.isPending || disableMutation.isPending || updateMutation.isPending || connection.status === "disabled" || !connection.check_ready;

  const startEditing = () => {
    if (!connection) return;
    setDisplayName(connection.display_name);
    setEnvironment(connection.environment);
    setIsEditing(true);
  };

  const cancelEditing = () => {
    if (connection) {
      setDisplayName(connection.display_name);
      setEnvironment(connection.environment);
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
              <SummaryStat label="Status" value={formatReadinessStatus(connection.status)} helper={verificationDiagnosis?.title || "Connection state"} />
              <SummaryStat label="Environment" value={connection.environment} helper={`Provider: ${connection.provider_type}`} />
              <SummaryStat
                label="Linked workspaces"
                value={String(connection.linked_workspace_count)}
                helper={connection.workspace_ready ? "Ready for workspace use" : connection.check_ready ? "Check required before workspace use" : "Billing setup required before workspace use"}
              />
              <SummaryStat label="Secret" value={connection.secret_configured ? "Configured" : "Missing"} helper="Required for connection checks" />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
              <div className="min-w-0 grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Connection health</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Latest check</h2>
                      <p className="mt-2 text-sm text-slate-600">Current Stripe check status and billing readiness.</p>
                    </div>
                    <span className={`self-start rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] sm:shrink-0 ${readinessTone(connection.sync_state)}`}>
                      {formatReadinessStatus(connection.sync_state)}
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3 lg:grid-cols-2">
                    <MetaItem label="Check state" value={connection.sync_state.replaceAll("_", " ")} />
                    <MetaItem label="Billing readiness" value={connection.workspace_ready ? "Ready" : "Blocked"} />
                    <MetaItem label="First ready at" value={connection.connected_at ? formatExactTimestamp(connection.connected_at) : "-"} />
                    <MetaItem label="Last checked" value={connection.last_synced_at ? formatExactTimestamp(connection.last_synced_at) : "-"} />
                  </div>
                  <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">{connection.sync_summary}</div>
                  {connection.last_sync_error ? (
                    <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 p-4">
                      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-rose-700">Latest issue</p>
                      <p className="mt-2 text-sm leading-relaxed text-rose-800">{connection.last_sync_error}</p>
                    </div>
                  ) : null}
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Verification</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Current status</h2>
                      <p className="mt-2 text-sm text-slate-600">One clear status with supporting facts.</p>
                    </div>
                  </div>
                  {verificationDiagnosis ? (
                    <div className={`mt-5 rounded-2xl border p-5 ${healthCheckTone(verificationDiagnosis.tone)}`}>
                      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                        <div className="min-w-0">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] opacity-80">Status</p>
                          <h3 className="mt-2 text-lg font-semibold">{verificationDiagnosis.title}</h3>
                          <p className="mt-2 text-sm leading-relaxed opacity-90">{verificationDiagnosis.summary}</p>
                        </div>
                      </div>
                      <div className="mt-4 grid gap-3 lg:grid-cols-2">
                        <div className="rounded-xl border border-current/15 bg-white/50 px-4 py-3 text-sm">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] opacity-75">Workspaces</p>
                          <p className="mt-2">{verificationDiagnosis.workspaceImpact}</p>
                        </div>
                        <div className="rounded-xl border border-current/15 bg-white/50 px-4 py-3 text-sm">
                          <p className="text-[11px] font-semibold uppercase tracking-[0.14em] opacity-75">Next step</p>
                          <p className="mt-2">{verificationDiagnosis.nextStep}</p>
                        </div>
                      </div>
                    </div>
                  ) : null}
                  <div className="mt-5 grid gap-3">
                    {healthChecks.map((check) => (
                      <div key={check.label} className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                          <div className="min-w-0">
                            <p className="text-sm font-semibold text-slate-950">{check.label}</p>
                            <p className="mt-2 text-sm leading-relaxed text-slate-600">{check.detail}</p>
                          </div>
                          <span className={`inline-flex shrink-0 items-center gap-2 self-start rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${healthCheckTone(check.tone)}`}>
                            <HealthCheckIcon tone={check.tone} />
                            {check.status}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Configuration</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Connection settings</h2>
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

              <aside className="min-w-0 grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Actions</p>
                  <div className="mt-4 grid gap-3">
                      <button
                        type="button"
                        onClick={() => refreshMutation.mutate()}
                        disabled={refreshActionDisabled}
                        className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {refreshMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                        {refreshActionLabel}
                      </button>
                      <p className="text-xs leading-relaxed text-slate-600">{refreshActionHelper}</p>
                      <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                        <p className="text-sm font-semibold text-slate-950">Rotate Stripe secret</p>
                        <p className="mt-2 text-xs leading-relaxed text-slate-600">
                          Use this when the Stripe secret changes. Alpha will require a fresh connection check afterward.
                        </p>
                        <div className="mt-3 grid gap-3">
                          <InputField
                            label="New Stripe secret key"
                            value={rotatedStripeSecretKey}
                            onChange={setRotatedStripeSecretKey}
                            placeholder="sk_test_..."
                            type="password"
                          />
                          <button
                            type="button"
                            onClick={() => rotateSecretMutation.mutate()}
                            disabled={!csrfToken || rotateSecretMutation.isPending || refreshMutation.isPending || disableMutation.isPending || updateMutation.isPending || connection.status === "disabled" || !rotatedStripeSecretKey.trim()}
                            className="inline-flex h-10 w-full max-w-full items-center justify-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {rotateSecretMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                            Rotate secret
                          </button>
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => disableMutation.mutate()}
                        disabled={!csrfToken || disableMutation.isPending || rotateSecretMutation.isPending || refreshMutation.isPending || updateMutation.isPending || connection.status === "disabled"}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 text-sm font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                      {disableMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldOff className="h-4 w-4" />}
                      Disable connection
                    </button>
                    <Link
                      href="/workspaces/new"
                      className="inline-flex h-10 w-full max-w-full items-center justify-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      <CreditCard className="h-4 w-4" />
                      Open workspace setup
                    </Link>
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Metadata</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Provider" value={connection.provider_type} />
                    <MetaItem label="Linked workspaces" value={String(connection.linked_workspace_count)} />
                    <MetaItem label="Created at" value={formatExactTimestamp(connection.created_at)} />
                    <MetaItem label="Updated at" value={formatExactTimestamp(connection.updated_at)} />
                    <MetaItem label="Disabled at" value={connection.disabled_at ? formatExactTimestamp(connection.disabled_at) : "-"} />
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

function InputField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        aria-label={label}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}
