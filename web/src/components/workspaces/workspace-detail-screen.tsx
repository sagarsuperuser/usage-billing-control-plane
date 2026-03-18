"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ArrowLeft, Building2, CreditCard, LoaderCircle } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchBillingProviderConnection, fetchBillingProviderConnections, fetchTenantOnboardingStatus, updateTenantWorkspaceBilling } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeTenantMissingStep, describeTenantSectionStep, formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function readinessTone(status?: string): string {
  return status === "ready"
    ? "border-emerald-400/40 bg-emerald-500/10 text-emerald-100"
    : "border-amber-400/40 bg-amber-500/10 text-amber-100";
}

export function WorkspaceDetailScreen({ tenantID }: { tenantID: string }) {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [selectedConnectionID, setSelectedConnectionID] = useState("");

  const tenantStatusQuery = useQuery({
    queryKey: ["tenant-onboarding-status", apiBaseURL, tenantID],
    queryFn: () => fetchTenantOnboardingStatus({ runtimeBaseURL: apiBaseURL, tenantID }),
    enabled: isAuthenticated && isPlatformAdmin && tenantID.trim().length > 0,
  });

  const selectedTenant = tenantStatusQuery.data?.tenant ?? null;
  const selectedReadiness = tenantStatusQuery.data?.readiness ?? null;
  const activeBillingConnectionID = selectedTenant?.workspace_billing.active_billing_connection_id || selectedTenant?.billing_provider_connection_id || "";
  const billingConnectionQuery = useQuery({
    queryKey: ["billing-provider-connection", apiBaseURL, activeBillingConnectionID],
    queryFn: () =>
      fetchBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        connectionID: activeBillingConnectionID,
      }),
    enabled: isAuthenticated && isPlatformAdmin && Boolean(activeBillingConnectionID),
  });
  const billingConnectionsQuery = useQuery({
    queryKey: ["billing-provider-connections", apiBaseURL, "workspace-detail"],
    queryFn: () => fetchBillingProviderConnections({ runtimeBaseURL: apiBaseURL, limit: 100, status: "connected", scope: "platform" }),
    enabled: isAuthenticated && isPlatformAdmin,
  });
  useEffect(() => {
    setSelectedConnectionID(activeBillingConnectionID);
  }, [activeBillingConnectionID]);
  const updateWorkspaceBillingMutation = useMutation({
    mutationFn: () =>
      updateTenantWorkspaceBilling({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        tenantID,
        billingProviderConnectionID: selectedConnectionID,
      }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, tenantID] }),
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["billing-provider-connection"] }),
      ]);
    },
  });
  const nextActions = selectedReadiness?.missing_steps.map(describeTenantMissingStep) ?? [];
  const availableConnections = billingConnectionsQuery.data ?? [];
  const canSaveWorkspaceBilling =
    Boolean(csrfToken) &&
    !updateWorkspaceBillingMutation.isPending &&
    Boolean(selectedConnectionID) &&
    selectedConnectionID !== activeBillingConnectionID;

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-amber-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1240px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/workspaces", label: "Workspaces" }, { label: selectedTenant?.name || tenantID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace detail is a platform-admin view. Sign in with a platform_admin API key to inspect cross-workspace readiness."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {tenantStatusQuery.isLoading ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 text-sm text-slate-300 backdrop-blur-xl">
            <div className="flex items-center gap-2">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading workspace detail
            </div>
          </section>
        ) : tenantStatusQuery.isError || !selectedTenant || !selectedReadiness ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace detail</p>
            <h1 className="mt-2 text-3xl font-semibold text-white">Workspace not available</h1>
            <p className="mt-3 text-sm text-slate-300">The requested workspace could not be loaded from the onboarding status API.</p>
            <Link
              href="/workspaces"
              className="mt-5 inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to workspaces
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Workspace detail</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-white md:text-4xl">{selectedTenant.name}</h1>
                  <p className="mt-2 break-all font-mono text-sm text-slate-400">{selectedTenant.id}</p>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <span className={`rounded-full px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] ${readinessTone(selectedReadiness.status)}`}>
                    {formatReadinessStatus(selectedReadiness.status)}
                  </span>
                  <Link
                    href="/workspaces"
                    className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Back to workspaces
                  </Link>
                  <Link
                    href="/workspaces/new"
                    className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
                  >
                    <Building2 className="h-4 w-4" />
                    New workspace
                  </Link>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Workspace" value={selectedReadiness.tenant.status} helper={selectedReadiness.tenant.tenant_active ? "Active" : "Needs activation"} />
              <SummaryStat
                label="Billing"
                value={selectedReadiness.billing_integration.status}
                helper={
                  selectedReadiness.billing_integration.billing_connected
                    ? `Active connection linked${selectedReadiness.billing_integration.isolation_mode ? ` · ${selectedReadiness.billing_integration.isolation_mode}` : ""}`
                    : selectedReadiness.billing_integration.pricing_ready
                      ? "Pricing ready, billing not attached"
                      : "Billing and pricing still need setup"
                }
              />
              <SummaryStat label="First customer" value={selectedReadiness.first_customer.status} helper={selectedReadiness.first_customer.customer_exists ? "Customer exists" : "No customer yet"} />
              <SummaryStat label="Open actions" value={String(selectedReadiness.missing_steps.length)} helper="Remaining checklist items" />
            </section>

            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Readiness</p>
                <h2 className="mt-2 text-2xl font-semibold text-white">What still needs action</h2>

                <div className="mt-5 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                  <p className="text-sm font-semibold text-white">Next actions</p>
                  <div className="mt-3 grid gap-2">
                    {nextActions.length > 0 ? (
                      nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />)
                    ) : (
                      <ChecklistLine done text="Workspace is ready for the next operational handoff." />
                    )}
                  </div>
                </div>

                <div className="mt-4 grid gap-3 xl:grid-cols-3">
                  <ReadinessCard title="Workspace" readiness={selectedReadiness.tenant.status} missing={selectedReadiness.tenant.missing_steps} />
                  <ReadinessCard title="Billing integration" readiness={selectedReadiness.billing_integration.status} missing={selectedReadiness.billing_integration.missing_steps} />
                  <ReadinessCard title="First customer" readiness={selectedReadiness.first_customer.status} missing={selectedReadiness.first_customer.missing_steps} />
                </div>
              </section>

              <aside className="flex flex-col gap-4">
                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace billing</p>
                  {activeBillingConnectionID ? (
                    <>
                      <dl className="mt-4 grid gap-3">
                        <MetaItem label="Active connection" value={activeBillingConnectionID} mono />
                        <MetaItem label="Workspace billing status" value={formatReadinessStatus(selectedTenant.workspace_billing.status || selectedReadiness.billing_integration.workspace_billing_status || selectedReadiness.billing_integration.status)} />
                        <MetaItem label="Connection status" value={billingConnectionQuery.data ? formatReadinessStatus(billingConnectionQuery.data.status) : billingConnectionQuery.isLoading ? "Loading" : "Unavailable"} />
                        <MetaItem label="Display name" value={billingConnectionQuery.data?.display_name || "-"} />
                        <MetaItem label="Isolation mode" value={selectedTenant.workspace_billing.isolation_mode ? formatReadinessStatus(selectedTenant.workspace_billing.isolation_mode) : selectedReadiness.billing_integration.isolation_mode ? formatReadinessStatus(selectedReadiness.billing_integration.isolation_mode) : "Shared"} />
                        <MetaItem label="Binding source" value={selectedTenant.workspace_billing.source || selectedReadiness.billing_integration.workspace_billing_source || "Pending binding"} />
                      </dl>
                      <Link
                        href={`/billing-connections/${encodeURIComponent(activeBillingConnectionID)}`}
                        className="mt-4 inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
                      >
                        <CreditCard className="h-4 w-4" />
                        Open billing connection
                      </Link>
                      <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Change active connection</p>
                        <div className="mt-3 flex flex-col gap-3">
                          <select
                            aria-label="Active billing connection"
                            value={selectedConnectionID}
                            onChange={(event) => setSelectedConnectionID(event.target.value)}
                            className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                          >
                            <option value="">Select one active billing connection</option>
                            {availableConnections.map((connection) => (
                              <option key={connection.id} value={connection.id}>
                                {connection.display_name} · {connection.environment}
                              </option>
                            ))}
                          </select>
                          <button
                            type="button"
                            onClick={() => updateWorkspaceBillingMutation.mutate()}
                            disabled={!canSaveWorkspaceBilling}
                            className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {updateWorkspaceBillingMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                            Save active connection
                          </button>
                        </div>
                      </div>
                    </>
                  ) : (
                    <>
                      <dl className="mt-4 grid gap-3">
                        <MetaItem label="Workspace billing status" value="Missing" />
                        <MetaItem label="Next action" value="Select one active billing connection below" />
                      </dl>
                      <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Assign active connection</p>
                        <div className="mt-3 flex flex-col gap-3">
                          <select
                            aria-label="Active billing connection"
                            value={selectedConnectionID}
                            onChange={(event) => setSelectedConnectionID(event.target.value)}
                            className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                          >
                            <option value="">Select one active billing connection</option>
                            {availableConnections.map((connection) => (
                              <option key={connection.id} value={connection.id}>
                                {connection.display_name} · {connection.environment}
                              </option>
                            ))}
                          </select>
                          <button
                            type="button"
                            onClick={() => updateWorkspaceBillingMutation.mutate()}
                            disabled={!canSaveWorkspaceBilling}
                            className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {updateWorkspaceBillingMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                            Save active connection
                          </button>
                        </div>
                      </div>
                    </>
                  )}
                </section>

                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Metadata</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Created" value={formatExactTimestamp(selectedTenant.created_at)} />
                    <MetaItem label="Updated" value={formatExactTimestamp(selectedTenant.updated_at)} />
                    <MetaItem label="Workspace status" value={formatReadinessStatus(selectedTenant.status)} />
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
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function ReadinessCard({ title, readiness, missing }: { title: string; readiness: string; missing: string[] }) {
  const lead = missing[0] ? describeTenantSectionStep(missing[0]) : "No action needed";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-white">{title}</p>
        <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${readinessTone(readiness)}`}>
          {formatReadinessStatus(readiness)}
        </span>
      </div>
      <p className="mt-3 text-xs text-slate-300">{lead}</p>
      <p className="mt-2 text-xs text-slate-500">{missing.length === 0 ? "All set" : `${missing.length} action item(s) remaining`}</p>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/10 bg-white/5 px-3 py-3">
      <span
        className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-500/20 text-emerald-100" : "bg-amber-500/20 text-amber-100"}`}
      >
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-200">{text}</p>
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
