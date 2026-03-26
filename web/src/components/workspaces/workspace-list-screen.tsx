"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { LoaderCircle, Plus } from "lucide-react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchTenantOnboardingStatus, fetchTenants } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
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
      return "border-stone-300 bg-stone-100 text-slate-700";
  }
}

function workspaceBillingInventoryLabel(tenant: Tenant): string {
  const billing = tenant.workspace_billing;
  if (!billing.configured) {
    return "Missing";
  }
  switch ((billing.diagnosis_code || billing.status || "").toLowerCase()) {
    case "connected":
      return "Attached";
    case "pending":
    case "provisioning":
    case "pending_verification":
      return "Pending";
    case "verification_failed":
      return "Failed";
    case "disabled":
      return "Disabled";
    default:
      return billing.connected ? "Attached" : "Pending";
  }
}

function workspaceAccessLabel(tenant: Tenant): string {
  const billing = tenant.workspace_billing;
  if (!billing.configured) {
    return "Platform setup";
  }
  switch ((billing.diagnosis_code || billing.status || "").toLowerCase()) {
    case "connected":
      return "Handoff ready";
    case "pending":
    case "provisioning":
    case "pending_verification":
      return "Verification pending";
    case "verification_failed":
      return "Platform repair";
    case "disabled":
      return "Blocked";
    default:
      return billing.connected ? "Handoff ready" : "Platform setup";
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
      billingNotReady: filteredTenants.filter((tenant) => !tenant.workspace_billing.connected).length,
    };
  }, [filteredTenants, readinessByTenant]);

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#eef4ef_0%,#f6f2eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Workspaces" }]} />

        <section className="rounded-3xl border border-stone-200 bg-white/92 shadow-[0_18px_50px_rgba(15,23,42,0.06)]">
          <div className="flex flex-col gap-5 p-5 lg:flex-row lg:items-start lg:justify-between lg:p-6">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-500">Workspaces</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Workspace handoff and readiness</h1>
              <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
                This directory should show which workspace is ready, which one is blocked, and which operational step still belongs to the platform before the workspace can run on its own.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/billing-connections"
                className="inline-flex items-center rounded-xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm font-medium text-slate-700 transition hover:bg-white"
              >
                Open billing connections
              </Link>
              <Link
                href="/workspaces/new"
                className="inline-flex items-center gap-2 rounded-xl bg-emerald-700 px-4 py-3 text-sm font-semibold text-white transition hover:bg-emerald-800"
              >
                <Plus className="h-4 w-4" />
                New workspace
              </Link>
            </div>
          </div>
          <div className="grid gap-3 border-t border-stone-200 px-5 py-4 sm:grid-cols-2 xl:grid-cols-4 lg:px-6">
            <MetricCard label="Visible" value={summary.total} />
            <MetricCard label="Ready" value={summary.ready} tone="success" />
            <MetricCard label="Needs attention" value={summary.needsAttention} tone="warn" />
            <MetricCard label="Billing not ready" value={summary.billingNotReady} tone="warn" />
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

        <section className="rounded-3xl border border-stone-200 bg-white/92 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.06)] lg:p-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Directory</p>
              <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Workspace inventory</h2>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search by workspace name or ID"
                className="h-11 min-w-[260px] rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition placeholder:text-slate-500 focus:ring-2"
              />
              <select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="h-11 rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-900 outline-none ring-emerald-300 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="active">Active</option>
                <option value="suspended">Suspended</option>
                <option value="deleted">Deleted</option>
              </select>
            </div>
          </div>

          <div className="mt-5 divide-y divide-stone-200">
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
  const nextStep = normalizeMissingSteps(readiness?.missing_steps)[0];

  return (
    <Link
      href={`/workspaces/${encodeURIComponent(tenant.id)}`}
      className="grid gap-4 py-4 first:pt-0 last:pb-0 lg:grid-cols-[minmax(0,1.35fr)_repeat(4,minmax(0,0.5fr))] lg:items-start"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-base font-semibold text-slate-950">{tenant.name}</p>
          <span className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] ${statusTone(tenant.status)}`}>
            {tenant.status}
          </span>
        </div>
        <p className="mt-1 break-all font-mono text-[11px] text-slate-500">{tenant.id}</p>
        <p className="mt-2 text-sm leading-6 text-slate-600">
          {nextStep ? `Next action: ${formatStep(nextStep)}` : "No immediate blocker is shown."}
        </p>
      </div>
      <StatCell label="Overall" value={readiness ? formatReadinessStatus(readiness.status) : "Loading"} />
      <StatCell label="Billing" value={workspaceBillingInventoryLabel(tenant)} />
      <StatCell label="First customer" value={readiness?.first_customer.customer_exists ? "Created" : "Missing"} />
      <StatCell label="Pricing" value={readiness?.billing_integration.pricing_ready ? "Ready" : "Missing"} />
      <StatCell label="Access" value={workspaceAccessLabel(tenant)} />
    </Link>
  );
}

function StatCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-900">{value}</p>
    </div>
  );
}

function MetricCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "success" | "warn";
}) {
  const toneClass = tone === "success" ? "text-emerald-700" : tone === "warn" ? "text-amber-700" : "text-slate-950";
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 text-3xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading workspace inventory
    </div>
  );
}

function EmptyState() {
  return (
    <div className="py-6 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No workspaces match the current filters.</p>
      <p className="mt-2">Clear filters or create a new workspace if you are bootstrapping a fresh tenant.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/workspaces/new" className="inline-flex h-10 items-center rounded-xl bg-emerald-700 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-emerald-800">Create workspace</Link>
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
