"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ArrowRight, Building2, CreditCard, KeyRound, LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchBillingProviderConnections, onboardTenant } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { showError } from "@/lib/toast";
import { type BillingProviderConnection, type TenantOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function connectionLabel(connection: BillingProviderConnection): string {
  return `${connection.display_name} · ${connection.environment} · ${connection.status}`;
}

const schema = z.object({
  tenant_id: z.string().min(1, "Required"),
  tenant_name: z.string().min(1, "Required"),
  billing_provider_connection_id: z.string(),
  bootstrap_admin_key: z.boolean(),
  admin_key_name: z.string(),
  allow_existing_active_keys: z.boolean(),
});

type FormFields = z.infer<typeof schema>;

export function TenantOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;
  const [result, setResult] = useState<TenantOnboardingResult | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: {
      tenant_id: "",
      tenant_name: "",
      billing_provider_connection_id: "",
      bootstrap_admin_key: true,
      admin_key_name: "",
      allow_existing_active_keys: false,
    },
  });

  const watched = watch();

  const billingConnectionsQuery = useQuery({
    queryKey: ["billing-provider-connections", apiBaseURL],
    queryFn: () => fetchBillingProviderConnections({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const connectedBillingConnections = useMemo(
    () => (billingConnectionsQuery.data ?? []).filter((item) => item.status === "connected" && item.scope === "platform"),
    [billingConnectionsQuery.data],
  );

  const selectedBillingConnection = connectedBillingConnections.find((item) => item.id === watched.billing_provider_connection_id) ?? null;

  const onboardMutation = useMutation({
    mutationFn: (data: FormFields) =>
      onboardTenant({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          id: data.tenant_id.trim(),
          name: data.tenant_name.trim(),
          billing_provider_connection_id: data.billing_provider_connection_id || undefined,
          bootstrap_admin_key: data.bootstrap_admin_key,
          admin_key_name: data.admin_key_name.trim() || undefined,
          allow_existing_active_keys: data.allow_existing_active_keys,
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-tenants"] }),
        queryClient.invalidateQueries({ queryKey: ["tenant-onboarding-status", apiBaseURL, payload.tenant.id] }),
      ]);
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Workspace setup failed", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => onboardMutation.mutate(data));
  const busy = isSubmitting || onboardMutation.isPending;
  const createdSecret = result?.tenant_admin_bootstrap.secret ?? "";

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/workspaces", label: "Workspaces" }, { label: "New" }]} />

        {canViewPlatformSurface ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace setup</p>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create workspace</h1>
                <p className="mt-3 max-w-3xl text-sm text-slate-600">
                  Create the workspace, attach billing now or later, and optionally mint the first admin credential.
                </p>
              </div>
              <div className="flex flex-wrap gap-3">
                <Link href="/billing-connections" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open billing connections</Link>
                <Link href="/workspaces" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open workspaces</Link>
              </div>
            </div>
          </section>
        ) : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="This screen drives platform onboarding APIs. Sign in with a platform admin account to create workspaces."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {canViewPlatformSurface && onboardMutation.isSuccess && result ? (
          <section className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {result.tenant_created
              ? `Workspace ${result.tenant.id} created successfully.`
              : `Workspace ${result.tenant.id} updated and readiness refreshed.`}
          </section>
        ) : null}

        {canViewPlatformSurface ? (
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
            <form onSubmit={onSubmit} noValidate>
              <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Operator setup</p>
                    <h2 className="mt-2 text-xl font-semibold text-slate-950">Workspace provisioning</h2>
                    <p className="mt-2 max-w-2xl text-sm text-slate-600">This page creates the workspace. Review readiness and billing from workspace detail.</p>
                  </div>
                  <span className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                    <Building2 className="h-5 w-5" />
                  </span>
                </div>

                <div className="mt-5 grid gap-3 lg:grid-cols-3">
                  <OperatorLine title="Workspace record" body="Create a durable workspace ID and a clear operator-facing name." />
                  <OperatorLine title="Billing handoff" body="Attach a billing connection now only when platform setup is already ready. Otherwise finish it later from workspace detail." />
                  <OperatorLine title="Access bootstrap" body="Create the first admin credential only when the platform needs an immediate machine identity for handoff." />
                </div>

                <div className="mt-5 grid gap-5">
                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Workspace record</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Workspace identity</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-2">
                      <InputField label="Workspace ID" placeholder="tenant_acme" error={errors.tenant_id?.message} {...register("tenant_id")} />
                      <InputField label="Workspace name" placeholder="Acme Corp" error={errors.tenant_name?.message} {...register("tenant_name")} />
                    </div>
                  </section>

                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Billing handoff</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Billing connection</h3>
                    <p className="mt-2 text-sm text-slate-600">You can leave this empty and attach billing after the workspace is created.</p>
                    {billingConnectionsQuery.isLoading ? (
                      <div className="mt-4 flex items-center gap-2 text-sm text-slate-600">
                        <LoaderCircle className="h-4 w-4 animate-spin" />
                        Loading billing connections
                      </div>
                    ) : connectedBillingConnections.length === 0 ? (
                      <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700">
                        No connected billing providers yet. You can still create the workspace.
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
                          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Billing connection</span>
                          <select
                            className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                            {...register("billing_provider_connection_id")}
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
                            </div>
                          </div>
                        ) : null}
                      </div>
                    )}
                  </section>

                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Access bootstrap</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Admin bootstrap service account</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-[1.15fr_0.85fr]">
                      <InputField label="Bootstrap service account name" placeholder="bootstrap-admin-tenant_acme" error={errors.admin_key_name?.message} {...register("admin_key_name")} />
                      <div className="rounded-xl border border-slate-200 bg-white p-4">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Advanced controls</p>
                        <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                          <input type="checkbox" className="h-4 w-4 rounded border-slate-300" {...register("bootstrap_admin_key")} />
                          Generate first admin service account credential
                        </label>
                        <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                          <input type="checkbox" className="h-4 w-4 rounded border-slate-300" {...register("allow_existing_active_keys")} />
                          Allow existing active credentials
                        </label>
                      </div>
                    </div>
                  </section>
                </div>

                <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <ChecklistLine done={(watched.tenant_id ?? "").trim().length > 0} text="Workspace ID is set" />
                    <ChecklistLine done={(watched.tenant_name ?? "").trim().length > 0} text="Workspace name is set" />
                    <ChecklistLine done text={watched.billing_provider_connection_id ? "Billing is selected" : "Billing can be attached later"} />
                    <ChecklistLine done text={connectedBillingConnections.length > 0 ? "A connected billing provider is available" : "Billing can be added later"} />
                  </div>
                </div>

                {errors.root?.message ? (
                  <p className="mt-5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p>
                ) : null}

                <div className="mt-6 flex flex-wrap gap-3">
                  <button
                    type="submit"
                    disabled={!isPlatformAdmin || !csrfToken || busy}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                    Run workspace setup
                  </button>
                  <button
                    type="button"
                    onClick={() => { reset(); setResult(null); onboardMutation.reset(); }}
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
                    <p className="mt-2">Store this one-time credential now.</p>
                    <p className="mt-3 break-all rounded-lg border border-amber-200 bg-white px-3 py-3 font-mono text-xs text-amber-800">{createdSecret}</p>
                  </div>
                ) : null}
              </section>
            </form>

            <aside className="min-w-0 grid gap-5 self-start">
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">After setup</p>
                <h2 className="mt-2 text-xl font-semibold text-slate-950">Next screens</h2>
                <div className="mt-4 grid gap-2">
                  <ChecklistLine done text="Create the workspace here" />
                  <ChecklistLine done text="Open workspace detail" />
                  <ChecklistLine done text="Manage billing from Billing connections" />
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
                      Billing connection
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
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Use workspaces after create</p>
                  <div className="mt-4 space-y-3 text-sm text-slate-600">
                    <p>Billing is attached from workspace detail or Billing connections.</p>
                    <p>You can create the workspace before attaching billing.</p>
                    <p>Use workspace detail for readiness and next steps.</p>
                  </div>
                </section>
              )}
            </aside>
          </div>
        ) : null}
      </main>
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

function OperatorLine({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}

function InputField({ label, error, ...inputProps }: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        {...inputProps}
        aria-invalid={Boolean(error)}
        className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`}
      />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
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
