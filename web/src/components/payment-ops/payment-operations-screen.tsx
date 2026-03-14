"use client";

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

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchInvoiceEvents, fetchInvoiceLifecycle, fetchInvoiceStatusSummary, fetchInvoiceStatuses, retryInvoicePayment } from "@/lib/api";
import { formatExactTimestamp, formatMoney, formatRelativeTimestamp } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";
import { type InvoiceStatusFilters } from "@/lib/types";
import { useSessionStore } from "@/store/use-session-store";

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
      return "bg-rose-600/20 text-rose-200 border border-rose-500/40";
    case "succeeded":
      return "bg-emerald-600/20 text-emerald-200 border border-emerald-500/40";
    case "pending":
      return "bg-amber-500/20 text-amber-100 border border-amber-400/40";
    default:
      return "bg-slate-600/20 text-slate-100 border border-slate-500/40";
  }
}

function invoiceBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "voided":
      return "bg-slate-500/20 text-slate-200 border border-slate-400/40";
    case "finalized":
      return "bg-cyan-600/20 text-cyan-100 border border-cyan-500/40";
    case "draft":
      return "bg-indigo-500/20 text-indigo-100 border border-indigo-400/40";
    default:
      return "bg-zinc-600/20 text-zinc-100 border border-zinc-500/40";
  }
}

export function PaymentOperationsScreen() {
  const queryClient = useQueryClient();
  const { selectedInvoiceID, setSelectedInvoiceID } = useSessionStore();
  const { apiBaseURL, csrfToken, isAuthenticated, canWrite, role } = useUISession();

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
    enabled: isAuthenticated,
  });
  const statusSummaryQuery = useQuery({
    queryKey: ["invoice-status-summary", apiBaseURL, organizationID, summaryStaleAfterSec],
    queryFn: () =>
      fetchInvoiceStatusSummary({
        runtimeBaseURL: apiBaseURL,
        organizationID: organizationID || undefined,
        staleAfterSec: summaryStaleAfterSec,
      }),
    enabled: isAuthenticated,
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
    enabled: isAuthenticated && selectedInvoiceID.length > 0 && timelineOpen,
  });
  const lifecycleQuery = useQuery({
    queryKey: ["invoice-lifecycle", apiBaseURL, selectedInvoiceID],
    queryFn: () =>
      fetchInvoiceLifecycle({
        runtimeBaseURL: apiBaseURL,
        invoiceID: selectedInvoiceID,
      }),
    enabled: isAuthenticated && selectedInvoiceID.length > 0 && timelineOpen,
  });

  const retryMutation = useMutation({
    mutationFn: (invoiceID: string) =>
      retryInvoicePayment({
        runtimeBaseURL: apiBaseURL,
        invoiceID,
        csrfToken,
      }),
    onSuccess: async (_, invoiceID) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["invoice-statuses"] }),
        queryClient.invalidateQueries({ queryKey: ["invoice-status-summary"] }),
        queryClient.invalidateQueries({ queryKey: ["invoice-events", apiBaseURL, invoiceID] }),
      ]);
    },
  });

  const items = statusesQuery.data?.items || [];
  const summary = statusSummaryQuery.data;
  const loadedCount = summary?.total_invoices ?? items.length;
  const failedCount = summary?.payment_status_counts?.failed ?? items.filter((item) => (item.payment_status || "").toLowerCase() === "failed").length;
  const overdueCount = summary?.overdue_count ?? items.filter((item) => Boolean(item.payment_overdue)).length;
  const attentionRequiredCount = summary?.attention_required_count ?? 0;
  const staleAttentionCount = summary?.stale_attention_required ?? 0;
  const selectedItem = items.find((item) => item.invoice_id === selectedInvoiceID);

  const canGoNextStatuses = items.length === statusLimit;
  const canGoPrevStatuses = statusOffset > 0;
  const canGoNextEvents = (eventsQuery.data?.items || []).length === eventLimit;
  const canGoPrevEvents = eventOffset > 0;

  const openTimeline = (invoiceID: string, orgID?: string) => {
    setSelectedInvoiceID(invoiceID);
    setSelectedOrganizationID((orgID || "").trim());
    setEventOffset(0);
    setTimelineOpen(true);
  };

  const resetStatusOffset = () => setStatusOffset(0);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1f2937_0%,_#0f172a_32%,_#090d16_74%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-50">
        <div className="absolute -left-32 top-8 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-orange-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Payment Operations Console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Failed Payment Triage</h1>
              <p className="mt-2 max-w-3xl text-sm text-slate-300 md:text-base">
                Inspect failed/pending/overdue invoices, open webhook timeline drawer, and trigger safe payment retries.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-6">
              <MetricCard label="Loaded invoices" value={loadedCount} />
              <MetricCard label="Failed" value={failedCount} tone="danger" />
              <MetricCard label="Overdue" value={overdueCount} tone="danger" />
              <MetricCard label="Attention" value={attentionRequiredCount} tone="danger" />
              <MetricCard label="Stale >5m" value={staleAttentionCount} />
              <MetricCard label="Timeline" value={timelineOpen ? "Open" : "Idle"} />
            </div>
          </div>

          <div className="mt-6 grid gap-3 md:grid-cols-2 xl:grid-cols-2">
            <InputField
              label="Organization ID"
              placeholder="optional org filter"
              value={organizationID}
              onChange={(value) => {
                setOrganizationID(value);
                resetStatusOffset();
              }}
            />
            <InputField
              label="Invoice Status"
              placeholder="finalized / draft / voided"
              value={invoiceStatus}
              onChange={(value) => {
                setInvoiceStatus(value);
                resetStatusOffset();
              }}
            />
          </div>

          <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Payment Overdue</label>
              <select
                className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                value={overdue}
                onChange={(event) => {
                  setOverdue(event.target.value as "all" | "true" | "false");
                  resetStatusOffset();
                }}
              >
                <option value="all">All</option>
                <option value="true">Overdue only</option>
                <option value="false">Not overdue</option>
              </select>
            </div>

            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Sort By</label>
              <select
                className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                value={statusSortBy}
                onChange={(event) => {
                  setStatusSortBy(event.target.value as (typeof statusSortOptions)[number]["value"]);
                  resetStatusOffset();
                }}
              >
                {statusSortOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>

            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Order</label>
              <select
                className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                value={statusOrder}
                onChange={(event) => {
                  setStatusOrder(event.target.value as (typeof orderOptions)[number]["value"]);
                  resetStatusOffset();
                }}
              >
                {orderOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>

            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Rows</label>
              <select
                className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                value={String(statusLimit)}
                onChange={(event) => {
                  setStatusLimit(Number(event.target.value));
                  resetStatusOffset();
                }}
              >
                <option value="25">25</option>
                <option value="50">50</option>
                <option value="100">100</option>
              </select>
            </div>

            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Actions</label>
              <button
                type="button"
                onClick={() => statusesQuery.refetch()}
                disabled={statusesQuery.isFetching || !isAuthenticated}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {statusesQuery.isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                Refresh
              </button>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <QuickFilterChip
              active={paymentStatus === "failed"}
              label="Failed"
              onClick={() => {
                setPaymentStatus(paymentStatus === "failed" ? "" : "failed");
                resetStatusOffset();
              }}
            />
            <QuickFilterChip
              active={paymentStatus === "pending"}
              label="Pending"
              onClick={() => {
                setPaymentStatus(paymentStatus === "pending" ? "" : "pending");
                resetStatusOffset();
              }}
            />
            <QuickFilterChip
              active={paymentStatus === "succeeded"}
              label="Succeeded"
              onClick={() => {
                setPaymentStatus(paymentStatus === "succeeded" ? "" : "succeeded");
                resetStatusOffset();
              }}
            />
          </div>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}

        {statusesQuery.error ? (
          <section className="rounded-2xl border border-rose-400/40 bg-rose-500/10 p-4 text-sm text-rose-100">
            {(statusesQuery.error as Error).message}
          </section>
        ) : null}

        <section className="rounded-2xl border border-white/10 bg-slate-900/75 p-3 backdrop-blur">
          <div className="overflow-auto">
            <table className="w-full min-w-[1140px] border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-xs uppercase tracking-wider text-slate-400">
                  <th className="px-3 py-1">Invoice</th>
                  <th className="px-3 py-1">Organization</th>
                  <th className="px-3 py-1">Customer</th>
                  <th className="px-3 py-1">Payment</th>
                  <th className="px-3 py-1">Invoice State</th>
                  <th className="px-3 py-1">Due</th>
                  <th className="px-3 py-1">Last Event</th>
                  <th className="px-3 py-1 text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => {
                  const selected = item.invoice_id === selectedInvoiceID;
                  const retrying = retryMutation.isPending && retryMutation.variables === item.invoice_id;
                  return (
                    <tr
                      key={item.invoice_id}
                      onClick={() => openTimeline(item.invoice_id, item.organization_id)}
                      className={`cursor-pointer transition ${
                        selected ? "bg-cyan-500/15" : "bg-slate-950/70 hover:bg-slate-800/90"
                      }`}
                    >
                      <td className="rounded-l-xl px-3 py-3 align-top">
                        <p className="font-medium text-cyan-200">{item.invoice_number || item.invoice_id}</p>
                        <p className="text-xs text-slate-400">{item.invoice_id}</p>
                      </td>
                      <td className="px-3 py-3 align-top text-slate-200">{item.organization_id || "-"}</td>
                      <td className="px-3 py-3 align-top text-slate-200">{item.customer_external_id || "-"}</td>
                      <td className="px-3 py-3 align-top">
                        <span className={`inline-flex rounded-lg px-2 py-1 text-xs font-medium ${paymentBadgeClass(item.payment_status)}`}>
                          {item.payment_status || "unknown"}
                        </span>
                        {item.last_payment_error ? (
                          <p className="mt-1 flex items-start gap-1 text-xs text-rose-200/90">
                            <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                            <span className="line-clamp-2">{item.last_payment_error}</span>
                          </p>
                        ) : null}
                      </td>
                      <td className="px-3 py-3 align-top">
                        <span className={`inline-flex rounded-lg px-2 py-1 text-xs font-medium ${invoiceBadgeClass(item.invoice_status)}`}>
                          {item.invoice_status || "unknown"}
                        </span>
                        <p className="mt-1 text-xs text-slate-400">Overdue: {String(item.payment_overdue ?? false)}</p>
                      </td>
                      <td className="px-3 py-3 align-top">
                        <p>{formatMoney(item.total_due_amount_cents, item.currency || "USD")}</p>
                        <p className="text-xs text-slate-400">
                          Paid {formatMoney(item.total_paid_amount_cents, item.currency || "USD")}
                        </p>
                      </td>
                      <td className="px-3 py-3 align-top">
                        <p className="text-slate-200">{item.last_event_type || "-"}</p>
                        <p className="text-xs text-slate-400" title={formatExactTimestamp(item.last_event_at)}>
                          {formatRelativeTimestamp(item.last_event_at)}
                        </p>
                      </td>
                      <td className="rounded-r-xl px-3 py-3 text-right align-top">
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              openTimeline(item.invoice_id, item.organization_id);
                            }}
                            className="inline-flex items-center gap-2 rounded-lg border border-cyan-400/50 bg-cyan-500/15 px-3 py-1.5 text-xs font-medium text-cyan-100 transition hover:bg-cyan-500/25"
                          >
                            Timeline
                          </button>
                          <button
                            type="button"
                            onClick={(event) => {
                              event.stopPropagation();
                              retryMutation.mutate(item.invoice_id);
                            }}
                            disabled={!isAuthenticated || !csrfToken || !canWrite || retrying}
                            className="inline-flex items-center gap-2 rounded-lg border border-emerald-400/50 bg-emerald-500/15 px-3 py-1.5 text-xs font-medium text-emerald-100 transition hover:bg-emerald-500/25 disabled:cursor-not-allowed disabled:opacity-50"
                            title={!canWrite ? "Writer or admin role required" : undefined}
                          >
                            {retrying ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RotateCcw className="h-3.5 w-3.5" />}
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

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-white/10 px-1 pt-3 text-xs text-slate-300">
            <p>
              Page {Math.floor(statusOffset / statusLimit) + 1}, showing {items.length} row(s)
            </p>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setStatusOffset(Math.max(0, statusOffset - statusLimit))}
                disabled={!canGoPrevStatuses || statusesQuery.isFetching}
                className="inline-flex items-center gap-1 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ChevronLeft className="h-3.5 w-3.5" /> Prev
              </button>
              <button
                type="button"
                onClick={() => setStatusOffset(statusOffset + statusLimit)}
                disabled={!canGoNextStatuses || statusesQuery.isFetching}
                className="inline-flex items-center gap-1 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Next <ChevronRight className="h-3.5 w-3.5" />
              </button>
            </div>
          </div>
        </section>

        {retryMutation.error ? (
          <section className="rounded-2xl border border-rose-400/40 bg-rose-500/10 p-4 text-sm text-rose-100">
            {(retryMutation.error as Error).message}
          </section>
        ) : null}

        {retryMutation.isSuccess ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-4 text-sm text-emerald-100">
            Retry request sent to billing engine for invoice <strong>{retryMutation.variables}</strong>.
          </section>
        ) : null}

        {isAuthenticated && !canWrite ? (
          <section className="rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
            Current session role <strong>{role ?? "reader"}</strong> is read-only for payment retry operations. Use a writer or admin key to trigger retries.
          </section>
        ) : null}
      </main>

      {timelineOpen && selectedInvoiceID ? (
        <div className="fixed inset-0 z-50 flex justify-end">
          <button
            aria-label="Close timeline"
            className="h-full flex-1 bg-slate-950/70 backdrop-blur-[2px]"
            onClick={() => setTimelineOpen(false)}
          />
          <aside className="relative h-full w-full max-w-2xl overflow-y-auto border-l border-white/10 bg-slate-900/95 p-5 shadow-2xl">
            <div className="mb-4 flex items-start justify-between gap-3">
              <div>
                <h2 className="text-xl font-semibold tracking-tight text-white">Invoice Timeline</h2>
                <p className="mt-1 text-xs text-slate-300">{selectedItem?.invoice_number || selectedInvoiceID}</p>
                <p className="text-[11px] text-slate-400">{selectedInvoiceID}</p>
              </div>
              <button
                type="button"
                onClick={() => setTimelineOpen(false)}
                className="rounded-lg border border-white/15 bg-white/5 p-2 text-slate-200 transition hover:bg-white/10"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <InputField
                label="Webhook Type"
                placeholder="invoice.payment_failure"
                value={eventWebhookType}
                onChange={(value) => {
                  setEventWebhookType(value);
                  setEventOffset(0);
                }}
              />
              <div className="grid gap-2">
                <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Sort</label>
                <select
                  className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  value={eventSortBy}
                  onChange={(event) => {
                    setEventSortBy(event.target.value as (typeof eventSortOptions)[number]["value"]);
                    setEventOffset(0);
                  }}
                >
                  {eventSortOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid gap-2">
                <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Order</label>
                <select
                  className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  value={eventOrder}
                  onChange={(event) => {
                    setEventOrder(event.target.value as (typeof orderOptions)[number]["value"]);
                    setEventOffset(0);
                  }}
                >
                  {orderOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid gap-2">
                <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Rows</label>
                <select
                  className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  value={String(eventLimit)}
                  onChange={(event) => {
                    setEventLimit(Number(event.target.value));
                    setEventOffset(0);
                  }}
                >
                  <option value="25">25</option>
                  <option value="50">50</option>
                  <option value="100">100</option>
                </select>
              </div>
            </div>

            <div className="mt-3 flex justify-end">
              <button
                type="button"
                onClick={() => eventsQuery.refetch()}
                disabled={eventsQuery.isFetching || !isAuthenticated}
                className="inline-flex h-9 items-center gap-2 rounded-lg border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {eventsQuery.isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                Refresh timeline
              </button>
            </div>

            <section className="mt-4 rounded-xl border border-white/10 bg-slate-950/60 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-200">Lifecycle Summary</h3>
                <button
                  type="button"
                  onClick={() => lifecycleQuery.refetch()}
                  disabled={lifecycleQuery.isFetching || !isAuthenticated}
                  className="inline-flex h-8 items-center gap-2 rounded-lg border border-cyan-400/40 bg-cyan-500/10 px-3 text-xs text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {lifecycleQuery.isFetching ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                  Refresh
                </button>
              </div>

              {lifecycleQuery.isLoading ? <EmptyState label="Loading lifecycle summary..." icon={<LoaderCircle className="h-4 w-4 animate-spin" />} /> : null}
              {lifecycleQuery.error ? (
                <EmptyState label={(lifecycleQuery.error as Error).message} icon={<AlertCircle className="h-4 w-4" />} tone="danger" />
              ) : null}

              {lifecycleQuery.data ? (
                <div className="space-y-3">
                  <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                    <MetricCard label="Failure signals" value={lifecycleQuery.data.failure_event_count} tone="danger" />
                    <MetricCard label="Pending signals" value={lifecycleQuery.data.pending_event_count} />
                    <MetricCard label="Overdue signals" value={lifecycleQuery.data.overdue_signal_count} tone="danger" />
                    <MetricCard label="Events analyzed" value={lifecycleQuery.data.events_analyzed} />
                  </div>
                  <div className="rounded-lg border border-white/10 bg-slate-900/70 p-3 text-xs text-slate-200">
                    <p className="font-semibold uppercase tracking-wider text-slate-300">Recommended Action</p>
                    <p className="mt-1 text-sm text-white">{lifecycleQuery.data.recommended_action}</p>
                    <p className="mt-1 text-slate-300">{lifecycleQuery.data.recommended_action_note}</p>
                    <p className="mt-2 text-[11px] text-slate-400">
                      Last failure: {lifecycleQuery.data.last_failure_at ? formatExactTimestamp(lifecycleQuery.data.last_failure_at) : "-"} | Last success:{" "}
                      {lifecycleQuery.data.last_success_at ? formatExactTimestamp(lifecycleQuery.data.last_success_at) : "-"}
                    </p>
                    {lifecycleQuery.data.event_window_truncated ? (
                      <p className="mt-1 text-[11px] text-amber-200">
                        Event window truncated at {lifecycleQuery.data.event_window_limit} rows. Use timeline filters for deeper history.
                      </p>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </section>

            <div className="mt-4 space-y-3">
              {!selectedInvoiceID ? <EmptyState label="Pick an invoice row to inspect its payment timeline." /> : null}

              {selectedInvoiceID && eventsQuery.isLoading ? (
                <EmptyState label="Loading timeline..." icon={<LoaderCircle className="h-4 w-4 animate-spin" />} />
              ) : null}

              {selectedInvoiceID && eventsQuery.error ? (
                <EmptyState label={(eventsQuery.error as Error).message} icon={<AlertCircle className="h-4 w-4" />} tone="danger" />
              ) : null}

              {selectedInvoiceID && eventsQuery.data?.items?.length === 0 ? (
                <EmptyState label="No webhook events found for this invoice yet." />
              ) : null}

              {(eventsQuery.data?.items || []).map((event) => (
                <article key={event.id} className="rounded-xl border border-white/10 bg-slate-950/70 p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-sm font-medium text-white">{event.webhook_type}</p>
                      <p className="text-xs text-slate-400">Occurred {formatExactTimestamp(event.occurred_at)}</p>
                      <p className="text-[11px] text-slate-500">Received {formatExactTimestamp(event.received_at)}</p>
                    </div>
                    <span className={`rounded-md px-2 py-1 text-[11px] ${paymentBadgeClass(event.payment_status)}`}>
                      {event.payment_status || "n/a"}
                    </span>
                  </div>
                  <dl className="mt-2 grid grid-cols-1 gap-1 text-xs text-slate-300">
                    <div className="flex items-center justify-between gap-2">
                      <dt className="text-slate-400">Object</dt>
                      <dd>{event.object_type}</dd>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <dt className="text-slate-400">Webhook key</dt>
                      <dd className="max-w-[62%] truncate" title={event.webhook_key}>
                        {event.webhook_key}
                      </dd>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <dt className="text-slate-400">Relative time</dt>
                      <dd>{formatRelativeTimestamp(event.received_at)}</dd>
                    </div>
                  </dl>
                </article>
              ))}
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-white/10 pt-3 text-xs text-slate-300">
              <p>
                Page {Math.floor(eventOffset / eventLimit) + 1}, showing {(eventsQuery.data?.items || []).length} event(s)
              </p>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setEventOffset(Math.max(0, eventOffset - eventLimit))}
                  disabled={!canGoPrevEvents || eventsQuery.isFetching}
                  className="inline-flex items-center gap-1 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ChevronLeft className="h-3.5 w-3.5" /> Prev
                </button>
                <button
                  type="button"
                  onClick={() => setEventOffset(eventOffset + eventLimit)}
                  disabled={!canGoNextEvents || eventsQuery.isFetching}
                  className="inline-flex items-center gap-1 rounded-lg border border-white/15 bg-white/5 px-2.5 py-1.5 transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-50"
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

function InputField(props: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  sensitive?: boolean;
}) {
  return (
    <div className="grid gap-2">
      <label className="text-xs font-medium uppercase tracking-wider text-slate-300">{props.label}</label>
      <input
        type={props.sensitive ? "password" : "text"}
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        placeholder={props.placeholder}
        className="h-10 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </div>
  );
}

function QuickFilterChip(props: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={props.onClick}
      className={`rounded-lg border px-3 py-1.5 text-xs transition ${
        props.active ? "border-cyan-400/60 bg-cyan-500/20 text-cyan-100" : "border-white/15 bg-white/5 text-slate-200 hover:bg-white/10"
      }`}
    >
      {props.label}
    </button>
  );
}

function EmptyState(props: {
  label: string;
  tone?: "neutral" | "danger";
  icon?: ReactNode;
}) {
  return (
    <div
      className={`rounded-xl border p-4 text-sm ${
        props.tone === "danger" ? "border-rose-400/40 bg-rose-500/10 text-rose-100" : "border-white/10 bg-slate-950/60 text-slate-300"
      }`}
    >
      <div className="flex items-center gap-2">
        {props.icon || <CreditCard className="h-4 w-4" />}
        <span>{props.label}</span>
      </div>
    </div>
  );
}

function MetricCard(props: {
  label: string;
  value: string | number;
  tone?: "default" | "danger";
}) {
  return (
    <article className={`rounded-xl border px-3 py-2 ${props.tone === "danger" ? "border-rose-400/30 bg-rose-500/10" : "border-white/15 bg-white/5"}`}>
      <p className="text-[11px] uppercase tracking-wider text-slate-400">{props.label}</p>
      <p className="mt-1 text-lg font-semibold text-white">{props.value}</p>
    </article>
  );
}
