"use client";

import { useEffect, useMemo, useState } from "react";
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

  const selectedEvent =
    (auditQuery.data?.items ?? []).find((item) => item.id === selectedEventID) ?? null;

  useEffect(() => {
    const items = auditQuery.data?.items ?? [];
    if (items.length === 0) {
      setSelectedEventID("");
      return;
    }
    if (selectedEventID && !items.some((item) => item.id === selectedEventID)) {
      setSelectedEventID("");
    }
  }, [auditQuery.data, selectedEventID]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Overview" }, { label: "Workspace audit" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && (scope !== "platform" || !isPlatformAdmin) ? (
          <ScopeNotice
            title="Platform session required"
            body="Workspace audit is a platform operator surface. Sign in with a platform admin session to inspect cross-workspace changes."
            actionHref="/control-plane"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Platform audit</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Workspace audit trail</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">
            Review workspace, billing, customer, and access changes from one operator surface.
          </p>
        </section>

        <section className="grid gap-4 md:grid-cols-4">
          <MetricCard label="Events loaded" value={String(auditQuery.data?.items.length ?? 0)} />
          <MetricCard label="Matching workspaces" value={tenantID ? "1" : String(tenantsQuery.data?.length ?? 0)} />
          <MetricCard label="Actions" value={String(actionOptions.length)} />
          <MetricCard label="Result window" value={String(auditQuery.data?.limit ?? DEFAULT_LIMIT)} />
        </section>

        <section className="grid gap-3 xl:grid-cols-3">
          <OperatorCard title="Operator posture" body="Use this surface to review cross-workspace changes without opening each workspace separately." />
          <OperatorCard title="Filter rule" body="Start broad, then narrow by workspace or event code only when the event stream is too large to review safely." />
          <OperatorCard title="Detail rule" body="Rows should explain the business action first. Use metadata only when the operator needs the exact before-and-after record." />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.7fr)_minmax(0,0.9fr)]">
            <label className="grid gap-2 text-sm text-slate-700">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Workspace</span>
              <select
                value={tenantID}
                onChange={(event) => setTenantID(event.target.value)}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="">All workspaces</option>
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
                placeholder="workspace.created"
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
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Recent workspace events</p>
            <h2 className="text-xl font-semibold text-slate-950">Audit history</h2>
            <p className="text-sm text-slate-600">Select an event to inspect the full change record.</p>
          </div>

          <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(320px,0.85fr)]">
            {auditQuery.isLoading ? <LoadingState /> : null}
            {!auditQuery.isLoading && (auditQuery.data?.items.length ?? 0) === 0 ? <EmptyState /> : null}
            {!auditQuery.isLoading && (auditQuery.data?.items.length ?? 0) > 0 ? (
              <>
                <div className="grid gap-3">
                  {(auditQuery.data?.items ?? []).map((event) => (
                    <AuditRow
                      key={event.id}
                      event={event}
                      workspaceName={workspaceNames.get(event.tenant_id)}
                      selected={event.id === selectedEventID}
                      onSelect={() => setSelectedEventID(event.id)}
                    />
                  ))}
                </div>
                <AuditDetail event={selectedEvent} workspaceName={selectedEvent ? workspaceNames.get(selectedEvent.tenant_id) : undefined} />
              </>
            ) : null}
          </div>
        </section>
      </main>
    </div>
  );
}

function AuditRow({
  event,
  workspaceName,
  selected,
  onSelect,
}: {
  event: TenantAuditEvent;
  workspaceName?: string;
  selected: boolean;
  onSelect: () => void;
}) {
  const metadataCount = Object.keys(event.metadata ?? {}).length;
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`View details for ${event.event_title} on ${event.tenant_id}`}
      className={`w-full rounded-xl border p-4 text-left transition ${
        selected
          ? "border-emerald-300 bg-emerald-50/60 shadow-sm"
          : "border-slate-200 bg-slate-50 hover:border-slate-300 hover:bg-white"
      }`}
    >
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
              {event.event_category}
            </span>
            <span className="text-[11px] text-slate-500">{metadataCount} field{metadataCount === 1 ? "" : "s"}</span>
          </div>
          <p className="mt-2 text-base font-semibold text-slate-950">{event.event_title}</p>
          <p className="mt-1 text-sm leading-relaxed text-slate-600">{event.event_summary}</p>
          <p className="mt-2 text-sm text-slate-700">
            <span className="font-medium text-slate-950">{workspaceName || event.tenant_id}</span>
            {workspaceName ? <> · <span className="font-mono text-slate-600">{event.tenant_id}</span></> : null}
            {event.actor_api_key_id ? (
              <>
                {" "}
                · <span className="font-mono text-slate-600">{event.actor_api_key_id}</span>
              </>
            ) : null}
          </p>
        </div>
        <div className="shrink-0 text-right">
          <p className="text-sm text-slate-500">{new Date(event.created_at).toLocaleString()}</p>
          <p className="mt-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">View details</p>
        </div>
      </div>
    </button>
  );
}

function AuditDetail({ event, workspaceName }: { event: TenantAuditEvent | null; workspaceName?: string }) {
  const entries = Object.entries(event?.metadata ?? {}).sort(([left], [right]) => left.localeCompare(right));

  if (!event) {
    return (
      <aside className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
        Select an audit event to inspect the full metadata.
      </aside>
    );
  }

  return (
    <aside className="rounded-xl border border-slate-200 bg-slate-50 p-5">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Event detail</p>
      <div className="mt-4 rounded-lg border border-slate-200 bg-white px-4 py-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{event.event_category}</p>
        <h3 className="mt-2 text-lg font-semibold text-slate-950">{event.event_title}</h3>
        <p className="mt-2 text-sm leading-relaxed text-slate-600">{event.event_summary}</p>
      </div>
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <DetailField label="Event code" value={event.event_code} mono />
        <DetailField label="Workspace" value={workspaceName || "-"} />
        <DetailField label="Workspace ID" value={event.tenant_id} mono />
        <DetailField label="Actor API key" value={event.actor_api_key_id || "-"} mono />
        <DetailField label="Created at" value={new Date(event.created_at).toLocaleString()} />
        <DetailField label="Event ID" value={event.id} mono className="sm:col-span-2" />
      </div>
      {entries.length > 0 ? (
        <dl className="mt-4 grid gap-3 sm:grid-cols-2">
          {entries.map(([key, value]) => (
            <div key={key} className="rounded-lg border border-slate-200 bg-white px-3 py-3">
              <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{formatMetadataLabel(key)}</dt>
              <dd className="mt-2 break-words text-sm text-slate-800">{formatMetadataValue(value)}</dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="mt-4 text-sm text-slate-500">No metadata attached.</p>
      )}
    </aside>
  );
}

function DetailField({
  label,
  value,
  mono,
  className = "",
}: {
  label: string;
  value: string;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border border-slate-200 bg-white px-3 py-3 ${className}`.trim()}>
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 break-words text-sm text-slate-800 ${mono ? "font-mono" : ""}`.trim()}>{value}</p>
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

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading workspace audit events
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No workspace audit events match the current filters.</p>
      <p className="mt-2">Try widening the workspace, action, or actor filters.</p>
    </div>
  );
}
