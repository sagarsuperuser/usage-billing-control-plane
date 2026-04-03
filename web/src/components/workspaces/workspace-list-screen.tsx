"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Skeleton } from "@/components/ui/skeleton";
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

function billingTone(tenant: Tenant): string {
  const label = workspaceBillingInventoryLabel(tenant);
  switch (label) {
    case "Attached":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "Pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "Missing":
    case "Failed":
    case "Disabled":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

function pricingLabel(readiness?: TenantOnboardingReadiness): string {
  if (!readiness) return "—";
  return readiness.billing_integration.pricing_ready ? "Ready" : "Missing";
}

function customersLabel(readiness?: TenantOnboardingReadiness): string {
  if (!readiness) return "—";
  return readiness.first_customer.customer_exists ? "Created" : "Missing";
}

export function WorkspaceListScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;
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

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { label: "Workspaces" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace directory is a platform-admin view. Sign in with a platform session to browse cross-workspace readiness."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {canViewPlatformSurface ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Workspaces{filteredTenants.length > 0 ? ` (${filteredTenants.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Search..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <select
                  value={statusFilter}
                  onChange={(event) => setStatusFilter(event.target.value)}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All statuses</option>
                  <option value="active">Active</option>
                  <option value="suspended">Suspended</option>
                  <option value="deleted">Deleted</option>
                </select>
                <Link href="/workspaces/new" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-3.5 w-3.5" />
                  New
                </Link>
              </div>
            </div>
            {tenantsQuery.isLoading ? (
              <LoadingState />
            ) : filteredTenants.length === 0 ? (
              <EmptyState />
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Name</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Billing</th>
                    <th className="px-4 py-2.5 font-semibold">Pricing</th>
                    <th className="px-4 py-2.5 font-semibold">Customers</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {filteredTenants.map((tenant) => {
                    const readiness = readinessByTenant.get(tenant.id);
                    return (
                      <tr key={tenant.id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link href={`/workspaces/${encodeURIComponent(tenant.id)}`} className="block">
                            <p className="font-medium text-slate-900">{tenant.name}</p>
                            <p className="mt-0.5 font-mono text-xs text-slate-400">{tenant.id}</p>
                          </Link>
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${statusTone(tenant.status)}`}>
                            {tenant.status}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${billingTone(tenant)}`}>
                            {workspaceBillingInventoryLabel(tenant)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-600">{pricingLabel(readiness)}</td>
                        <td className="px-4 py-3 text-slate-600">{customersLabel(readiness)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        ) : null}
      </main>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="divide-y divide-stone-100">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-32" /><Skeleton className="mt-1 h-3 w-20" /></div>
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-3 w-14" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No workspaces</p>
      <p className="text-xs text-slate-500">Create a workspace to get started.</p>
      <Link href="/workspaces/new" className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
        <Plus className="h-3.5 w-3.5" />
        New workspace
      </Link>
    </div>
  );
}
