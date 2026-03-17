"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
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
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "pending":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
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
    if (!term) return tenants;
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
      missingBilling: filteredTenants.filter(
        (tenant) => !tenant.billing_provider_connection_id && (!tenant.lago_organization_id || !tenant.lago_billing_provider_code)
      ).length,
    };
  }, [filteredTenants, readinessByTenant]);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-amber-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Workspaces" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Platform Directory</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Workspaces</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Browse workspace health, linked billing connections, and next actions. Creation now lives in a dedicated setup flow.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/billing-connections"
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Billing connections
              </Link>
              <Link
                href="/workspaces/new"
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
              >
                <Plus className="h-4 w-4" />
                New workspace
              </Link>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace directory is a platform-admin view. Sign in with a platform_admin API key to browse cross-workspace readiness."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Visible workspaces" value={summary.total} />
          <MetricCard label="Ready" value={summary.ready} tone="success" />
          <MetricCard label="Needs attention" value={summary.needsAttention} tone="warn" />
          <MetricCard label="Missing billing" value={summary.missingBilling} tone="warn" />
        </section>

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace list</p>
              <h2 className="mt-2 text-2xl font-semibold text-white">Browse and inspect</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by name or workspace ID"
                className="h-11 min-w-[260px] rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="active">Active</option>
                <option value="suspended">Suspended</option>
                <option value="deleted">Deleted</option>
              </select>
            </div>
          </div>

          <div className="mt-5 grid gap-3">
            {tenantsQuery.isLoading ? (
              <LoadingState />
            ) : filteredTenants.length === 0 ? (
              <EmptyState />
            ) : (
              filteredTenants.map((tenant) => (
                <WorkspaceRow key={tenant.id} tenant={tenant} readiness={readinessByTenant.get(tenant.id)} />
              ))
            )}
          </div>
        </section>
      </main>
    </div>
  );
}

function WorkspaceRow({ tenant, readiness }: { tenant: Tenant; readiness?: TenantOnboardingReadiness }) {
  const nextStep = readiness?.missing_steps[0];
  return (
    <Link
      href={`/workspaces/${encodeURIComponent(tenant.id)}`}
      className="grid gap-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4 transition hover:border-cyan-400/40 hover:bg-cyan-500/5 lg:grid-cols-[minmax(0,1.1fr)_repeat(3,minmax(0,0.55fr))_auto] lg:items-center"
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="truncate text-lg font-semibold text-white">{tenant.name}</h3>
          <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${statusTone(tenant.status)}`}>
            {tenant.status}
          </span>
        </div>
        <p className="mt-1 break-all font-mono text-xs text-slate-400">{tenant.id}</p>
        <p className="mt-2 text-sm text-slate-300">
          {nextStep ? `Next action: ${formatStep(nextStep)}` : "All major onboarding checkpoints are complete."}
        </p>
      </div>
      <StatusCell label="Overall" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
      <StatusCell label="Billing" value={tenant.billing_provider_connection_id ? "Linked" : readiness?.billing_integration.billing_mapping_ready ? "Mapped" : "Missing"} />
      <StatusCell label="First customer" value={readiness?.first_customer.customer_exists ? "Created" : "Missing"} />
      <StatusCell label="Pricing" value={readiness?.billing_integration.pricing_ready ? "Ready" : "Missing"} />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-cyan-100">
        Open
        <ChevronRight className="h-4 w-4" />
      </span>
    </Link>
  );
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 text-sm font-semibold text-white">{value}</p>
    </div>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "warn" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "warn" ? "text-amber-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-6 text-sm text-slate-300">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading workspace inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-2xl border border-dashed border-white/10 bg-slate-950/40 px-5 py-8 text-sm text-slate-300">
      <p className="font-semibold text-white">No workspaces match the current filters.</p>
      <p className="mt-2 text-slate-400">Clear filters or create a new workspace if you are bootstrapping a fresh tenant.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/workspaces/new" className="inline-flex h-10 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-cyan-100 transition hover:bg-cyan-500/20">Create workspace</Link>
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
