"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ArrowRight, Building2, CreditCard, KeyRound, LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchBillingProviderConnections, onboardTenant } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
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
    [billingConnectionsQuery.data],
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
      setFlash(payload.tenant_created ? `Workspace ${payload.tenant.id} created successfully.` : `Workspace ${payload.tenant.id} updated and readiness refreshed.`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, payload.tenant.id] }),
      ]);
    },
  });

  const createdSecret = result?.tenant_admin_bootstrap.secret ?? "";
  const canSubmit = Boolean(isPlatformAdmin && csrfToken && tenantID.trim() && tenantName.trim());

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/workspaces", label: "Workspaces" }, { label: "New" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace setup</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create workspace</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Create a workspace first, optionally attach an active billing connection, and mint the first admin service account credential if you need it. Billing connection lifecycle stays on dedicated billing pages.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link href="/billing-connections" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open billing connections</Link>
              <Link href="/workspaces" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open workspaces</Link>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="This screen drives platform onboarding APIs. Sign in with a platform admin account to create workspaces."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {flash ? <section className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{flash}</section> : null}

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
          <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Guided setup</p>
                <h2 className="mt-2 text-xl font-semibold text-slate-950">Workspace provisioning</h2>
                <p className="mt-2 max-w-2xl text-sm text-slate-600">This flow only creates or reconciles the workspace. Ongoing readiness review happens on the workspace detail page.</p>
              </div>
              <span className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                <Building2 className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-5 grid gap-3 lg:grid-cols-3">
              <StepCard index="1" title="Name the workspace" body="Use a stable ID and a display name operators will recognize later." />
              <StepCard index="2" title="Attach billing later" body="Optionally preselect one active connection now, or attach it from workspace detail after creation." />
              <StepCard index="3" title="Bootstrap admin access" body="Generate the first admin service account credential now or leave it for a controlled handoff." />
            </div>

            <div className="mt-5 grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Step 1</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Workspace identity</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <InputField label="Workspace ID" value={tenantID} onChange={setTenantID} placeholder="tenant_acme" />
                  <InputField label="Workspace name" value={tenantName} onChange={setTenantName} placeholder="Acme Corp" />
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Step 2</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Active billing connection</h3>
                <p className="mt-2 text-sm text-slate-600">Billing connections are created separately. You can leave this empty, create the workspace, and attach billing from the workspace detail page after the backend finishes org bootstrap.</p>
                {billingConnectionsQuery.isLoading ? (
                  <div className="mt-4 flex items-center gap-2 text-sm text-slate-600">
                    <LoaderCircle className="h-4 w-4 animate-spin" />
                    Loading billing connections
                  </div>
                ) : connectedBillingConnections.length === 0 ? (
                  <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700">
                    No connected billing providers are available yet. Workspace creation can still continue without one.
                    <div className="mt-3">
                      <Link href="/billing-connections/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 text-sm font-medium text-amber-700 transition hover:bg-amber-100">
                        <CreditCard className="h-4 w-4" />
                        Create billing connection
                      </Link>
                    </div>
                  </div>
                ) : (
                  <div className="mt-4 grid gap-4">
                    <label className="grid gap-2 text-sm text-slate-700">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Active billing connection</span>
                      <select
                        aria-label="Billing connection"
                        value={billingProviderConnectionID}
                        onChange={(event) => setBillingProviderConnectionID(event.target.value)}
                        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                      >
                        <option value="">Attach billing later</option>
                        {connectedBillingConnections.map((connection) => (
                          <option key={connection.id} value={connection.id}>
                            {connectionLabel(connection)}
                          </option>
                        ))}
                      </select>
                    </label>
                    {selectedBillingConnection ? (
                      <div className="rounded-xl border border-slate-200 bg-white p-4">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Selected connection</p>
                        <h4 className="mt-2 text-base font-semibold text-slate-950">{selectedBillingConnection.display_name}</h4>
                        <div className="mt-3 grid gap-3 md:grid-cols-2">
                          <MetaItem label="Connection ID" value={selectedBillingConnection.id} mono />
                          <MetaItem label="Environment" value={selectedBillingConnection.environment} />
                          <MetaItem label="Connection health" value={formatReadinessStatus(selectedBillingConnection.status)} />
                          <MetaItem label="Workspace billing" value="Will be attached during setup" />
                        </div>
                      </div>
                    ) : null}
                  </div>
                )}
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Step 3</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Admin bootstrap service account</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-[1.15fr_0.85fr]">
                  <InputField label="Bootstrap service account name" value={adminKeyName} onChange={setAdminKeyName} placeholder="bootstrap-admin-tenant_acme" />
                  <div className="rounded-xl border border-slate-200 bg-white p-4">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Advanced controls</p>
                    <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                      <input type="checkbox" checked={bootstrapAdminKey} onChange={(event) => setBootstrapAdminKey(event.target.checked)} className="h-4 w-4 rounded border-slate-300" />
                      Generate first admin service account credential
                    </label>
                    <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                      <input type="checkbox" checked={allowExistingActiveKeys} onChange={(event) => setAllowExistingActiveKeys(event.target.checked)} className="h-4 w-4 rounded border-slate-300" />
                      Allow existing active credentials
                    </label>
                  </div>
                </div>
              </section>
            </div>

            <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
              <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Before you run</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={tenantID.trim().length > 0} text="Workspace ID is set" />
                <ChecklistLine done={tenantName.trim().length > 0} text="Workspace name is set" />
                <ChecklistLine done text={billingProviderConnectionID ? "Billing connection is selected" : "Billing connection can be attached later"} />
                <ChecklistLine done text={connectedBillingConnections.length > 0 ? "A connected billing provider exists" : "Workspace can be created before billing is connected"} />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardMutation.mutate();
                }}
                disabled={!canSubmit || onboardMutation.isPending}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
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
                className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
              >
                Reset form
              </button>
            </div>

            {createdSecret ? (
              <div className="mt-6 rounded-xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-700">
                <div className="flex items-center gap-2 font-semibold text-amber-800">
                  <KeyRound className="h-4 w-4" />
                  First admin service account credential
                </div>
                <p className="mt-2">Capture this one-time credential now and hand it off through your secure admin bootstrap path.</p>
                <p className="mt-3 break-all rounded-lg border border-amber-200 bg-white px-3 py-3 font-mono text-xs text-amber-800">{createdSecret}</p>
              </div>
            ) : null}
          </section>

          <aside className="min-w-0 grid gap-5 self-start">
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">After setup</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Use dedicated workspace pages</h2>
              <div className="mt-4 grid gap-2">
                <ChecklistLine done text="Create or reconcile the workspace here" />
                <ChecklistLine done text="Open workspace detail to review readiness" />
                <ChecklistLine done text="Manage Stripe and sync from billing connections" />
              </div>
            </section>

            {result?.tenant ? (
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace created</p>
                <h2 className="mt-2 break-words text-xl font-semibold text-slate-950">{result.tenant.name}</h2>
                <p className="mt-1 break-all font-mono text-xs text-slate-500">{result.tenant.id}</p>
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <SummaryStat label="Workspace" value={result.readiness.tenant.status} helper={result.readiness.tenant.tenant_active ? "Active" : "Needs activation"} />
                  <SummaryStat label="Overall" value={result.readiness.status} helper={`${normalizeMissingSteps(result.readiness.missing_steps).length} checklist items remain`} />
                </div>
                {result.tenant.billing_provider_connection_id ? (
                  <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                    Active billing connection
                    <p className="mt-2 break-all font-mono text-xs text-slate-500">{result.tenant.billing_provider_connection_id}</p>
                  </div>
                ) : null}
                <div className="mt-5 flex flex-col gap-3">
                  <Link href={`/workspaces/${encodeURIComponent(result.tenant.id)}`} className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                    View workspace detail
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link href="/workspaces" className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                    Open workspace directory
                  </Link>
                </div>
              </section>
            ) : (
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">What changes now</p>
                <div className="mt-4 space-y-3 text-sm text-slate-600">
                  <p>Billing connections now own Stripe secret storage and provider sync.</p>
                  <p>Workspace setup can leave billing unassigned until the workspace exists.</p>
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
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Step {index}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm text-slate-600">{body}</p>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-white px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value}</p>
    </div>
  );
}
