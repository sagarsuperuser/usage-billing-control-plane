"use client";

import { useMemo, useState } from "react";
import { Building2, KeyRound, LoaderCircle, RefreshCw, ShieldCheck } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchTenantOnboardingStatus, fetchTenants, onboardTenant } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { type Tenant, type TenantOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

const EMPTY_TENANTS: Tenant[] = [];

function readinessTone(status?: string): string {
  return status === "ready"
    ? "border-emerald-400/40 bg-emerald-500/10 text-emerald-100"
    : "border-amber-400/40 bg-amber-500/10 text-amber-100";
}

function tenantStatusTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "active":
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "suspended":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    case "deleted":
      return "border-rose-400/40 bg-rose-500/10 text-rose-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
  }
}

export function TenantOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();

  const [statusFilter, setStatusFilter] = useState("");
  const [selectedTenantID, setSelectedTenantID] = useState("");
  const [tenantID, setTenantID] = useState("");
  const [tenantName, setTenantName] = useState("");
  const [lagoOrganizationID, setLagoOrganizationID] = useState("");
  const [billingProviderCode, setBillingProviderCode] = useState("");
  const [bootstrapAdminKey, setBootstrapAdminKey] = useState(true);
  const [adminKeyName, setAdminKeyName] = useState("");
  const [allowExistingActiveKeys, setAllowExistingActiveKeys] = useState(false);
  const [result, setResult] = useState<TenantOnboardingResult | null>(null);
  const [flash, setFlash] = useState<string | null>(null);

  const tenantsQuery = useQuery({
    queryKey: ["tenants", apiBaseURL, statusFilter],
    queryFn: () => fetchTenants({ runtimeBaseURL: apiBaseURL, status: statusFilter || undefined }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const tenantStatusQuery = useQuery({
    queryKey: ["tenant-onboarding-status", apiBaseURL, selectedTenantID],
    queryFn: () => fetchTenantOnboardingStatus({ runtimeBaseURL: apiBaseURL, tenantID: selectedTenantID }),
    enabled: isAuthenticated && isPlatformAdmin && selectedTenantID.trim().length > 0,
  });

  const onboardMutation = useMutation({
    mutationFn: () =>
      onboardTenant({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          id: tenantID.trim(),
          name: tenantName.trim(),
          lago_organization_id: lagoOrganizationID.trim() || undefined,
          lago_billing_provider_code: billingProviderCode.trim() || undefined,
          bootstrap_admin_key: bootstrapAdminKey,
          admin_key_name: adminKeyName.trim() || undefined,
          allow_existing_active_keys: allowExistingActiveKeys,
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      setSelectedTenantID(payload.tenant.id);
      setTenantID(payload.tenant.id);
      setTenantName(payload.tenant.name);
      setFlash(
        payload.tenant_created
          ? `Tenant ${payload.tenant.id} created and readiness computed.`
          : `Tenant ${payload.tenant.id} reconciled and readiness refreshed.`
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, payload.tenant.id] }),
      ]);
    },
  });

  const tenants = tenantsQuery.data ?? EMPTY_TENANTS;
  const selectedTenant = tenantStatusQuery.data?.tenant ?? tenants.find((item) => item.id === selectedTenantID) ?? null;
  const selectedReadiness = tenantStatusQuery.data?.readiness ?? result?.readiness ?? null;
  const createdSecret = result?.tenant_admin_bootstrap.secret ?? "";

  const topMetrics = useMemo(() => {
    const active = tenants.filter((tenant) => tenant.status === "active").length;
    const pending = tenants.filter((tenant) => tenant.status !== "active").length;
    return { total: tenants.length, active, pending };
  }, [tenants]);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-amber-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Platform Operator Console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Tenant Onboarding</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Create or reconcile tenants, bootstrap the first tenant admin key, and inspect readiness without leaving Alpha.
              </p>
            </div>
            <div className="grid grid-cols-3 gap-3 text-sm">
              <MetricCard label="Tenants" value={topMetrics.total} />
              <MetricCard label="Active" value={topMetrics.active} tone="success" />
              <MetricCard label="Non-active" value={topMetrics.pending} tone="warn" />
            </div>
          </div>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="This screen drives /internal onboarding APIs. Sign in with a platform_admin API key to create tenants or inspect cross-tenant readiness."
          />
        ) : null}

        {flash ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100">
            {flash}
          </section>
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Create or Reconcile</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Tenant bootstrap workflow</h2>
              </div>
              <span className="inline-flex rounded-xl border border-cyan-400/40 bg-cyan-500/10 p-3 text-cyan-100">
                <Building2 className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-6 grid gap-3 md:grid-cols-2">
              <InputField label="Tenant ID" value={tenantID} onChange={setTenantID} placeholder="tenant_acme" />
              <InputField label="Tenant name" value={tenantName} onChange={setTenantName} placeholder="Acme Corp" />
              <InputField
                label="Lago organization ID"
                value={lagoOrganizationID}
                onChange={setLagoOrganizationID}
                placeholder="org_acme"
              />
              <InputField
                label="Billing provider code"
                value={billingProviderCode}
                onChange={setBillingProviderCode}
                placeholder="stripe_default"
              />
              <InputField
                label="Admin key name"
                value={adminKeyName}
                onChange={setAdminKeyName}
                placeholder="bootstrap-admin-tenant_acme"
              />
              <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                <p className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Bootstrap policy</p>
                <label className="mt-3 flex items-center gap-2 text-sm text-slate-200">
                  <input
                    type="checkbox"
                    checked={bootstrapAdminKey}
                    onChange={(event) => setBootstrapAdminKey(event.target.checked)}
                    className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                  />
                  Bootstrap tenant admin key
                </label>
                <label className="mt-3 flex items-center gap-2 text-sm text-slate-200">
                  <input
                    type="checkbox"
                    checked={allowExistingActiveKeys}
                    onChange={(event) => setAllowExistingActiveKeys(event.target.checked)}
                    className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                  />
                  Allow existing active tenant keys
                </label>
              </div>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardMutation.mutate();
                }}
                disabled={!isPlatformAdmin || !csrfToken || onboardMutation.isPending || !tenantID.trim()}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {onboardMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                Run tenant onboarding
              </button>
              <button
                type="button"
                onClick={() => {
                  setTenantID("");
                  setTenantName("");
                  setLagoOrganizationID("");
                  setBillingProviderCode("");
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
                  Bootstrap admin secret
                </div>
                <p className="mt-2 break-all rounded-xl border border-white/10 bg-slate-950/60 px-3 py-3 font-mono text-xs text-amber-50">
                  {createdSecret}
                </p>
                <p className="mt-2 text-xs text-amber-200">This value is shown once. Capture it in the handoff flow now.</p>
              </div>
            ) : null}
          </section>

          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Readiness + Inventory</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Operator view</h2>
              </div>
              <button
                type="button"
                onClick={() => {
                  void Promise.all([
                    tenantsQuery.refetch(),
                    selectedTenantID ? tenantStatusQuery.refetch() : Promise.resolve(),
                  ]);
                }}
                disabled={!isPlatformAdmin || tenantsQuery.isFetching || tenantStatusQuery.isFetching}
                className="inline-flex h-10 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <RefreshCw className={`h-4 w-4 ${(tenantsQuery.isFetching || tenantStatusQuery.isFetching) ? "animate-spin" : ""}`} />
                Refresh
              </button>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-[0.9fr_1.1fr]">
              <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm font-semibold text-white">Tenants</p>
                  <select
                    value={statusFilter}
                    onChange={(event) => setStatusFilter(event.target.value)}
                    className="h-9 rounded-lg border border-white/15 bg-slate-950/70 px-3 text-xs text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  >
                    <option value="">All</option>
                    <option value="active">Active</option>
                    <option value="suspended">Suspended</option>
                    <option value="deleted">Deleted</option>
                  </select>
                </div>
                <div className="mt-3 max-h-[420px] space-y-2 overflow-y-auto pr-1">
                  {tenants.map((tenant) => {
                    const active = tenant.id === selectedTenantID;
                    return (
                      <button
                        key={tenant.id}
                        type="button"
                        onClick={() => setSelectedTenantID(tenant.id)}
                        className={`w-full rounded-2xl border p-3 text-left transition ${
                          active
                            ? "border-cyan-400/50 bg-cyan-500/10"
                            : "border-white/10 bg-white/5 hover:bg-white/10"
                        }`}
                      >
                        <div className="flex items-center justify-between gap-3">
                          <p className="font-semibold text-white">{tenant.name}</p>
                          <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${tenantStatusTone(tenant.status)}`}>
                            {tenant.status}
                          </span>
                        </div>
                        <p className="mt-1 font-mono text-xs text-slate-400">{tenant.id}</p>
                      </button>
                    );
                  })}
                  {!tenantsQuery.isFetching && tenants.length === 0 ? (
                    <p className="rounded-2xl border border-dashed border-white/10 px-4 py-6 text-sm text-slate-400">
                      No tenants loaded for the selected filter.
                    </p>
                  ) : null}
                </div>
              </div>

              <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                {!selectedTenant || !selectedReadiness ? (
                  <div className="rounded-2xl border border-dashed border-white/10 px-4 py-8 text-sm text-slate-400">
                    Select a tenant to inspect readiness and bootstrap output.
                  </div>
                ) : (
                  <>
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Selected tenant</p>
                        <h3 className="mt-1 text-lg font-semibold text-white">{selectedTenant.name}</h3>
                        <p className="font-mono text-xs text-slate-400">{selectedTenant.id}</p>
                      </div>
                      <span className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em] ${readinessTone(selectedReadiness.status)}`}>
                        {selectedReadiness.status}
                      </span>
                    </div>

                    <div className="mt-4 grid gap-3 md:grid-cols-3">
                      <ReadinessCard title="Tenant" readiness={selectedReadiness.tenant.status} missing={selectedReadiness.tenant.missing_steps} />
                      <ReadinessCard
                        title="Billing integration"
                        readiness={selectedReadiness.billing_integration.status}
                        missing={selectedReadiness.billing_integration.missing_steps}
                      />
                      <ReadinessCard
                        title="First customer"
                        readiness={selectedReadiness.first_customer.status}
                        missing={selectedReadiness.first_customer.missing_steps}
                      />
                    </div>

                    <div className="mt-4 rounded-2xl border border-white/10 bg-slate-900/60 p-4 text-sm text-slate-200">
                      <p className="font-semibold text-white">Missing steps</p>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {selectedReadiness.missing_steps.length > 0 ? (
                          selectedReadiness.missing_steps.map((step) => (
                            <span key={step} className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-slate-300">
                              {step}
                            </span>
                          ))
                        ) : (
                          <span className="rounded-full border border-emerald-400/30 bg-emerald-500/10 px-3 py-1 text-xs text-emerald-100">
                            No missing steps
                          </span>
                        )}
                      </div>
                    </div>

                    <dl className="mt-4 grid gap-3 md:grid-cols-2">
                      <MetaItem label="Lago organization" value={selectedTenant.lago_organization_id || "-"} mono />
                      <MetaItem label="Billing provider" value={selectedTenant.lago_billing_provider_code || "-"} mono />
                      <MetaItem label="Created" value={formatExactTimestamp(selectedTenant.created_at)} />
                      <MetaItem label="Updated" value={formatExactTimestamp(selectedTenant.updated_at)} />
                    </dl>
                  </>
                )}
              </div>
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}

function ScopeNotice({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-amber-400/40 bg-amber-500/10 p-4 text-sm text-amber-100">
      <p className="font-semibold text-amber-50">{title}</p>
      <p className="mt-1 text-amber-100/90">{body}</p>
    </section>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-amber-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/50 px-4 py-3">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
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
    <label className="grid gap-2">
      <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-300">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </label>
  );
}

function ReadinessCard({ title, readiness, missing }: { title: string; readiness: string; missing: string[] }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-semibold text-white">{title}</p>
        <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${readinessTone(readiness)}`}>
          {readiness}
        </span>
      </div>
      <p className="mt-3 text-xs text-slate-400">{missing.length} missing step(s)</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <dt className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</dt>
      <dd className={`mt-2 text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value}</dd>
    </div>
  );
}
