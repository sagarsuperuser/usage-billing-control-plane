"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchTenantAuditEvents, fetchTenants } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";
import { type TenantAuditEvent } from "@/lib/types";

const DEFAULT_LIMIT = 50;

export function TenantAuditScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [tenantID, setTenantID] = useState("");
  const [action, setAction] = useState("");
  const [actorAPIKeyID, setActorAPIKeyID] = useState("");

  const tenantsQuery = useQuery({
    queryKey: ["platform-tenants", apiBaseURL],
    queryFn: () => fetchTenants({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const auditQuery = useQuery({
    queryKey: ["tenant-audit", apiBaseURL, tenantID, action, actorAPIKeyID],
    queryFn: () =>
      fetchTenantAuditEvents({
        runtimeBaseURL: apiBaseURL,
        tenantID: tenantID || undefined,
        action: action || undefined,
        actorAPIKeyID: actorAPIKeyID || undefined,
        limit: DEFAULT_LIMIT,
        offset: 0,
      }),
    enabled: isAuthenticated && isPlatformAdmin,
  });

  const actionOptions = useMemo(() => {
    const values = new Set<string>();
    for (const item of auditQuery.data?.items ?? []) {
      if (item.action) values.add(item.action);
    }
    return Array.from(values).sort();
  }, [auditQuery.data]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Overview" }, { label: "Tenant audit" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && (scope !== "platform" || !isPlatformAdmin) ? (
          <ScopeNotice
            title="Platform session required"
            body="Tenant audit is a platform operator surface. Sign in with a platform admin session to inspect cross-workspace changes."
            actionHref="/control-plane"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Platform audit</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Tenant audit trail</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">
            Inspect tenant-level changes such as onboarding, workspace billing updates, payment setup requests, and access operations from one operator surface.
          </p>
        </section>

        <section className="grid gap-4 md:grid-cols-4">
          <MetricCard label="Events loaded" value={String(auditQuery.data?.items.length ?? 0)} />
          <MetricCard label="Matching tenants" value={tenantID ? "1" : String(tenantsQuery.data?.length ?? 0)} />
          <MetricCard label="Actions" value={String(actionOptions.length)} />
          <MetricCard label="Result window" value={String(auditQuery.data?.limit ?? DEFAULT_LIMIT)} />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.7fr)_minmax(0,0.9fr)]">
            <label className="grid gap-2 text-sm text-slate-700">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Tenant</span>
              <select
                value={tenantID}
                onChange={(event) => setTenantID(event.target.value)}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="">All tenants</option>
                {(tenantsQuery.data ?? []).map((tenant) => (
                  <option key={tenant.id} value={tenant.id}>
                    {tenant.id}
                  </option>
                ))}
              </select>
            </label>
            <label className="grid gap-2 text-sm text-slate-700">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Action</span>
              <input
                value={action}
                onChange={(event) => setAction(event.target.value)}
                list="tenant-audit-actions"
                placeholder="created, payment_setup_requested"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <datalist id="tenant-audit-actions">
                {actionOptions.map((item) => (
                  <option key={item} value={item} />
                ))}
              </datalist>
            </label>
            <label className="grid gap-2 text-sm text-slate-700">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Actor API key</span>
              <input
                value={actorAPIKeyID}
                onChange={(event) => setActorAPIKeyID(event.target.value)}
                placeholder="apk_platform_admin"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </label>
          </div>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-2">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Recent tenant events</p>
            <h2 className="text-xl font-semibold text-slate-950">Operator-visible audit history</h2>
          </div>

          <div className="mt-5 grid gap-3">
            {auditQuery.isLoading ? <LoadingState /> : null}
            {!auditQuery.isLoading && (auditQuery.data?.items.length ?? 0) === 0 ? <EmptyState /> : null}
            {(auditQuery.data?.items ?? []).map((event) => (
              <AuditRow key={event.id} event={event} />
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}

function AuditRow({ event }: { event: TenantAuditEvent }) {
  const entries = Object.entries(event.metadata ?? {}).sort(([left], [right]) => left.localeCompare(right));
  return (
    <article className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
              {event.action}
            </span>
            <span className="text-xs font-mono text-slate-500">{event.id}</span>
          </div>
          <p className="mt-2 text-sm text-slate-700">
            Tenant <span className="font-medium text-slate-950">{event.tenant_id}</span>
            {event.actor_api_key_id ? (
              <>
                {" "}
                via <span className="font-mono text-slate-600">{event.actor_api_key_id}</span>
              </>
            ) : null}
          </p>
        </div>
        <p className="shrink-0 text-sm text-slate-500">{new Date(event.created_at).toLocaleString()}</p>
      </div>
      {entries.length > 0 ? (
        <dl className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {entries.map(([key, value]) => (
            <div key={key} className="rounded-lg border border-slate-200 bg-white px-3 py-3">
              <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{key}</dt>
              <dd className="mt-2 break-words text-sm text-slate-800">{formatMetadataValue(value)}</dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="mt-4 text-sm text-slate-500">No metadata attached.</p>
      )}
    </article>
  );
}

function formatMetadataValue(value: unknown): string {
  if (value === null || value === undefined) {
    return "-";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading tenant audit events
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No tenant audit events match the current filters.</p>
      <p className="mt-2">Try widening the tenant, action, or actor filters.</p>
    </div>
  );
}
