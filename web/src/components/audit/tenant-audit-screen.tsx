"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchTenantAuditEvents, fetchTenants } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";
import { type TenantAuditEvent } from "@/lib/types";

const DEFAULT_LIMIT = 50;

export function TenantAuditScreen() {
  const { apiBaseURL, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;
  const [tenantID, setTenantID] = useState("");
  const [action, setAction] = useState("");
  const [actorAPIKeyID, setActorAPIKeyID] = useState("");
  const [selectedEventID, setSelectedEventID] = useState("");

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
      if (item.event_code) values.add(item.event_code);
    }
    return Array.from(values).sort();
  }, [auditQuery.data]);

  const workspaceNames = useMemo(() => {
    const out = new Map<string, string>();
    for (const item of tenantsQuery.data ?? []) {
      out.set(item.id, item.name);
    }
    return out;
  }, [tenantsQuery.data]);

  const auditItems = auditQuery.data?.items ?? [];
  const selectedEventIDValue =
    selectedEventID && auditItems.some((item) => item.id === selectedEventID) ? selectedEventID : "";
  const selectedEvent = auditItems.find((item) => item.id === selectedEventIDValue) ?? null;

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Overview" }, { label: "Workspace audit" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && (scope !== "platform" || !isPlatformAdmin) ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace audit is for platform admins. Sign in with a platform admin session to inspect cross-workspace changes."
            actionHref="/control-plane"
            actionLabel="Open platform home"
          />
        ) : null}

        {canViewPlatformSurface ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
            {/* ---- Header ---- */}
            <div className="px-5 py-4">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-3 min-w-0">
                  <h1 className="text-base font-semibold text-slate-900">Workspace audit</h1>
                  <span className="text-xs text-slate-500">{auditItems.length} events</span>
                </div>
              </div>
            </div>

            {/* ---- Filters ---- */}
            <div className="px-5 py-4">
              <div className="grid gap-3 sm:grid-cols-3">
                <label className="grid gap-1 text-sm">
                  <span className="text-xs text-slate-400">Workspace</span>
                  <select
                    value={tenantID}
                    onChange={(e) => setTenantID(e.target.value)}
                    className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                  >
                    <option value="">All workspaces</option>
                    {(tenantsQuery.data ?? []).map((tenant) => (
                      <option key={tenant.id} value={tenant.id}>{tenant.id}</option>
                    ))}
                  </select>
                </label>
                <label className="grid gap-1 text-sm">
                  <span className="text-xs text-slate-400">Action</span>
                  <input
                    value={action}
                    onChange={(e) => setAction(e.target.value)}
                    list="tenant-audit-actions"
                    placeholder="workspace.created"
                    className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                  />
                  <datalist id="tenant-audit-actions">
                    {actionOptions.map((item) => <option key={item} value={item} />)}
                  </datalist>
                </label>
                <label className="grid gap-1 text-sm">
                  <span className="text-xs text-slate-400">Actor API key</span>
                  <input
                    value={actorAPIKeyID}
                    onChange={(e) => setActorAPIKeyID(e.target.value)}
                    placeholder="apk_platform_admin"
                    className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                  />
                </label>
              </div>
            </div>

            {/* ---- Table ---- */}
            <div className="overflow-auto">
              <table className="w-full min-w-[700px] text-sm">
                <thead>
                  <tr className="border-b border-stone-200 text-left text-xs text-slate-400">
                    <th className="px-5 py-2 font-medium">Timestamp</th>
                    <th className="px-3 py-2 font-medium">Workspace</th>
                    <th className="px-3 py-2 font-medium">Action</th>
                    <th className="px-3 py-2 font-medium">Actor</th>
                  </tr>
                </thead>
                <tbody>
                  {auditQuery.isLoading ? (
                    <tr>
                      <td colSpan={4} className="px-5 py-6 text-center">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                          Loading workspace audit events
                        </div>
                      </td>
                    </tr>
                  ) : auditItems.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-5 py-6 text-center text-sm text-slate-500">
                        No workspace audit events match the current filters.
                      </td>
                    </tr>
                  ) : (
                    auditItems.map((event) => {
                      const selected = event.id === selectedEventIDValue;
                      return (
                        <tr
                          key={event.id}
                          onClick={() => setSelectedEventID(event.id)}
                          className={`cursor-pointer border-b border-stone-100 transition ${selected ? "bg-slate-50" : "bg-white hover:bg-slate-50"}`}
                        >
                          <td className="px-5 py-3 text-slate-500">{new Date(event.created_at).toLocaleString()}</td>
                          <td className="px-3 py-3">
                            <p className="font-medium text-slate-900">{workspaceNames.get(event.tenant_id) || event.tenant_id}</p>
                          </td>
                          <td className="px-3 py-3">
                            <p className="text-slate-700">{event.event_title}</p>
                            <span className="mt-0.5 inline-flex rounded-full border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-slate-600">
                              {event.event_category}
                            </span>
                          </td>
                          <td className="px-3 py-3 font-mono text-xs text-slate-500">{event.actor_api_key_id || "-"}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>
        ) : null}

        {/* ---- Event detail ---- */}
        {canViewPlatformSurface && selectedEvent ? (
          <AuditDetail event={selectedEvent} workspaceName={workspaceNames.get(selectedEvent.tenant_id)} />
        ) : null}
      </main>
    </div>
  );
}

function AuditDetail({ event, workspaceName }: { event: TenantAuditEvent; workspaceName?: string }) {
  const entries = Object.entries(event.metadata ?? {}).sort(([left], [right]) => left.localeCompare(right));

  return (
    <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
      <div className="px-5 py-4">
        <p className="text-xs font-medium text-slate-400 mb-1">Event detail</p>
        <p className="text-sm font-semibold text-slate-900">{event.event_title}</p>
        <p className="mt-0.5 text-sm text-slate-600">{event.event_summary}</p>
      </div>
      <div className="px-5 py-4">
        <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
          <div>
            <dt className="text-xs text-slate-400">Event code</dt>
            <dd className="mt-0.5 text-sm font-mono text-slate-700">{event.event_code}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-400">Workspace</dt>
            <dd className="mt-0.5 text-sm text-slate-700">{workspaceName || "-"}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-400">Workspace ID</dt>
            <dd className="mt-0.5 text-sm font-mono text-slate-700">{event.tenant_id}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-400">Actor API key</dt>
            <dd className="mt-0.5 text-sm font-mono text-slate-700">{event.actor_api_key_id || "-"}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-400">Created at</dt>
            <dd className="mt-0.5 text-sm text-slate-700">{new Date(event.created_at).toLocaleString()}</dd>
          </div>
          <div>
            <dt className="text-xs text-slate-400">Event ID</dt>
            <dd className="mt-0.5 text-sm font-mono text-slate-700">{event.id}</dd>
          </div>
        </dl>
      </div>
      {entries.length > 0 ? (
        <div className="px-5 py-4">
          <p className="text-xs font-medium text-slate-400 mb-3">Metadata</p>
          <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
            {entries.map(([key, value]) => (
              <div key={key}>
                <dt className="text-xs text-slate-400">{formatMetadataLabel(key)}</dt>
                <dd className="mt-0.5 break-words text-sm text-slate-700">{formatMetadataValue(value)}</dd>
              </div>
            ))}
          </dl>
        </div>
      ) : null}
    </div>
  );
}

function formatMetadataLabel(value: string): string {
  switch (value) {
    case "billing_provider_connection_id":
      return "Billing connection";
    case "previous_billing_provider_connection_id":
      return "Previous billing connection";
    case "new_billing_provider_connection_id":
      return "New billing connection";
    case "lago_organization_id":
    case "new_lago_organization_id":
      return "Billing organization reference";
    case "previous_lago_organization_id":
      return "Previous billing organization reference";
    case "lago_billing_provider_code":
    case "new_lago_billing_provider_code":
      return "Billing provider reference";
    case "previous_lago_billing_provider_code":
      return "Previous billing provider reference";
    case "customer_id":
      return "Customer";
    case "payment_method_type":
      return "Payment method";
  }
  const normalized = value.replaceAll(".", " ").replaceAll("_", " ").trim();
  if (!normalized) {
    return value;
  }
  return normalized.replace(/\b\w/g, (char) => char.toUpperCase());
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
