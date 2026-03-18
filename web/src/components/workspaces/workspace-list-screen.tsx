"use client";

import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";
import { ArrowRight, Building2, LoaderCircle, Plus, ShieldCheck } from "lucide-react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchTenantOnboardingStatus, fetchTenants } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { type Tenant, type TenantOnboardingReadiness } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

function statusTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "suspended":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-stone-200 bg-stone-100 text-slate-700";
  }
}

export function WorkspaceListScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");

  const tenantsQuery = useQuery({
    queryKey: ["tenants", apiBaseURL, statusFilter],
    queryFn: () => fetchTenants({ runtimeBaseURL: apiBaseURL, status: statusFilter || undefined }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const filteredTenants = useMemo(() => {
    const tenants = tenantsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (term.length === 0) return tenants;
    return tenants.filter((tenant) => tenant.id.toLowerCase().includes(term) || tenant.name.toLowerCase().includes(term));
  }, [search, tenantsQuery.data]);

  const readinessQueries = useQueries({
    queries: filteredTenants.map((tenant) => ({
      queryKey: ["tenant-onboarding-status", apiBaseURL, tenant.id],
      queryFn: () => fetchTenantOnboardingStatus({ runtimeBaseURL: apiBaseURL, tenantID: tenant.id }),
      enabled: isAuthenticated && isPlatformAdmin,
    })),
  });

  const readinessByTenant = useMemo(() => {
    const map = new Map<string, TenantOnboardingReadiness>();
    readinessQueries.forEach((query) => {
      if (query.data) map.set(query.data.tenant_id, query.data.readiness);
    });
    return map;
  }, [readinessQueries]);

  const summary = useMemo(() => {
    const readiness = filteredTenants.flatMap((tenant) => {
      const item = readinessByTenant.get(tenant.id);
      return item ? [item] : [];
    });
    return {
      total: filteredTenants.length,
      ready: readiness.filter((item) => item.status === "ready").length,
      needsAttention: readiness.filter((item) => item.status !== "ready").length,
      missingBilling: filteredTenants.filter((tenant) => !tenant.workspace_billing.connected).length,
    };
  }, [filteredTenants, readinessByTenant]);

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#dfeee3_0%,#f4ede3_18%,#f7f3eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Workspaces" }]} />

        <section className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700/70">Platform directory</p>
            <h1 className="mt-3 text-4xl font-semibold tracking-tight text-slate-950 md:text-5xl">Workspaces</h1>
            <p className="mt-4 max-w-3xl text-base leading-7 text-slate-600">
              Workspaces are the operational handoff boundary. This directory should answer which workspace is ready, which one is blocked, and what the next ownership step is.
            </p>
            <div className="mt-6 grid gap-3 md:grid-cols-4">
              <MetricCard label="Visible" value={summary.total} />
              <MetricCard label="Ready" value={summary.ready} tone="success" />
              <MetricCard label="Needs attention" value={summary.needsAttention} tone="warn" />
              <MetricCard label="Missing billing" value={summary.missingBilling} tone="warn" />
            </div>
          </div>

          <div className="rounded-[32px] border border-emerald-900/10 bg-[linear-gradient(160deg,#0f3b2f_0%,#1a5a44_100%)] p-6 text-emerald-50 shadow-[0_25px_70px_rgba(15,23,42,0.12)]">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-100/70">Ownership boundary</p>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight">One workspace. One active billing path.</h2>
            <p className="mt-4 text-sm leading-7 text-emerald-50/80">
              Billing connections are reusable assets. Workspace billing is the assignment and readiness layer. Keep those concepts separate and the product stays adaptable.
            </p>
            <div className="mt-6 flex flex-wrap gap-3">
              <Link
                href="/workspaces/new"
                className="inline-flex items-center gap-2 rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-emerald-900 transition hover:bg-emerald-50"
              >
                <Plus className="h-4 w-4" />
                New workspace
              </Link>
              <Link
                href="/billing-connections"
                className="inline-flex items-center gap-2 rounded-2xl border border-white/12 bg-white/8 px-4 py-3 text-sm font-semibold text-emerald-50 transition hover:bg-white/12"
              >
                Open billing connections
              </Link>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace directory is a platform-admin view. Sign in with a platform session to browse cross-workspace readiness."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        <section className="grid gap-4 xl:grid-cols-[0.72fr_1.28fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Operator guide</p>
            <h2 className="mt-3 text-2xl font-semibold tracking-tight text-slate-950">What this directory should make obvious</h2>
            <div className="mt-5 grid gap-3">
              <GuideCard
                icon={<Building2 className="h-5 w-5 text-emerald-700" />}
                title="Which workspace is truly ready?"
                body="Read workspace readiness from billing, pricing, and first-customer status together instead of forcing admins into multiple pages."
              />
              <GuideCard
                icon={<ShieldCheck className="h-5 w-5 text-emerald-700" />}
                title="Where does ownership pass to the workspace?"
                body="A workspace becomes self-operating only after billing is attached and access has been handed off to workspace members."
              />
            </div>
          </div>

          <section className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Workspace catalogue</p>
                <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Browse by readiness and handoff state</h2>
              </div>
              <div className="flex flex-col gap-3 sm:flex-row">
                <input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search by workspace name or ID"
                  className="h-11 min-w-[260px] rounded-2xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition placeholder:text-slate-500 focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  className="h-11 rounded-2xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="active">Active</option>
                  <option value="suspended">Suspended</option>
                  <option value="deleted">Deleted</option>
                </select>
              </div>
            </div>

            <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {tenantsQuery.isLoading ? (
                <LoadingState />
              ) : filteredTenants.length === 0 ? (
                <EmptyState />
              ) : (
                filteredTenants.map((tenant) => (
                  <WorkspaceCard key={tenant.id} tenant={tenant} readiness={readinessByTenant.get(tenant.id)} />
                ))
              )}
            </div>
          </section>
        </section>
      </main>
    </div>
  );
}

function WorkspaceCard({ tenant, readiness }: { tenant: Tenant; readiness?: TenantOnboardingReadiness }) {
  const nextStep = readiness?.missing_steps[0];
  return (
    <Link
      href={`/workspaces/${encodeURIComponent(tenant.id)}`}
      className="rounded-[28px] border border-stone-200 bg-[#fbfaf6] p-5 transition hover:-translate-y-0.5 hover:border-emerald-200 hover:bg-white"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace</p>
          <h3 className="mt-2 text-xl font-semibold tracking-tight text-slate-950">{tenant.name}</h3>
        </div>
        <span className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] ${statusTone(tenant.status)}`}>
          {tenant.status}
        </span>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <DetailPill label="Overall" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
        <DetailPill
          label="Billing"
          value={tenant.workspace_billing.connected ? "Attached" : "Missing"}
        />
        <DetailPill label="First customer" value={readiness?.first_customer.customer_exists ? "Created" : "Missing"} />
        <DetailPill label="Pricing" value={readiness?.billing_integration.pricing_ready ? "Ready" : "Missing"} />
      </div>

      <p className="mt-4 text-sm leading-6 text-slate-600">
        {nextStep ? `Next action: ${formatStep(nextStep)}` : "All major onboarding checkpoints are complete."}
      </p>
      <p className="mt-4 break-all font-mono text-[11px] text-slate-500">{tenant.id}</p>

      <div className="mt-5 inline-flex items-center gap-2 text-sm font-semibold text-emerald-700">
        Open workspace
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
  const toneClass = tone === "success" ? "text-emerald-700" : tone === "warn" ? "text-amber-700" : "text-slate-950";
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
      Loading workspace inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="md:col-span-2 xl:col-span-3 rounded-[24px] border border-dashed border-stone-300 bg-stone-50/70 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No workspaces match the current filters.</p>
      <p className="mt-2">Clear filters or create a new workspace if you are bootstrapping a fresh tenant.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/workspaces/new" className="inline-flex h-10 items-center rounded-2xl bg-emerald-700 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-emerald-800">Create workspace</Link>
      </div>
    </div>
  );
}

function formatStep(step: string): string {
  return step
    .replace(/^tenant\./, "")
    .replace(/^billing_integration\./, "")
    .replace(/^first_customer\./, "")
    .replaceAll("_", " ");
}
