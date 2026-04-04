"use client";

import { useEffect, useMemo, useState } from "react";
import {
  ChevronDown,
  ChevronUp,
  Download,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { StatusChip } from "@/components/ui/status-chip";
import { CursorPagination } from "@/components/ui/mini-pagination";

import {
  fetchTenantWorkspaceServiceAccountAudit,
  fetchTenantWorkspaceServiceAccountAuditExports,
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
  const [auditExportPage, setAuditExportPage] = useState(1);
  const [auditExportCursor, setAuditExportCursor] = useState<string | undefined>(undefined);
  const [auditExportCursorHistory, setAuditExportCursorHistory] = useState<Array<string | undefined>>([]);
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [showExports, setShowExports] = useState(false);

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

  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    setSelectedAuditEventID("");
    setAuditPage(1);
    setAuditCursor(undefined);
    setAuditCursorHistory([]);
    setAuditExportPage(1);
    setAuditExportCursor(undefined);
    setAuditExportCursorHistory([]);
  }, [selectedAuditServiceAccountIDValue]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const serviceAccountAuditQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAudit({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 8,
        cursor: auditCursor,
      }),
    enabled: Boolean(session) && selectedAuditServiceAccountIDValue !== "",
  });
  const serviceAccountAuditExportsQuery = useQuery({
    queryKey: ["tenant-workspace-service-account-audit-exports", apiBaseURL, session?.tenant_id, selectedAuditServiceAccountIDValue],
    queryFn: () =>
      fetchTenantWorkspaceServiceAccountAuditExports({
        runtimeBaseURL: apiBaseURL,
        serviceAccountID: selectedAuditServiceAccountIDValue,
        limit: 4,
        cursor: auditExportCursor,
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
  const auditExportItems = serviceAccountAuditExportsQuery.data?.items ?? [];
  const selectedAuditEventIDValue =
    selectedAuditEventID && auditItems.some((item) => item.id === selectedAuditEventID) ? selectedAuditEventID : "";
  const selectedAuditEvent = auditItems.find((item) => item.id === selectedAuditEventIDValue) ?? null;

  const auditHasPreviousPage = auditCursorHistory.length > 0;
  const auditHasNextPage = Boolean(serviceAccountAuditQuery.data?.next_cursor);
  const auditExportHasPreviousPage = auditExportCursorHistory.length > 0;
  const auditExportHasNextPage = Boolean(serviceAccountAuditExportsQuery.data?.next_cursor);

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

  const goToNextAuditExportPage = () => {
    const nextCursor = serviceAccountAuditExportsQuery.data?.next_cursor;
    if (!nextCursor) return;
    setAuditExportCursorHistory((c) => [...c, auditExportCursor]);
    setAuditExportCursor(nextCursor);
    setAuditExportPage((c) => c + 1);
  };

  const goToPreviousAuditExportPage = () => {
    if (auditExportCursorHistory.length === 0) return;
    setAuditExportCursorHistory((c) => c.slice(0, -1));
    setAuditExportCursor(auditExportCursorHistory[auditExportCursorHistory.length - 1]);
    setAuditExportPage((c) => Math.max(1, c - 1));
  };

  const downloadAuditCSV = (serviceAccountID: string) => {
    const path = `/v1/workspace/service-accounts/${encodeURIComponent(serviceAccountID)}/audit`;
    const url = new URL(path, apiBaseURL || window.location.origin);
    url.searchParams.set("limit", "500");
    url.searchParams.set("format", "csv");
    const link = document.createElement("a");
    link.href = url.toString();
    link.download = `service-account-${serviceAccountID}-audit.csv`;
    document.body.appendChild(link);
    link.click();
    link.remove();
  };

  /* --- Render ----------------------------------------------------- */

  return (
    <div className="divide-y divide-stone-200">
      {/* Header: filters */}
      <div className="flex flex-wrap items-center gap-2 px-5 py-3">
        <select
          aria-label="Audit service account"
          value={selectedAuditServiceAccountIDValue}
          onChange={(event) => setSelectedAuditServiceAccountID(event.target.value)}
          className="h-7 rounded border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-1"
        >
          {serviceAccounts.map((account) => (
            <option key={account.id} value={account.id}>{account.name}</option>
          ))}
        </select>
        <input type="date" value={fromDate} onChange={(e) => setFromDate(e.target.value)} aria-label="From date" className="h-7 rounded border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-1" />
        <input type="date" value={toDate} onChange={(e) => setToDate(e.target.value)} aria-label="To date" className="h-7 rounded border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-1" />
        <select value={actionFilter} onChange={(e) => setActionFilter(e.target.value)} aria-label="Action type filter" className="h-7 rounded border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-1">
          <option value="">All actions</option>
          <option value="created">Issued</option>
          <option value="rotated">Rotated</option>
          <option value="revoked">Revoked</option>
        </select>
        <div className="ml-auto flex items-center gap-2">
          <CursorPagination page={auditPage} hasPrevious={auditHasPreviousPage} hasNext={auditHasNextPage} onPrevious={goToPreviousAuditPage} onNext={goToNextAuditPage} label="Audit events" />
          <button
            type="button"
            onClick={() => downloadAuditCSV(selectedAuditServiceAccountIDValue)}
            disabled={!selectedAuditServiceAccountIDValue}
            className="inline-flex h-7 items-center gap-1.5 rounded border border-stone-200 bg-white px-2 text-xs font-medium text-slate-600 transition hover:bg-stone-100 disabled:opacity-50"
          >
            <Download className="h-3 w-3" />
            Export
          </button>
        </div>
      </div>

      {selectedAuditServiceAccount ? (
        <>
          {/* Events table */}
          {auditItems.length > 0 ? (
            <div>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-medium uppercase tracking-wider text-slate-400">
                    <th className="px-5 py-2 font-semibold">When</th>
                    <th className="px-4 py-2 font-semibold">Action</th>
                    <th className="px-4 py-2 font-semibold">Summary</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {auditItems.map((event) => {
                    const presentation = describeAuditEvent(event);
                    const selected = event.id === selectedAuditEventIDValue;
                    return (
                      <tr
                        key={event.id}
                        onClick={() => setSelectedAuditEventID(event.id === selectedAuditEventIDValue ? "" : event.id)}
                        aria-label={`View service account audit details for ${presentation.title}`}
                        className={`cursor-pointer transition ${selected ? "bg-sky-50/70" : "hover:bg-stone-50"}`}
                      >
                        <td className="whitespace-nowrap px-5 py-2.5 text-xs text-slate-500" title={formatExactTimestamp(event.created_at)}>{formatRelativeTimestamp(event.created_at)}</td>
                        <td className="px-4 py-2.5">
                          <StatusChip tone={event.action === "revoked" ? "danger" : event.action === "rotated" ? "warning" : "success"}>
                            {formatAuditActionLabel(event.action)}
                          </StatusChip>
                        </td>
                        <td className="px-4 py-2.5 text-xs text-slate-700">{presentation.summary}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>

              {/* Inline event detail */}
              {selectedAuditEvent && (
                <div className="border-t border-stone-200 bg-stone-50/50 px-5 py-3.5">
                  <AuditEventDetail event={selectedAuditEvent} />
                </div>
              )}
            </div>
          ) : (
            <p className="px-5 py-8 text-center text-xs text-slate-400">No audit events found.</p>
          )}

          {/* Exports — collapsed */}
          <div>
            <button
              type="button"
              onClick={() => setShowExports(!showExports)}
              className="flex w-full items-center justify-between px-5 py-2.5 text-left transition hover:bg-stone-50"
            >
              <p className="text-xs font-semibold text-slate-700">Exports</p>
              {showExports ? <ChevronUp className="h-3.5 w-3.5 text-slate-400" /> : <ChevronDown className="h-3.5 w-3.5 text-slate-400" />}
            </button>
            {showExports && (
              <div className="border-t border-stone-100">
                <div className="flex items-center justify-end px-5 py-2">
                  <CursorPagination page={auditExportPage} hasPrevious={auditExportHasPreviousPage} hasNext={auditExportHasNextPage} onPrevious={goToPreviousAuditExportPage} onNext={goToNextAuditExportPage} label="Audit exports" />
                </div>
                <div className="divide-y divide-stone-100">
                  {auditExportItems.length > 0 ? (
                    auditExportItems.map((item) => (
                      <div key={item.job.id} className="flex items-center justify-between gap-3 px-5 py-2.5">
                        <div className="flex items-center gap-2">
                          <StatusChip tone={item.download_url ? "success" : item.job.status === "failed" ? "danger" : "info"}>
                            {formatAuditExportStatus(item.job.status)}
                          </StatusChip>
                          <span className="text-xs text-slate-500">{formatRelativeTimestamp(item.job.created_at)} · {item.job.row_count} rows</span>
                        </div>
                        {item.download_url ? (
                          <a href={item.download_url} target="_blank" rel="noreferrer" className="inline-flex h-6 items-center gap-1 rounded border border-stone-200 bg-white px-2 text-[11px] font-medium text-slate-600 transition hover:bg-stone-100">
                            <Download className="h-2.5 w-2.5" />
                            Download
                          </a>
                        ) : null}
                      </div>
                    ))
                  ) : (
                    <p className="px-5 py-4 text-xs text-slate-400">No exports yet.</p>
                  )}
                </div>
              </div>
            )}
          </div>
        </>
      ) : (
        <p className="px-5 py-8 text-center text-xs text-slate-400">Create a service account first to view audit events.</p>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function AuditEventDetail({ event }: { event: APIKeyAuditEvent }) {
  const metadata = event.metadata ?? {};
  const presentation = describeAuditEvent(event);
  const role = readMeta(metadata, "role");
  const environment = readMeta(metadata, "environment");

  return (
    <dl className="grid gap-x-6 gap-y-2 sm:grid-cols-3">
      <div>
        <dt className="text-[11px] font-medium text-slate-400">What happened</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{presentation.title}</dd>
      </div>
      <div>
        <dt className="text-[11px] font-medium text-slate-400">When</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{formatExactTimestamp(event.created_at)}</dd>
      </div>
      <div>
        <dt className="text-[11px] font-medium text-slate-400">Actor</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{event.actor_api_key_id ? "API key" : "Console"}</dd>
      </div>
      {role ? (
        <div>
          <dt className="text-[11px] font-medium text-slate-400">Role</dt>
          <dd className="mt-0.5 text-sm text-slate-900">{formatRole(role)}</dd>
        </div>
      ) : null}
      {environment ? (
        <div>
          <dt className="text-[11px] font-medium text-slate-400">Environment</dt>
          <dd className="mt-0.5 text-sm text-slate-900">{environment}</dd>
        </div>
      ) : null}
    </dl>
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

function formatAuditExportStatus(status: string): string {
  return status.split("_").filter(Boolean).map((s) => s.charAt(0).toUpperCase() + s.slice(1)).join(" ");
}
