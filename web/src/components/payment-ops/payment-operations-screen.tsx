
import { type ReactNode, useMemo, useState } from "react";
import {
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  CreditCard,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { BillingFailureDiagnosisCard } from "@/components/billing/billing-failure-diagnosis";
import { fetchInvoiceEvents, fetchInvoiceLifecycle, fetchInvoiceStatusSummary, fetchInvoiceStatuses, retryInvoicePayment } from "@/lib/api";
import { formatExactTimestamp, formatMoney, formatRelativeTimestamp } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";
import { type InvoiceStatusFilters } from "@/lib/types";
import { useSessionStore } from "@/store/use-session-store";
import { billingFailureDiagnosis, formatBillingState } from "@/lib/billing-lifecycle";
import { showError, showSuccess } from "@/lib/toast";

const statusSortOptions = [
  { value: "last_event_at", label: "Last event" },
  { value: "updated_at", label: "Projection updated" },
  { value: "total_due_amount_cents", label: "Total due" },
  { value: "total_amount_cents", label: "Invoice total" },
] as const;

const eventSortOptions = [
  { value: "received_at", label: "Received at" },
  { value: "occurred_at", label: "Occurred at" },
] as const;

const orderOptions = [
  { value: "desc", label: "Newest first" },
  { value: "asc", label: "Oldest first" },
] as const;

function paymentBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "failed":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "succeeded":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

function invoiceBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "voided":
      return "border-slate-200 bg-slate-50 text-slate-600";
    case "finalized":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "draft":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
}

function diagnosisBadgeClass(tone: "healthy" | "warning" | "danger"): string {
  switch (tone) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-rose-200 bg-rose-50 text-rose-700";
  }
}

export function PaymentOperationsScreen() {
  const queryClient = useQueryClient();
  const { selectedInvoiceID, setSelectedInvoiceID } = useSessionStore();
  const { apiBaseURL, csrfToken, isAuthenticated, isLoading: sessionLoading, canWrite, role, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [organizationID, setOrganizationID] = useState("");
  const [paymentStatus, setPaymentStatus] = useState("");
  const [invoiceStatus, setInvoiceStatus] = useState("");
  const [overdue, setOverdue] = useState<"all" | "true" | "false">("all");
  const [statusSortBy, setStatusSortBy] = useState<(typeof statusSortOptions)[number]["value"]>("last_event_at");
  const [statusOrder, setStatusOrder] = useState<(typeof orderOptions)[number]["value"]>("desc");
  const [statusLimit, setStatusLimit] = useState(25);
  const [statusOffset, setStatusOffset] = useState(0);

  const [timelineOpen, setTimelineOpen] = useState(false);
  const [selectedOrganizationID, setSelectedOrganizationID] = useState("");
  const [eventWebhookType, setEventWebhookType] = useState("");
  const [eventSortBy, setEventSortBy] = useState<(typeof eventSortOptions)[number]["value"]>("received_at");
  const [eventOrder, setEventOrder] = useState<(typeof orderOptions)[number]["value"]>("desc");
  const [eventLimit, setEventLimit] = useState(50);
  const [eventOffset, setEventOffset] = useState(0);
  const [selectedEventID, setSelectedEventID] = useState("");
  const summaryStaleAfterSec = 300;

  const filters: InvoiceStatusFilters = useMemo(
    () => ({
      organization_id: organizationID || undefined,
      payment_status: paymentStatus || undefined,
      invoice_status: invoiceStatus || undefined,
      payment_overdue: overdue === "all" ? undefined : overdue === "true",
      sort_by: statusSortBy,
      order: statusOrder,
      limit: statusLimit,
      offset: statusOffset,
    }),
    [organizationID, paymentStatus, invoiceStatus, overdue, statusSortBy, statusOrder, statusLimit, statusOffset]
  );

  const statusesQuery = useQuery({
    queryKey: ["invoice-statuses", apiBaseURL, filters],
    queryFn: () =>
      fetchInvoiceStatuses({
        runtimeBaseURL: apiBaseURL,
        filters,
      }),
    enabled: isTenantSession,
  });
  const statusSummaryQuery = useQuery({
    queryKey: ["invoice-status-summary", apiBaseURL, organizationID, summaryStaleAfterSec],
    queryFn: () =>
      fetchInvoiceStatusSummary({
        runtimeBaseURL: apiBaseURL,
        organizationID: organizationID || undefined,
        staleAfterSec: summaryStaleAfterSec,
      }),
    enabled: isTenantSession,
  });

  const eventsQuery = useQuery({
    queryKey: [
      "invoice-events",
      apiBaseURL,
      selectedInvoiceID,
      selectedOrganizationID,
      eventWebhookType,
      eventSortBy,
      eventOrder,
      eventLimit,
      eventOffset,
    ],
    queryFn: () =>
      fetchInvoiceEvents({
        runtimeBaseURL: apiBaseURL,
        invoiceID: selectedInvoiceID,
        organizationID: selectedOrganizationID || undefined,
        webhookType: eventWebhookType || undefined,
        sortBy: eventSortBy,
        order: eventOrder,
        limit: eventLimit,
        offset: eventOffset,
      }),
    enabled: isTenantSession && selectedInvoiceID.length > 0 && timelineOpen,
  });
  const lifecycleQuery = useQuery({
    queryKey: ["invoice-lifecycle", apiBaseURL, selectedInvoiceID],
    queryFn: () =>
      fetchInvoiceLifecycle({
        runtimeBaseURL: apiBaseURL,
        invoiceID: selectedInvoiceID,
      }),
    enabled: isTenantSession && selectedInvoiceID.length > 0 && timelineOpen,
  });

  const retryMutation = useMutation({
    mutationFn: (invoiceID: string) =>
      retryInvoicePayment({
        runtimeBaseURL: apiBaseURL,
        invoiceID,
        csrfToken,
      }),
    onSuccess: async (_, invoiceID) => {
      showSuccess("Payment retry queued", "Invoice payment retry has been submitted.");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["invoice-statuses"] }),
        queryClient.invalidateQueries({ queryKey: ["invoice-status-summary"] }),
        queryClient.invalidateQueries({ queryKey: ["invoice-events", apiBaseURL, invoiceID] }),
      ]);
    },
    onError: (err: Error) => {
      showError("Retry failed", err.message || "Could not retry invoice payment.");
    },
  });

  const items = statusesQuery.data?.items || [];
  const summary = statusSummaryQuery.data;
  const selectedItem = items.find((item) => item.invoice_id === selectedInvoiceID);
  const timelineItems = eventsQuery.data?.items ?? [];
  const selectedEventIDValue =
    timelineOpen && selectedEventID && timelineItems.some((item) => item.id === selectedEventID)
      ? selectedEventID
      : "";
  const selectedEvent = timelineItems.find((item) => item.id === selectedEventIDValue) ?? null;

  const canGoNextStatuses = items.length === statusLimit;
  const canGoPrevStatuses = statusOffset > 0;
  const canGoNextEvents = (eventsQuery.data?.items || []).length === eventLimit;
  const canGoPrevEvents = eventOffset > 0;

  const openTimeline = (invoiceID: string, orgID?: string) => {
    setSelectedInvoiceID(invoiceID);
    setSelectedOrganizationID((orgID || "").trim());
    setEventOffset(0);
    setSelectedEventID("");
    setTimelineOpen(true);
  };

  const resetStatusOffset = () => setStatusOffset(0);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">

        <LoginRedirectNotice />

        <>
            <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
              {/* ---- Header ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900">Payment operations</h1>
                    <span className="text-xs text-slate-500">{summary?.total_invoices ?? items.length} invoices</span>
                    {(summary?.payment_status_counts?.failed ?? 0) > 0 ? (
                      <span className="shrink-0 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-rose-700">
                        {summary?.payment_status_counts?.failed} failed
                      </span>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() => statusesQuery.refetch()}
                      disabled={statusesQuery.isFetching}
                      className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {statusesQuery.isFetching ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                      Refresh
                    </button>
                  </div>
                </div>
              </div>

              {/* ---- Filters ---- */}
              <div className="px-5 py-4">
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
                  <label className="grid gap-1 text-sm">
                    <span className="text-xs text-slate-400">Organization ID</span>
                    <input
                      type="text"
                      value={organizationID}
                      onChange={(e) => { setOrganizationID(e.target.value); resetStatusOffset(); }}
                      placeholder="optional org filter"
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                    />
                  </label>
                  <label className="grid gap-1 text-sm">
                    <span className="text-xs text-slate-400">Invoice status</span>
                    <input
                      type="text"
                      value={invoiceStatus}
                      onChange={(e) => { setInvoiceStatus(e.target.value); resetStatusOffset(); }}
                      placeholder="finalized / draft / voided"
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                    />
                  </label>
                  <label className="grid gap-1 text-sm">
                    <span className="text-xs text-slate-400">Overdue</span>
                    <select
                      value={overdue}
                      onChange={(e) => { setOverdue(e.target.value as "all" | "true" | "false"); resetStatusOffset(); }}
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      <option value="all">All</option>
                      <option value="true">Overdue only</option>
                      <option value="false">Not overdue</option>
                    </select>
                  </label>
                  <label className="grid gap-1 text-sm">
                    <span className="text-xs text-slate-400">Sort by</span>
                    <select
                      value={statusSortBy}
                      onChange={(e) => { setStatusSortBy(e.target.value as (typeof statusSortOptions)[number]["value"]); resetStatusOffset(); }}
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      {statusSortOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                    </select>
                  </label>
                  <label className="grid gap-1 text-sm">
                    <span className="text-xs text-slate-400">Order</span>
                    <select
                      value={statusOrder}
                      onChange={(e) => { setStatusOrder(e.target.value as (typeof orderOptions)[number]["value"]); resetStatusOffset(); }}
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      {orderOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                    </select>
                  </label>
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <QuickFilterChip active={paymentStatus === "failed"} label="Failed" onClick={() => { setPaymentStatus(paymentStatus === "failed" ? "" : "failed"); resetStatusOffset(); }} />
                  <QuickFilterChip active={paymentStatus === "pending"} label="Pending" onClick={() => { setPaymentStatus(paymentStatus === "pending" ? "" : "pending"); resetStatusOffset(); }} />
                  <QuickFilterChip active={paymentStatus === "succeeded"} label="Succeeded" onClick={() => { setPaymentStatus(paymentStatus === "succeeded" ? "" : "succeeded"); resetStatusOffset(); }} />
                </div>
              </div>

              {/* ---- Table ---- */}
              <div className="overflow-auto">
                <table className="w-full min-w-[900px] text-sm">
                  <thead>
                    <tr className="border-b border-stone-200 text-left text-xs text-slate-400">
                      <th className="px-5 py-2 font-medium">Invoice</th>
                      <th className="px-3 py-2 font-medium">Customer</th>
                      <th className="px-3 py-2 font-medium">Status</th>
                      <th className="px-3 py-2 font-medium">Amount</th>
                      <th className="px-3 py-2 font-medium">Last event</th>
                      <th className="px-3 py-2 font-medium text-right">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {items.map((item) => {
                      const selected = item.invoice_id === selectedInvoiceID;
                      const retrying = retryMutation.isPending && retryMutation.variables === item.invoice_id;
                      const diagnosis = billingFailureDiagnosis(item);
                      return (
                        <tr
                          key={item.invoice_id}
                          onClick={() => openTimeline(item.invoice_id, item.organization_id)}
                          className={`cursor-pointer border-b border-stone-100 transition ${selected ? "bg-slate-50" : "bg-white hover:bg-slate-50"}`}
                        >
                          <td className="px-5 py-3 align-top">
                            <p className="font-medium text-slate-900">{item.invoice_number || item.invoice_id}</p>
                            <p className="text-xs text-slate-400">{item.organization_id || "-"}</p>
                          </td>
                          <td className="px-3 py-3 align-top text-slate-700">{item.customer_external_id || "-"}</td>
                          <td className="px-3 py-3 align-top">
                            <span className={`inline-flex rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${paymentBadgeClass(item.payment_status)}`}>
                              {item.payment_status || "unknown"}
                            </span>
                            {item.last_payment_error ? (
                              <p className="mt-1 flex items-start gap-1 text-xs text-rose-700">
                                <AlertCircle className="mt-0.5 h-3 w-3 shrink-0" />
                                <span className="line-clamp-1">{item.last_payment_error}</span>
                              </p>
                            ) : null}
                            <span className={`mt-1 inline-flex rounded-full border px-2 py-0.5 text-[10px] font-medium ${diagnosisBadgeClass(diagnosis.tone)}`}>
                              {diagnosis.title}
                            </span>
                          </td>
                          <td className="px-3 py-3 align-top">
                            <p className="text-slate-700">{formatMoney(item.total_due_amount_cents, item.currency || "USD")}</p>
                            <p className="text-xs text-slate-400">Paid {formatMoney(item.total_paid_amount_cents, item.currency || "USD")}</p>
                          </td>
                          <td className="px-3 py-3 align-top">
                            <p className="text-slate-700">{item.last_event_type || "-"}</p>
                            <p className="text-xs text-slate-400" title={formatExactTimestamp(item.last_event_at)}>
                              {formatRelativeTimestamp(item.last_event_at)}
                            </p>
                          </td>
                          <td className="px-3 py-3 text-right align-top">
                            <div className="flex justify-end gap-2">
                              <button
                                type="button"
                                onClick={(e) => { e.stopPropagation(); openTimeline(item.invoice_id, item.organization_id); }}
                                className="inline-flex items-center rounded-md border border-slate-200 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                              >
                                Timeline
                              </button>
                              <button
                                type="button"
                                onClick={(e) => { e.stopPropagation(); retryMutation.mutate(item.invoice_id); }}
                                disabled={!isAuthenticated || !csrfToken || !canWrite || retrying}
                                className="inline-flex items-center gap-1.5 rounded-md border border-slate-200 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                                title={!canWrite ? "Writer or admin role required" : undefined}
                              >
                                {retrying ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <RotateCcw className="h-3 w-3" />}
                                Retry
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>

              {/* ---- Pagination ---- */}
              <div className="flex flex-wrap items-center justify-between gap-3 px-5 py-3 text-xs text-slate-500">
                <p>Page {Math.floor(statusOffset / statusLimit) + 1}, showing {items.length} row(s)</p>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setStatusOffset(Math.max(0, statusOffset - statusLimit))}
                    disabled={!canGoPrevStatuses || statusesQuery.isFetching}
                    className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2.5 py-1 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <ChevronLeft className="h-3.5 w-3.5" /> Prev
                  </button>
                  <button
                    type="button"
                    onClick={() => setStatusOffset(statusOffset + statusLimit)}
                    disabled={!canGoNextStatuses || statusesQuery.isFetching}
                    className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2.5 py-1 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Next <ChevronRight className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>
            </div>

            {statusesQuery.error ? (
              <div className="rounded-md border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                {(statusesQuery.error as Error).message}
              </div>
            ) : null}

            {retryMutation.error ? (
              <div className="rounded-md border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                {(retryMutation.error as Error).message}
              </div>
            ) : null}

            {retryMutation.isSuccess ? (
              <div className="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
                Retry request sent for invoice <strong>{retryMutation.variables}</strong>.
              </div>
            ) : null}

            {isAuthenticated && !canWrite ? (
              <div className="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700">
                Current session role <strong>{role ?? "reader"}</strong> is read-only for payment retry operations.
              </div>
            ) : null}
          </>
      </main>

      {/* ---- Slide-out timeline panel ---- */}
      {timelineOpen && selectedInvoiceID ? (
        <div className="fixed inset-0 z-50 flex justify-end">
          <button
            aria-label="Close timeline"
            className="h-full flex-1 bg-black/10 backdrop-blur-[2px]"
            onClick={() => setTimelineOpen(false)}
          />
          <aside className="relative h-full w-full max-w-2xl overflow-y-auto border-l border-stone-200 bg-white p-5 shadow-2xl">
            <div className="mb-4 flex items-start justify-between gap-3">
              <div>
                <h2 className="text-sm font-semibold text-slate-900">Invoice timeline</h2>
                <p className="mt-0.5 text-xs text-slate-500">{selectedItem?.invoice_number || selectedInvoiceID}</p>
              </div>
              <button
                type="button"
                onClick={() => setTimelineOpen(false)}
                className="rounded-md border border-slate-200 bg-white p-1.5 text-slate-700 transition hover:bg-slate-50"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <label className="grid gap-1 text-sm">
                <span className="text-xs text-slate-400">Webhook type</span>
                <input
                  type="text"
                  value={eventWebhookType}
                  onChange={(e) => { setEventWebhookType(e.target.value); setEventOffset(0); }}
                  placeholder="invoice.payment_failure"
                  className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
              </label>
              <label className="grid gap-1 text-sm">
                <span className="text-xs text-slate-400">Sort</span>
                <select
                  value={eventSortBy}
                  onChange={(e) => { setEventSortBy(e.target.value as (typeof eventSortOptions)[number]["value"]); setEventOffset(0); }}
                  className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {eventSortOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </label>
              <label className="grid gap-1 text-sm">
                <span className="text-xs text-slate-400">Order</span>
                <select
                  value={eventOrder}
                  onChange={(e) => { setEventOrder(e.target.value as (typeof orderOptions)[number]["value"]); setEventOffset(0); }}
                  className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {orderOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </label>
              <label className="grid gap-1 text-sm">
                <span className="text-xs text-slate-400">Rows</span>
                <select
                  value={String(eventLimit)}
                  onChange={(e) => { setEventLimit(Number(e.target.value)); setEventOffset(0); }}
                  className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="25">25</option>
                  <option value="50">50</option>
                  <option value="100">100</option>
                </select>
              </label>
            </div>

            <div className="mt-3 flex justify-end">
              <button
                type="button"
                onClick={() => eventsQuery.refetch()}
                disabled={eventsQuery.isFetching || !isAuthenticated}
                className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {eventsQuery.isFetching ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                Refresh
              </button>
            </div>

            {/* ---- Diagnosis ---- */}
            <div className="mt-4 rounded-lg border border-stone-200 bg-slate-50 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <p className="text-xs font-medium text-slate-400">Current diagnosis</p>
                <button
                  type="button"
                  onClick={() => lifecycleQuery.refetch()}
                  disabled={lifecycleQuery.isFetching || !isAuthenticated}
                  className="inline-flex h-7 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-2.5 text-xs text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {lifecycleQuery.isFetching ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3" />}
                  Refresh
                </button>
              </div>

              {lifecycleQuery.isLoading ? <EmptyState label="Loading lifecycle summary..." icon={<LoaderCircle className="h-4 w-4 animate-spin" />} /> : null}
              {lifecycleQuery.error ? (
                <EmptyState label={(lifecycleQuery.error as Error).message} icon={<AlertCircle className="h-4 w-4" />} tone="danger" />
              ) : null}

              {lifecycleQuery.data ? (
                <div className="space-y-3">
                  <BillingFailureDiagnosisCard
                    diagnosis={billingFailureDiagnosis({
                      payment_status: selectedItem?.payment_status,
                      invoice_status: selectedItem?.invoice_status,
                      payment_overdue: selectedItem?.payment_overdue,
                      last_payment_error: selectedItem?.last_payment_error,
                      lifecycle: lifecycleQuery.data,
                    })}
                    label="Failure diagnosis"
                  />
                  <dl className="grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-4">
                    <div>
                      <dt className="text-xs text-slate-400">Failures</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{lifecycleQuery.data.failure_event_count}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Pending</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{lifecycleQuery.data.pending_event_count}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Overdue</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{lifecycleQuery.data.overdue_signal_count}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Analyzed</dt>
                      <dd className="mt-0.5 text-sm text-slate-700">{lifecycleQuery.data.events_analyzed}</dd>
                    </div>
                  </dl>
                  <div className="text-sm text-slate-600">
                    <p><span className="font-medium text-slate-900">{formatBillingState(lifecycleQuery.data.recommended_action)}</span></p>
                    <p className="mt-0.5 text-xs">{lifecycleQuery.data.recommended_action_note}</p>
                    <p className="mt-1 text-xs text-slate-400">
                      Last failure: {lifecycleQuery.data.last_failure_at ? formatExactTimestamp(lifecycleQuery.data.last_failure_at) : "-"} | Last success:{" "}
                      {lifecycleQuery.data.last_success_at ? formatExactTimestamp(lifecycleQuery.data.last_success_at) : "-"}
                    </p>
                    {lifecycleQuery.data.event_window_truncated ? (
                      <p className="mt-1 text-xs text-amber-700">
                        Event window truncated at {lifecycleQuery.data.event_window_limit} rows.
                      </p>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>

            {/* ---- Timeline events ---- */}
            <div className="mt-4 space-y-3">
              {!selectedInvoiceID ? <EmptyState label="Pick an invoice row to inspect its payment timeline." /> : null}
              {selectedInvoiceID && eventsQuery.isLoading ? <EmptyState label="Loading timeline..." icon={<LoaderCircle className="h-4 w-4 animate-spin" />} /> : null}
              {selectedInvoiceID && eventsQuery.error ? <EmptyState label={(eventsQuery.error as Error).message} icon={<AlertCircle className="h-4 w-4" />} tone="danger" /> : null}
              {selectedInvoiceID && eventsQuery.data?.items?.length === 0 ? <EmptyState label="No webhook events found for this invoice yet." /> : null}

              {(eventsQuery.data?.items || []).length > 0 ? (
                <div className="grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(280px,0.85fr)]">
                  <div className="space-y-2">
                    {(eventsQuery.data?.items || []).map((event) => (
                      <TimelineEventRow
                        key={event.id}
                        event={event}
                        selected={event.id === selectedEventID}
                        onSelect={() => setSelectedEventID(event.id)}
                      />
                    ))}
                  </div>
                  <TimelineEventDetail event={selectedEvent} />
                </div>
              ) : null}
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-stone-200 pt-3 text-xs text-slate-500">
              <p>Page {Math.floor(eventOffset / eventLimit) + 1}, showing {(eventsQuery.data?.items || []).length} event(s)</p>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setEventOffset(Math.max(0, eventOffset - eventLimit))}
                  disabled={!canGoPrevEvents || eventsQuery.isFetching}
                  className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2.5 py-1 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ChevronLeft className="h-3.5 w-3.5" /> Prev
                </button>
                <button
                  type="button"
                  onClick={() => setEventOffset(eventOffset + eventLimit)}
                  disabled={!canGoNextEvents || eventsQuery.isFetching}
                  className="inline-flex items-center gap-1 rounded-md border border-slate-200 bg-white px-2.5 py-1 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Next <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </aside>
        </div>
      ) : null}
    </div>
  );
}

function TimelineEventRow({
  event,
  selected,
  onSelect,
}: {
  event: Awaited<ReturnType<typeof fetchInvoiceEvents>>["items"][number];
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`View timeline event ${event.webhook_type}`}
      className={`w-full rounded-lg border p-3 text-left transition ${
        selected
          ? "border-slate-300 bg-slate-50 shadow-sm"
          : "border-stone-200 bg-white hover:border-stone-300 hover:bg-slate-50"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-slate-900">{event.webhook_type}</p>
          <p className="mt-0.5 text-xs text-slate-400">Occurred {formatExactTimestamp(event.occurred_at)}</p>
          <p className="text-[11px] text-slate-400">Received {formatRelativeTimestamp(event.received_at)}</p>
        </div>
        <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${paymentBadgeClass(event.payment_status)}`}>
          {event.payment_status || "n/a"}
        </span>
      </div>
    </button>
  );
}

function TimelineEventDetail({
  event,
}: {
  event: Awaited<ReturnType<typeof fetchInvoiceEvents>>["items"][number] | null;
}) {
  if (!event) {
    return (
      <aside className="rounded-lg border border-dashed border-slate-300 bg-slate-50 px-4 py-6 text-sm text-slate-500">
        Select a timeline event to inspect its raw webhook fields.
      </aside>
    );
  }

  return (
    <aside className="rounded-lg border border-stone-200 bg-slate-50 p-4">
      <p className="text-xs font-medium text-slate-400 mb-3">Event detail</p>
      <dl className="grid gap-2 sm:grid-cols-2">
        <div>
          <dt className="text-xs text-slate-400">Webhook type</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{event.webhook_type}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-400">Payment status</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{event.payment_status || "-"}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-400">Object</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{event.object_type}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-400">Occurred at</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(event.occurred_at)}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-400">Received at</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(event.received_at)}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-400">Relative time</dt>
          <dd className="mt-0.5 text-sm text-slate-700">{formatRelativeTimestamp(event.received_at)}</dd>
        </div>
        <div className="sm:col-span-2">
          <dt className="text-xs text-slate-400">Webhook key</dt>
          <dd className="mt-0.5 break-all text-sm font-mono text-slate-700">{event.webhook_key}</dd>
        </div>
      </dl>
    </aside>
  );
}

function QuickFilterChip(props: { active: boolean; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={props.onClick}
      className={`rounded-md border px-2.5 py-1 text-xs transition ${
        props.active ? "border-slate-900 bg-slate-900 text-white" : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50"
      }`}
    >
      {props.label}
    </button>
  );
}

function EmptyState(props: { label: string; tone?: "neutral" | "danger"; icon?: ReactNode }) {
  return (
    <div className={`rounded-lg border p-4 text-sm ${props.tone === "danger" ? "border-rose-200 bg-rose-50 text-rose-700" : "border-stone-200 bg-slate-50 text-slate-500"}`}>
      <div className="flex items-center gap-2">
        {props.icon || <CreditCard className="h-4 w-4" />}
        <span>{props.label}</span>
      </div>
    </div>
  );
}
