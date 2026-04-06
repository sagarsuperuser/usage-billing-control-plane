
import { Button } from "@/components/ui/button";
import { useEffect, useMemo, useState } from "react";
import { Download, X } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { StatusChip } from "@/components/ui/status-chip";
import { CursorPagination } from "@/components/ui/mini-pagination";
import { DatePicker } from "@/components/ui/date-picker";

import {
  fetchTenantWorkspaceServiceAccountAudit,
  fetchTenantWorkspaceServiceAccounts,
} from "@/lib/api";
import { formatExactTimestamp, formatRelativeTimestamp } from "@/lib/format";
import { type APIKeyAuditEvent } from "@/lib/types";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

export interface WorkspaceAuditTabProps {
  apiBaseURL: string;
  csrfToken: string;
  isAdmin: boolean;
  session: { tenant_id?: string; subject_id?: string } | null;
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function WorkspaceAuditTab({ apiBaseURL, session }: WorkspaceAuditTabProps) {
  const [selectedAuditServiceAccountID, setSelectedAuditServiceAccountID] = useState("");
  const [selectedAuditEventID, setSelectedAuditEventID] = useState("");
  const [auditPage, setAuditPage] = useState(1);
  const [auditCursor, setAuditCursor] = useState<string | undefined>(undefined);
  const [auditCursorHistory, setAuditCursorHistory] = useState<Array<string | undefined>>([]);
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [actionFilter, setActionFilter] = useState("");

  /* --- Queries ---------------------------------------------------- */

  const serviceAccountsQuery = useQuery({
    queryKey: ["tenant-workspace-service-accounts", apiBaseURL, session?.tenant_id],
    queryFn: () => fetchTenantWorkspaceServiceAccounts({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(session),
  });

  const serviceAccounts = serviceAccountsQuery.data ?? [];
  const selectedAuditServiceAccountIDValue = selectedAuditServiceAccountID || serviceAccounts[0]?.id || "";
  const selectedAuditServiceAccount =
    serviceAccounts.find((item) => item.id === selectedAuditServiceAccountIDValue) ?? serviceAccounts[0] ?? null;

  /* eslint-disable react-hooks/set-state-in-effect -- resetting derived state when the selected service account changes */
  useEffect(() => {
    setSelectedAuditEventID("");
    setAuditPage(1);
    setAuditCursor(undefined);
    setAuditCursorHistory([]);
  }, [selectedAuditServiceAccountIDValue]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const serviceAccountAuditQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAudit({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 10,
        cursor: auditCursor,
      }),
    enabled: Boolean(session) && selectedAuditServiceAccountIDValue !== "",
  });

  const auditItemsRaw = serviceAccountAuditQuery.data?.items ?? [];
  const auditItems = useMemo(() => {
    let items = auditItemsRaw;
    if (fromDate) {
      const from = new Date(fromDate);
      items = items.filter(e => new Date(e.created_at) >= from);
    }
    if (toDate) {
      const to = new Date(toDate);
      to.setDate(to.getDate() + 1);
      items = items.filter(e => new Date(e.created_at) < to);
    }
    if (actionFilter) {
      items = items.filter(e => e.action === actionFilter);
    }
    return items;
  }, [auditItemsRaw, fromDate, toDate, actionFilter]);

  const selectedAuditEvent = auditItems.find((item) => item.id === selectedAuditEventID) ?? null;
  const auditHasPreviousPage = auditCursorHistory.length > 0;
  const auditHasNextPage = Boolean(serviceAccountAuditQuery.data?.next_cursor);

  /* --- Pagination helpers ----------------------------------------- */

  const goToNextAuditPage = () => {
    const nextCursor = serviceAccountAuditQuery.data?.next_cursor;
    if (!nextCursor) return;
    setAuditCursorHistory((c) => [...c, auditCursor]);
    setAuditCursor(nextCursor);
    setAuditPage((c) => c + 1);
    setSelectedAuditEventID("");
  };

  const goToPreviousAuditPage = () => {
    if (auditCursorHistory.length === 0) return;
    setAuditCursorHistory((c) => c.slice(0, -1));
    setAuditCursor(auditCursorHistory[auditCursorHistory.length - 1]);
    setAuditPage((c) => Math.max(1, c - 1));
    setSelectedAuditEventID("");
  };

  const downloadAuditCSV = (serviceAccountID: string) => {
    const path = `/v1/workspace/service-accounts/${encodeURIComponent(serviceAccountID)}/audit`;
    const url = new URL(path, apiBaseURL || window.location.origin);
    url.searchParams.set("limit", "500");
    url.searchParams.set("format", "csv");
    const link = document.createElement("a");
    link.href = url.toString();
    link.download = `audit-${serviceAccountID}.csv`;
    document.body.appendChild(link);
    link.click();
    link.remove();
  };

  /* --- Render ----------------------------------------------------- */

  return (
    <>
      {/* Detail drawer */}
      {selectedAuditEvent && (
        <div className="fixed inset-0 z-40 flex justify-end" onClick={(e) => { if (e.target === e.currentTarget) setSelectedAuditEventID(""); }}>
          <div className="h-full w-full max-w-sm border-l border-border bg-surface shadow-xl overflow-y-auto">
            <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
              <p className="text-sm font-semibold text-text-primary">Event detail</p>
              <button type="button" onClick={() => setSelectedAuditEventID("")} className="inline-flex h-6 w-6 items-center justify-center rounded text-text-faint transition hover:bg-surface-tertiary hover:text-text-secondary">
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
            <AuditEventDetail event={selectedAuditEvent} />
          </div>
        </div>
      )}

      <div className="divide-y divide-border">
        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2 px-5 py-3">
          <select
            aria-label="Audit service account"
            value={selectedAuditServiceAccountIDValue}
            onChange={(event) => setSelectedAuditServiceAccountID(event.target.value)}
            className="h-7 rounded border border-border bg-surface px-2 text-xs text-text-secondary outline-none ring-slate-400 transition focus:ring-1"
          >
            {serviceAccounts.map((account) => (
              <option key={account.id} value={account.id}>{account.name}</option>
            ))}
          </select>
          <DatePicker value={fromDate} onChange={setFromDate} placeholder="From" aria-label="From date" />
          <DatePicker value={toDate} onChange={setToDate} placeholder="To" aria-label="To date" />
          <select value={actionFilter} onChange={(e) => setActionFilter(e.target.value)} aria-label="Action type filter" className="h-7 rounded border border-border bg-surface px-2 text-xs text-text-secondary outline-none ring-slate-400 transition focus:ring-1">
            <option value="">All actions</option>
            <option value="created">Issued</option>
            <option value="rotated">Rotated</option>
            <option value="revoked">Revoked</option>
          </select>
          <div className="ml-auto flex items-center gap-2">
            <CursorPagination page={auditPage} hasPrevious={auditHasPreviousPage} hasNext={auditHasNextPage} onPrevious={goToPreviousAuditPage} onNext={goToNextAuditPage} label="Audit events" />
            <Button
              variant="secondary"
              size="sm"
              onClick={() => downloadAuditCSV(selectedAuditServiceAccountIDValue)}
              disabled={!selectedAuditServiceAccountIDValue}
            >
              <Download className="h-3 w-3" />
              Export
            </Button>
          </div>
        </div>

        {/* Events table */}
        {selectedAuditServiceAccount ? (
          auditItems.length > 0 ? (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border-light text-left text-[11px] font-medium uppercase tracking-wider text-text-faint">
                  <th className="px-5 py-2 font-semibold">When</th>
                  <th className="px-4 py-2 font-semibold">Action</th>
                  <th className="px-4 py-2 font-semibold">Summary</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-light">
                {auditItems.map((event) => {
                  const presentation = describeAuditEvent(event);
                  const selected = event.id === selectedAuditEventID;
                  return (
                    <tr
                      key={event.id}
                      onClick={() => setSelectedAuditEventID(event.id === selectedAuditEventID ? "" : event.id)}
                      aria-label={`View service account audit details for ${presentation.title}`}
                      className={`cursor-pointer transition ${selected ? "bg-sky-50/70" : "hover:bg-surface-secondary"}`}
                    >
                      <td className="whitespace-nowrap px-5 py-2.5 text-xs text-text-muted" title={formatExactTimestamp(event.created_at)}>{formatRelativeTimestamp(event.created_at)}</td>
                      <td className="px-4 py-2.5">
                        <StatusChip tone={event.action === "revoked" ? "danger" : event.action === "rotated" ? "warning" : "success"}>
                          {formatAuditActionLabel(event.action)}
                        </StatusChip>
                      </td>
                      <td className="px-4 py-2.5 text-xs text-text-secondary">{presentation.summary}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : (
            <p className="px-5 py-8 text-center text-xs text-text-faint">No audit events found.</p>
          )
        ) : (
          <p className="px-5 py-8 text-center text-xs text-text-faint">Create a service account first to view audit events.</p>
        )}
      </div>
    </>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function AuditEventDetail({ event }: { event: APIKeyAuditEvent }) {
  const [showRaw, setShowRaw] = useState(false);
  const metadata = event.metadata ?? {};
  const presentation = describeAuditEvent(event);
  const role = readMeta(metadata, "role");
  const environment = readMeta(metadata, "environment");
  const purpose = readMeta(metadata, "purpose");
  const keyID = readMeta(metadata, "api_key_id") || event.api_key_id || "";
  const newKeyID = readMeta(metadata, "new_api_key_id");
  const ownerID = readMeta(metadata, "owner_id");
  const actorKeyID = event.actor_api_key_id || "";

  return (
    <div>
      {/* Header summary */}
      <div className="border-b border-border-light px-5 py-4">
        <div className="flex items-center gap-2">
          <StatusChip tone={event.action === "revoked" ? "danger" : event.action === "rotated" ? "warning" : "success"}>
            {formatAuditActionLabel(event.action)}
          </StatusChip>
          <span className="text-xs text-text-muted">{formatExactTimestamp(event.created_at)}</span>
        </div>
        <p className="mt-2 text-sm text-text-primary">{presentation.title}</p>
        <p className="mt-0.5 text-xs text-text-muted">{presentation.summary}</p>
      </div>

      {/* Structured fields */}
      <div className="divide-y divide-border-light">
        <DetailRow label="Performed by" value={actorKeyID ? `API key ${actorKeyID.slice(0, 12)}...` : "Workspace admin (console)"} />
        {keyID ? <DetailRow label="Affected key" value={keyID} mono /> : null}
        {newKeyID ? <DetailRow label="Replacement key" value={newKeyID} mono /> : null}
        {ownerID ? <DetailRow label="Service account" value={ownerID} mono /> : null}
        {role ? <DetailRow label="Permission level" value={formatRole(role)} /> : null}
        {environment ? <DetailRow label="Environment" value={environment} /> : null}
        {purpose ? <DetailRow label="Purpose" value={purpose} /> : null}
        <DetailRow label="Event ID" value={event.id} mono />
      </div>

      {/* Raw metadata (expandable — Stripe pattern) */}
      {Object.keys(metadata).length > 0 && (
        <div className="border-t border-border">
          <button
            type="button"
            onClick={() => setShowRaw(!showRaw)}
            className="w-full px-5 py-2.5 text-left text-[11px] font-medium text-text-faint transition hover:text-text-muted"
          >
            {showRaw ? "Hide raw metadata" : "View raw metadata"}
          </button>
          {showRaw && (
            <pre className="mx-5 mb-4 max-h-60 overflow-auto rounded border border-border-light bg-surface-secondary p-3 font-mono text-[11px] text-text-secondary">
              {JSON.stringify(metadata, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-4 px-5 py-2.5">
      <span className="shrink-0 text-xs text-text-faint">{label}</span>
      <span className={`text-right text-xs text-text-primary ${mono ? "font-mono text-[11px]" : ""}`}>{value}</span>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function readMeta(metadata: Record<string, unknown>, key: string): string {
  const value = metadata[key];
  return typeof value === "string" ? value.trim() : "";
}

function formatRole(role: string): string {
  return role === "admin" ? "Admin" : role === "writer" ? "Writer" : "Reader";
}

function describeAuditEvent(event: APIKeyAuditEvent): { title: string; summary: string } {
  const purpose = typeof event.metadata?.purpose === "string" ? event.metadata.purpose.trim() : "";
  const environment = typeof event.metadata?.environment === "string" ? event.metadata.environment.trim() : "";
  const context = [purpose, environment].filter(Boolean).join(" \u00b7 ");
  const contextSuffix = context ? ` for ${context}` : "";

  switch (event.action) {
    case "created":
      return { title: "Credential issued", summary: `A new key was issued${contextSuffix}.` };
    case "revoked":
      return { title: "Credential revoked", summary: `A key was revoked${contextSuffix}.` };
    case "rotated":
      return { title: "Credential rotated", summary: `A key was rotated and replaced${contextSuffix}.` };
    default:
      return { title: formatAuditActionLabel(event.action), summary: context ? `Activity recorded for ${context}.` : "Activity recorded." };
  }
}

function formatAuditActionLabel(action: string): string {
  return action.split(/[_\s]+/).filter(Boolean).map((s) => s.charAt(0).toUpperCase() + s.slice(1)).join(" ");
}
