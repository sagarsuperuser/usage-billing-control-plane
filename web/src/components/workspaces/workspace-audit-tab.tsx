"use client";

import { type ReactNode, useEffect, useMemo, useState } from "react";
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Download,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import {
  fetchTenantWorkspaceServiceAccountAudit,
  fetchTenantWorkspaceServiceAccountAuditExports,
  fetchTenantWorkspaceServiceAccounts,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
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
    setAuditCursorHistory((current) => [...current, auditCursor]);
    setAuditCursor(nextCursor);
    setAuditPage((current) => current + 1);
    setSelectedAuditEventID("");
  };

  const goToPreviousAuditPage = () => {
    if (auditCursorHistory.length === 0) return;
    const previousCursor = auditCursorHistory[auditCursorHistory.length - 1];
    setAuditCursorHistory((current) => current.slice(0, -1));
    setAuditCursor(previousCursor);
    setAuditPage((current) => Math.max(1, current - 1));
    setSelectedAuditEventID("");
  };

  const goToNextAuditExportPage = () => {
    const nextCursor = serviceAccountAuditExportsQuery.data?.next_cursor;
    if (!nextCursor) return;
    setAuditExportCursorHistory((current) => [...current, auditExportCursor]);
    setAuditExportCursor(nextCursor);
    setAuditExportPage((current) => current + 1);
  };

  const goToPreviousAuditExportPage = () => {
    if (auditExportCursorHistory.length === 0) return;
    const previousCursor = auditExportCursorHistory[auditExportCursorHistory.length - 1];
    setAuditExportCursorHistory((current) => current.slice(0, -1));
    setAuditExportCursor(previousCursor);
    setAuditExportPage((current) => Math.max(1, current - 1));
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
    <div className="p-6">
      {/* Header: SA dropdown + Export CSV */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-sm font-semibold text-slate-900">Credential audit log</p>
          <p className="text-xs text-slate-500">Credential issue, rotation, and revocation events.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            aria-label="Audit service account"
            value={selectedAuditServiceAccountIDValue}
            onChange={(event) => setSelectedAuditServiceAccountID(event.target.value)}
            className="h-8 rounded-lg border border-stone-200 bg-white px-3 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-2"
          >
            {serviceAccounts.map((account) => (
              <option key={account.id} value={account.id}>{account.name}</option>
            ))}
          </select>
          <input type="date" value={fromDate} onChange={(e) => setFromDate(e.target.value)} aria-label="From date" className="h-8 rounded-lg border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-2" />
          <input type="date" value={toDate} onChange={(e) => setToDate(e.target.value)} aria-label="To date" className="h-8 rounded-lg border border-stone-200 bg-white px-2 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-2" />
          <select value={actionFilter} onChange={(e) => setActionFilter(e.target.value)} aria-label="Action type filter" className="h-8 rounded-lg border border-stone-200 bg-white px-3 text-xs text-slate-800 outline-none ring-slate-400 transition focus:ring-2">
            <option value="">All actions</option>
            <option value="created">Issued</option>
            <option value="rotated">Rotated</option>
            <option value="revoked">Revoked</option>
          </select>
          <button
            type="button"
            onClick={() => downloadAuditCSV(selectedAuditServiceAccountIDValue)}
            disabled={!selectedAuditServiceAccountIDValue}
            className="inline-flex h-8 items-center gap-2 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Download className="h-3.5 w-3.5" />
            Export CSV
          </button>
        </div>
      </div>

      {selectedAuditServiceAccount ? (
        <div className="mt-4 grid gap-4">
          {/* Events table */}
          <div className="overflow-hidden rounded-lg border border-stone-200">
            <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-3">
              <p className="text-sm font-medium text-slate-700">Recent events</p>
              <CursorPaginationControls page={auditPage} hasPreviousPage={auditHasPreviousPage} hasNextPage={auditHasNextPage} onPrevious={goToPreviousAuditPage} onNext={goToNextAuditPage} label="Audit events" />
            </div>
            {auditItems.length > 0 ? (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-xs font-medium text-slate-400">
                    <th className="px-4 py-2.5 font-semibold">Timestamp</th>
                    <th className="px-4 py-2.5 font-semibold">Action</th>
                    <th className="px-4 py-2.5 font-semibold">Summary</th>
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
                        <td className="whitespace-nowrap px-4 py-3 text-xs text-slate-600">{formatExactTimestamp(event.created_at)}</td>
                        <td className="px-4 py-3">
                          <StatusChip tone={event.action === "revoked" ? "danger" : event.action === "rotated" ? "warning" : "success"}>
                            {formatAuditActionLabel(event.action)}
                          </StatusChip>
                        </td>
                        <td className="px-4 py-3 text-slate-700">{presentation.summary}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            ) : (
              <p className="px-4 py-6 text-sm text-slate-500">No audit events yet.</p>
            )}

            {/* Inline event detail */}
            {selectedAuditEvent && (
              <div className="border-t border-stone-200 bg-stone-50/50 px-4 py-4">
                <ServiceAccountAuditDetail event={selectedAuditEvent} />
              </div>
            )}
          </div>

          {/* Exports — collapsed by default */}
          <div className="overflow-hidden rounded-lg border border-stone-200">
            <button
              type="button"
              onClick={() => setShowExports(!showExports)}
              className="flex w-full items-center justify-between bg-stone-50 px-4 py-3 text-left transition hover:bg-stone-100"
            >
              <p className="text-sm font-medium text-slate-700">Exports</p>
              {showExports ? <ChevronUp className="h-4 w-4 text-slate-400" /> : <ChevronDown className="h-4 w-4 text-slate-400" />}
            </button>
            {showExports && (
              <>
                <div className="flex items-center justify-end border-t border-stone-200 bg-stone-50 px-4 py-2">
                  <CursorPaginationControls page={auditExportPage} hasPreviousPage={auditExportHasPreviousPage} hasNextPage={auditExportHasNextPage} onPrevious={goToPreviousAuditExportPage} onNext={goToNextAuditExportPage} label="Audit exports" />
                </div>
                <div className="divide-y divide-stone-100">
                  {auditExportItems.length > 0 ? (
                    auditExportItems.map((item) => (
                      <div key={item.job.id} className="flex items-center justify-between gap-4 px-4 py-3">
                        <div className="flex items-center gap-3">
                          <StatusChip tone={item.download_url ? "success" : item.job.status === "failed" ? "danger" : "info"}>
                            {formatAuditExportStatus(item.job.status)}
                          </StatusChip>
                          <div>
                            <p className="text-xs text-slate-700">{formatExactTimestamp(item.job.created_at)}</p>
                            <p className="text-[11px] text-slate-500">{item.job.row_count} row(s)</p>
                          </div>
                        </div>
                        {item.download_url ? (
                          <a href={item.download_url} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-100">
                            <Download className="h-3 w-3" />
                            Download
                          </a>
                        ) : (
                          <span className="text-xs text-slate-500">Pending</span>
                        )}
                      </div>
                    ))
                  ) : (
                    <p className="px-4 py-6 text-sm text-slate-500">No exports yet.</p>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      ) : (
        <p className="mt-4 text-sm text-slate-500">Create a service account first to inspect machine-credential audit.</p>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function ServiceAccountAuditDetail({ event }: { event: APIKeyAuditEvent }) {
  const metadata = event.metadata ?? {};
  const presentation = describeAuditEvent(event);
  const credentialName = readAuditMetadataString(metadata, "name");
  const role = readAuditMetadataString(metadata, "role");
  const environment = readAuditMetadataString(metadata, "environment");
  const actorLabel = event.actor_api_key_id ? "Credential session" : "Admin session";

  return (
    <dl className="grid gap-x-6 gap-y-2 sm:grid-cols-2">
      <div>
        <dt className="text-xs font-medium text-slate-500">What happened</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{presentation.title}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium text-slate-500">When</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{formatExactTimestamp(event.created_at)}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium text-slate-500">Actor</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{actorLabel}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium text-slate-500">Credential</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{credentialName || "Not recorded"}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium text-slate-500">Role</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{role ? formatServiceAccountRole(role) : "Not recorded"}</dd>
      </div>
      <div>
        <dt className="text-xs font-medium text-slate-500">Environment</dt>
        <dd className="mt-0.5 text-sm text-slate-900">{environment || "Not recorded"}</dd>
      </div>
    </dl>
  );
}

/* ------------------------------------------------------------------ */
/*  Shared helpers (local)                                             */
/* ------------------------------------------------------------------ */

function StatusChip({
  tone,
  children,
}: {
  tone: "success" | "neutral" | "warning" | "danger" | "info";
  children: ReactNode;
}) {
  const toneClassName =
    tone === "success"
      ? "border-emerald-200 bg-emerald-50 text-emerald-700"
      : tone === "warning"
        ? "border-amber-200 bg-amber-50 text-amber-700"
        : tone === "danger"
          ? "border-rose-200 bg-rose-50 text-rose-700"
          : tone === "info"
            ? "border-sky-200 bg-sky-50 text-sky-700"
            : "border-stone-200 bg-stone-100 text-slate-700";

  return (
    <span className={`inline-flex h-7 items-center rounded-full border px-2.5 text-[11px] font-semibold ${toneClassName}`}>
      {children}
    </span>
  );
}

function CursorPaginationControls({
  page,
  hasPreviousPage,
  hasNextPage,
  onPrevious,
  onNext,
  label,
}: {
  page: number;
  hasPreviousPage: boolean;
  hasNextPage: boolean;
  onPrevious: () => void;
  onNext: () => void;
  label: string;
}) {
  return (
    <div className="inline-flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-2 py-2">
      <button
        type="button"
        onClick={onPrevious}
        disabled={!hasPreviousPage}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronLeft className="h-4 w-4" />
      </button>
      <span className="min-w-[84px] text-center text-xs font-medium text-slate-500">
        Page {page}
      </span>
      <button
        type="button"
        onClick={onNext}
        disabled={!hasNextPage}
        aria-label={`Next ${label} page`}
        className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-stone-200 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <ChevronRight className="h-4 w-4" />
      </button>
    </div>
  );
}

function readAuditMetadataString(metadata: Record<string, unknown>, key: string): string {
  const value = metadata[key];
  return typeof value === "string" ? value.trim() : "";
}

function formatServiceAccountRole(role: string): string {
  switch (role) {
    case "admin":
      return "Admin";
    case "writer":
      return "Writer";
    default:
      return "Reader";
  }
}

function describeAuditEvent(event: APIKeyAuditEvent): { title: string; summary: string; supporting: string } {
  const metadataCount = Object.keys(event.metadata ?? {}).length;
  const purpose = typeof event.metadata?.purpose === "string" ? event.metadata.purpose.trim() : "";
  const environment = typeof event.metadata?.environment === "string" ? event.metadata.environment.trim() : "";
  const context = [purpose, environment].filter(Boolean).join(" \u00b7 ");
  const contextSuffix = context ? ` for ${context}` : "";
  const actorSummary = event.actor_api_key_id ? "Changed by another credential" : "Changed from the workspace access console";
  const metadataSummary = metadataCount > 0 ? `${metadataCount} supporting field${metadataCount === 1 ? "" : "s"}` : "No supporting fields";

  switch (event.action) {
    case "created":
      return {
        title: "Credential issued",
        summary: `A new credential was issued${contextSuffix}.`,
        supporting: `${actorSummary} \u00b7 ${metadataSummary}`,
      };
    case "revoked":
      return {
        title: "Credential revoked",
        summary: `A credential was revoked${contextSuffix}.`,
        supporting: `${actorSummary} \u00b7 ${metadataSummary}`,
      };
    case "rotated":
      return {
        title: "Credential rotated",
        summary: `A credential was rotated and replaced${contextSuffix}.`,
        supporting: `${actorSummary} \u00b7 ${metadataSummary}`,
      };
    default:
      return {
        title: formatAuditActionLabel(event.action),
        summary: context ? `Credential activity recorded for ${context}.` : "Credential activity recorded.",
        supporting: `${actorSummary} \u00b7 ${metadataSummary}`,
      };
  }
}

function formatAuditActionLabel(action: string): string {
  return action
    .split(/[_\s]+/)
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function formatAuditExportStatus(status: string): string {
  return status
    .split("_")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}
