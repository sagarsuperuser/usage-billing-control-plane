"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ArrowRight, Building2, CreditCard, KeyRound, LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchBillingProviderConnections, onboardTenant } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { type BillingProviderConnection, type TenantOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function connectionLabel(connection: BillingProviderConnection): string {
  return `${connection.display_name} · ${connection.environment} · ${connection.status}`;
}

export function TenantOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();

  const [tenantID, setTenantID] = useState("");
  const [tenantName, setTenantName] = useState("");
  const [billingProviderConnectionID, setBillingProviderConnectionID] = useState("");
  const [bootstrapAdminKey, setBootstrapAdminKey] = useState(true);
  const [adminKeyName, setAdminKeyName] = useState("");
  const [allowExistingActiveKeys, setAllowExistingActiveKeys] = useState(false);
  const [result, setResult] = useState<TenantOnboardingResult | null>(null);
  const [flash, setFlash] = useState<string | null>(null);

  const billingConnectionsQuery = useQuery({
    queryKey: ["billing-provider-connections", apiBaseURL],
    queryFn: () => fetchBillingProviderConnections({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const connectedBillingConnections = useMemo(
    () => (billingConnectionsQuery.data ?? []).filter((item) => item.status === "connected" && item.scope === "platform"),
    [billingConnectionsQuery.data]
  );

  const selectedBillingConnection = connectedBillingConnections.find((item) => item.id === billingProviderConnectionID) ?? null;

  const onboardMutation = useMutation({
    mutationFn: () =>
      onboardTenant({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          id: tenantID.trim(),
          name: tenantName.trim(),
          billing_provider_connection_id: billingProviderConnectionID || undefined,
          bootstrap_admin_key: bootstrapAdminKey,
          admin_key_name: adminKeyName.trim() || undefined,
          allow_existing_active_keys: allowExistingActiveKeys,
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      setTenantID(payload.tenant.id);
      setTenantName(payload.tenant.name);
      setFlash(
        payload.tenant_created
          ? `Workspace ${payload.tenant.id} created successfully.`
          : `Workspace ${payload.tenant.id} updated and readiness refreshed.`
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, payload.tenant.id] }),
      ]);
    },
  });

  const createdSecret = result?.tenant_admin_bootstrap.secret ?? "";

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-amber-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1200px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Platform Setup</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Workspace Setup</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Create a workspace, attach a connected billing provider, and generate the first admin credential. Create billing connections first so workspace setup stays focused on tenant provisioning.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/billing-connections"
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Open billing connections
              </Link>
              <Link
                href="/workspaces"
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
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
            body="This screen drives /internal onboarding APIs. Sign in with a platform_admin API key to create workspaces."
          />
        ) : null}

        {flash ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100">
            {flash}
          </section>
        ) : null}

        <div className="grid gap-6 2xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="min-w-0 rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Guided setup</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Create workspace</h2>
                <p className="mt-2 max-w-2xl text-sm text-slate-300">
                  This page only creates or reconciles the workspace. Billing connection lifecycle now lives on dedicated billing connection pages.
                </p>
              </div>
              <span className="inline-flex rounded-xl border border-cyan-400/40 bg-cyan-500/10 p-3 text-cyan-100">
                <Building2 className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-6 grid gap-3 lg:grid-cols-3">
              <StepCard index="1" title="Name the workspace" body="Use a stable workspace ID and a display name your team will recognize later." />
              <StepCard index="2" title="Select billing connection" body="Choose a connected billing provider owned by Alpha instead of entering raw billing engine mappings here." />
              <StepCard index="3" title="Create admin access" body="Generate the first admin credential now, or leave it for a controlled handoff later." />
            </div>

            <div className="mt-6 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 1</p>
              <h3 className="mt-2 text-lg font-semibold text-white">Workspace identity</h3>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <InputField label="Workspace ID" value={tenantID} onChange={setTenantID} placeholder="tenant_acme" />
                <InputField label="Workspace name" value={tenantName} onChange={setTenantName} placeholder="Acme Corp" />
              </div>
            </div>

            <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 2</p>
              <h3 className="mt-2 text-lg font-semibold text-white">Billing connection</h3>
              <p className="mt-2 text-sm text-slate-300">
                Billing connections are created separately so Stripe secrets and Lago sync stay managed at the platform layer.
              </p>
              {billingConnectionsQuery.isLoading ? (
                <div className="mt-4 flex items-center gap-2 text-sm text-slate-300">
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                  Loading billing connections
                </div>
              ) : connectedBillingConnections.length === 0 ? (
                <div className="mt-4 rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
                  No connected billing providers are available yet.
                  <div className="mt-3">
                    <Link
                      href="/billing-connections/new"
                      className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-300/30 bg-white/10 px-3 text-sm font-medium text-amber-50 transition hover:bg-white/15"
                    >
                      <CreditCard className="h-4 w-4" />
                      Create billing connection
                    </Link>
                  </div>
                </div>
              ) : (
                <div className="mt-4 grid gap-4">
                  <label className="grid gap-2 text-sm text-slate-200">
                    <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Billing connection</span>
                    <select
                      aria-label="Billing connection"
                      value={billingProviderConnectionID}
                      onChange={(event) => setBillingProviderConnectionID(event.target.value)}
                      className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                    >
                      <option value="">Select a connected billing provider</option>
                      {connectedBillingConnections.map((connection) => (
                        <option key={connection.id} value={connection.id}>
                          {connectionLabel(connection)}
                        </option>
                      ))}
                    </select>
                  </label>
                  {selectedBillingConnection ? (
                    <div className="rounded-2xl border border-cyan-400/20 bg-cyan-500/5 p-4">
                      <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Selected connection</p>
                      <h4 className="mt-2 text-base font-semibold text-white">{selectedBillingConnection.display_name}</h4>
                      <div className="mt-3 grid gap-3 md:grid-cols-2">
                        <MetaItem label="Connection ID" value={selectedBillingConnection.id} mono />
                        <MetaItem label="Environment" value={selectedBillingConnection.environment} />
                        <MetaItem label="Billing organization" value={selectedBillingConnection.lago_organization_id || "-"} mono />
                        <MetaItem label="Lago provider code" value={selectedBillingConnection.lago_provider_code || "-"} mono />
                      </div>
                    </div>
                  ) : null}
                </div>
              )}
            </div>

            <details className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <summary className="cursor-pointer list-none">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 3</p>
                    <h3 className="mt-2 text-lg font-semibold text-white">Advanced admin access options</h3>
                  </div>
                  <span className="text-xs uppercase tracking-[0.14em] text-slate-400">Expand</span>
                </div>
              </summary>
              <div className="mt-4 grid gap-4 md:grid-cols-[1.15fr_0.85fr]">
                <InputField
                  label="Admin credential name"
                  value={adminKeyName}
                  onChange={setAdminKeyName}
                  placeholder="bootstrap-admin-tenant_acme"
                />
                <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
                  <p className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Advanced controls</p>
                  <label className="mt-3 flex items-center gap-2 text-sm text-slate-200">
                    <input
                      type="checkbox"
                      checked={bootstrapAdminKey}
                      onChange={(event) => setBootstrapAdminKey(event.target.checked)}
                      className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                    />
                    Generate first admin credential
                  </label>
                  <label className="mt-3 flex items-center gap-2 text-sm text-slate-200">
                    <input
                      type="checkbox"
                      checked={allowExistingActiveKeys}
                      onChange={(event) => setAllowExistingActiveKeys(event.target.checked)}
                      className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                    />
                    Allow existing active credentials
                  </label>
                </div>
              </div>
            </details>

            <div className="mt-4 rounded-2xl border border-cyan-400/20 bg-cyan-500/5 p-4">
              <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Before you run</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={tenantID.trim().length > 0} text="Workspace ID is set" />
                <ChecklistLine done={tenantName.trim().length > 0} text="Workspace name is set" />
                <ChecklistLine done={Boolean(billingProviderConnectionID)} text="Billing connection is selected" />
                <ChecklistLine done={connectedBillingConnections.length > 0} text="At least one connected billing provider exists" />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardMutation.mutate();
                }}
                disabled={
                  !isPlatformAdmin ||
                  !csrfToken ||
                  onboardMutation.isPending ||
                  !tenantID.trim() ||
                  !tenantName.trim() ||
                  !billingProviderConnectionID
                }
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {onboardMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                Run workspace setup
              </button>
              <button
                type="button"
                onClick={() => {
                  setTenantID("");
                  setTenantName("");
                  setBillingProviderConnectionID("");
                  setAdminKeyName("");
                  setBootstrapAdminKey(true);
                  setAllowExistingActiveKeys(false);
                  setResult(null);
                  setFlash(null);
                }}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Reset form
              </button>
            </div>

            {createdSecret ? (
              <div className="mt-6 rounded-2xl border border-amber-400/40 bg-amber-500/10 p-4 text-sm text-amber-100">
                <div className="flex items-center gap-2 font-semibold text-amber-50">
                  <KeyRound className="h-4 w-4" />
                  First admin credential
                </div>
                <p className="mt-2 break-all rounded-xl border border-white/10 bg-slate-950/60 px-3 py-3 font-mono text-xs text-amber-50">
                  {createdSecret}
                </p>
                <p className="mt-2 text-xs text-amber-200">This value is shown once. Capture it now and hand it off securely.</p>
              </div>
            ) : null}
          </section>

          <aside className="flex flex-col gap-4">
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">After setup</p>
              <h2 className="mt-2 text-xl font-semibold text-white">Use dedicated workspace pages</h2>
              <div className="mt-4 grid gap-3">
                <ChecklistLine done text="Create or reconcile the workspace here" />
                <ChecklistLine done text="Open the workspace detail page to review readiness" />
                <ChecklistLine done text="Manage Stripe and Lago sync from Billing Connections" />
              </div>
            </section>

            {result?.tenant ? (
              <section className="rounded-3xl border border-cyan-400/20 bg-cyan-500/5 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace created</p>
                <h2 className="mt-2 break-words text-xl font-semibold text-white">{result.tenant.name}</h2>
                <p className="mt-1 break-all font-mono text-xs text-slate-400">{result.tenant.id}</p>
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <SummaryStat
                    label="Workspace"
                    value={result.readiness.tenant.status}
                    helper={result.readiness.tenant.tenant_active ? "Active" : "Needs activation"}
                  />
                  <SummaryStat
                    label="Overall"
                    value={result.readiness.status}
                    helper={`${result.readiness.missing_steps.length} checklist items remain`}
                  />
                </div>
                {result.tenant.billing_provider_connection_id ? (
                  <div className="mt-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-4 text-sm text-slate-200">
                    Linked billing connection
                    <p className="mt-2 break-all font-mono text-xs text-slate-400">{result.tenant.billing_provider_connection_id}</p>
                  </div>
                ) : null}
                <div className="mt-5 flex flex-col gap-3">
                  <Link
                    href={`/workspaces/${encodeURIComponent(result.tenant.id)}`}
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
                  >
                    View workspace detail
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link
                    href="/workspaces"
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                  >
                    Open workspace directory
                  </Link>
                </div>
              </section>
            ) : (
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">What changes now</p>
                <div className="mt-4 space-y-3 text-sm text-slate-300">
                  <p>Billing connections now own Stripe secret storage and Lago sync.</p>
                  <p>Workspace setup only links a prepared billing connection to the tenant.</p>
                  <p>Use workspace detail pages for readiness review and next actions.</p>
                </div>
              </section>
            )}
          </aside>
        </div>
      </main>
    </div>
  );
}

function StepCard({ index, title, body }: { index: string; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/45 p-4">
      <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Step {index}</p>
      <p className="mt-2 text-sm font-semibold text-white">{title}</p>
      <p className="mt-2 text-sm text-slate-300">{body}</p>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="min-w-0 rounded-2xl border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <label className="grid gap-2 text-sm text-slate-200">
      <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{label}</span>
      <input
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </label>
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
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 break-all text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value}</p>
    </div>
  );
}
