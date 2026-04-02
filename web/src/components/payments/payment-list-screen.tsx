"use client";

import Link from "next/link";
import { Skeleton } from "@/components/ui/skeleton";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPayments } from "@/lib/api";
import { billingFailureDiagnosis } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { type PaymentFilters, type PaymentSummary } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

const sortOptions = [
  { value: "last_event_at", label: "Last event" },
  { value: "updated_at", label: "Updated" },
  { value: "total_due_amount_cents", label: "Amount due" },
  { value: "total_amount_cents", label: "Invoice total" },
] as const;

const orderOptions = [
  { value: "desc", label: "Newest first" },
  { value: "asc", label: "Oldest first" },
] as const;

const paymentStatusOptions = ["failed", "pending", "succeeded", "processing", "requires_action"] as const;
const invoiceStatusOptions = ["finalized", "draft", "voided", "pending"] as const;

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

function diagnosisToneClass(tone: "healthy" | "warning" | "danger"): string {
  switch (tone) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-800";
    default:
      return "border-rose-200 bg-rose-50 text-rose-800";
  }
}

export function PaymentListScreen() {
  const searchParams = useSearchParams();
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [customerExternalID, setCustomerExternalID] = useState(searchParams.get("customer_external_id") || "");
  const [invoiceID, setInvoiceID] = useState(searchParams.get("invoice_id") || "");
  const [invoiceNumber, setInvoiceNumber] = useState(searchParams.get("invoice_number") || "");
  const [invoiceStatus, setInvoiceStatus] = useState(searchParams.get("invoice_status") || "");
  const [paymentStatus, setPaymentStatus] = useState(searchParams.get("payment_status") || "");
  const [lastEventType, setLastEventType] = useState(searchParams.get("last_event_type") || "");
  const [paymentOverdue, setPaymentOverdue] = useState<"all" | "true" | "false">("all");
  const [sortBy, setSortBy] = useState<PaymentFilters["sort_by"]>("last_event_at");
  const [order, setOrder] = useState<PaymentFilters["order"]>("desc");

  const filters = useMemo<PaymentFilters>(
    () => ({
      customer_external_id: customerExternalID.trim() || undefined,
      invoice_id: invoiceID.trim() || undefined,
      invoice_number: invoiceNumber.trim() || undefined,
      last_event_type: lastEventType.trim() || undefined,
      invoice_status: invoiceStatus.trim() || undefined,
      payment_status: paymentStatus.trim() || undefined,
      payment_overdue: paymentOverdue === "all" ? undefined : paymentOverdue === "true",
      sort_by: sortBy,
      order,
      limit: 100,
      offset: 0,
    }),
    [customerExternalID, invoiceID, invoiceNumber, lastEventType, invoiceStatus, paymentStatus, paymentOverdue, sortBy, order],
  );

  const exportURL = useMemo(() => {
    if (!apiBaseURL) return "";
    const query = new URLSearchParams();
    if (filters.customer_external_id) query.set("customer_external_id", filters.customer_external_id);
    if (filters.invoice_id) query.set("invoice_id", filters.invoice_id);
    if (filters.invoice_number) query.set("invoice_number", filters.invoice_number);
    if (filters.last_event_type) query.set("last_event_type", filters.last_event_type);
    if (filters.invoice_status) query.set("invoice_status", filters.invoice_status);
    if (filters.payment_status) query.set("payment_status", filters.payment_status);
    if (typeof filters.payment_overdue === "boolean") query.set("payment_overdue", String(filters.payment_overdue));
    if (filters.sort_by) query.set("sort_by", filters.sort_by);
    if (filters.order) query.set("order", filters.order);
    query.set("limit", String(filters.limit ?? 100));
    query.set("offset", String(filters.offset ?? 0));
    query.set("format", "csv");
    return `${apiBaseURL}/v1/payments?${query.toString()}`;
  }, [apiBaseURL, filters]);

  const paymentsQuery = useQuery({
    queryKey: ["payments", apiBaseURL, filters],
    queryFn: () => fetchPayments({ runtimeBaseURL: apiBaseURL, filters }),
    enabled: isTenantSession,
  });

  const items = useMemo(() => paymentsQuery.data?.items ?? [], [paymentsQuery.data?.items]);
  const stats = useMemo(
    () => ({
      total: items.length,
      failed: items.filter((item) => (item.payment_status || "").toLowerCase() === "failed").length,
      overdue: items.filter((item) => Boolean(item.payment_overdue)).length,
      actionRequired: items.filter((item) => ["failed", "pending"].includes((item.payment_status || "").toLowerCase())).length,
    }),
    [items],
  );

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Workspace" }, { label: "Payments" }]} />

        {isTenantSession ? <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payments</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Payments</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Track payment status, spot overdue invoices, and trigger retries when needed.
              </p>
            </div>
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Payments are workspace-scoped. Sign in with a workspace account to inspect payment state, lifecycle signals, and recovery readiness."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <>
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="Visible payments" value={stats.total} />
              <MetricCard label="Failed" value={stats.failed} />
              <MetricCard label="Overdue" value={stats.overdue} />
              <MetricCard label="Need operator action" value={stats.actionRequired} />
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Filter and inspect</h2>
            </div>
            <div className="grid gap-3 lg:grid-cols-3">
              <input
                value={customerExternalID}
                onChange={(event) => setCustomerExternalID(event.target.value)}
                placeholder="Customer external ID"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <input
                value={invoiceID}
                onChange={(event) => setInvoiceID(event.target.value)}
                placeholder="Invoice ID"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <input
                value={invoiceNumber}
                onChange={(event) => setInvoiceNumber(event.target.value)}
                placeholder="Invoice number"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>
            <div className="grid gap-3 lg:grid-cols-4">
              <input
                value={lastEventType}
                onChange={(event) => setLastEventType(event.target.value)}
                placeholder="Last event type"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <select
                value={invoiceStatus}
                onChange={(event) => setInvoiceStatus(event.target.value)}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="">All invoice statuses</option>
                {invoiceStatusOptions.map((option) => (
                  <option key={option} value={option}>
                    {formatState(option)}
                  </option>
                ))}
              </select>
              <select
                value={paymentStatus}
                onChange={(event) => setPaymentStatus(event.target.value)}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="">All payment statuses</option>
                {paymentStatusOptions.map((option) => (
                  <option key={option} value={option}>
                    {formatState(option)}
                  </option>
                ))}
              </select>
              <select
                value={paymentOverdue}
                onChange={(event) => setPaymentOverdue(event.target.value as "all" | "true" | "false")}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="all">All due states</option>
                <option value="true">Overdue only</option>
                <option value="false">Not overdue</option>
              </select>
            </div>
            <div className="grid gap-3 lg:grid-cols-3">
              <select
                value={sortBy}
                onChange={(event) => setSortBy(event.target.value as PaymentFilters["sort_by"])}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                {sortOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <select
                value={order}
                onChange={(event) => setOrder(event.target.value as PaymentFilters["order"])}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                {orderOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={() => {
                  setCustomerExternalID("");
                  setInvoiceID("");
                  setInvoiceNumber("");
                  setLastEventType("");
                  setInvoiceStatus("");
                  setPaymentStatus("");
                  setPaymentOverdue("all");
                  setSortBy("last_event_at");
                  setOrder("desc");
                }}
                className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
              >
                Clear filters
              </button>
              <a
                href={exportURL || undefined}
                download="payments.csv"
                className={`inline-flex h-10 items-center justify-center rounded-lg border px-4 text-sm font-medium transition ${
                  exportURL
                    ? "border-slate-900 bg-slate-900 text-white hover:bg-slate-800"
                    : "cursor-not-allowed border-slate-200 bg-slate-100 text-slate-400"
                }`}
              >
                Export CSV
              </a>
            </div>
          </div>

              <div className="mt-5 grid gap-3">
                {paymentsQuery.isLoading ? (
                  <LoadingState />
                ) : items.length === 0 ? (
                  <EmptyState />
                ) : (
                  items.map((item) => <PaymentRow key={item.invoice_id} item={item} />)
                )}
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}

function PaymentRow({ item }: { item: PaymentSummary }) {
  const diagnosis = billingFailureDiagnosis(item);
  const primaryLabel = item.customer_display_name || item.customer_external_id || "Unlinked customer";

  return (
    <Link
      href={`/payments/${encodeURIComponent(item.invoice_id)}`}
      className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-white lg:grid-cols-[minmax(0,1.6fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.9fr)_110px] lg:items-start"
    >
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{item.invoice_number || item.invoice_id}</h3>
        <p className="mt-1 text-sm text-slate-600">
          {primaryLabel} · {formatMoney(item.total_due_amount_cents, item.currency || "USD")}
        </p>
        <div className="mt-3 rounded-xl border border-slate-200 bg-white px-3 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${diagnosisToneClass(diagnosis.tone)}`}>
              {diagnosis.title}
            </span>
          </div>
          <p className="mt-2 text-xs leading-relaxed text-slate-600">{diagnosis.summary}</p>
        </div>
      </div>
      <InventoryCell label="Payment" value={formatState(item.payment_status)} />
      <InventoryCell label="Due state" value={item.payment_overdue ? "Overdue" : "Current"} />
      <InventoryCell label="Last event" value={formatExactTimestamp(item.last_event_at)} />
      <div className="flex items-center justify-between gap-3 lg:justify-end">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">Open</p>
      </div>
    </Link>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function InventoryCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 break-all text-sm font-semibold text-slate-950">{value || "-"}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="grid gap-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.9fr)_110px] lg:items-start">
          <div className="min-w-0">
            <Skeleton className="h-5 w-44" />
            <Skeleton className="mt-2 h-4 w-36" />
            <div className="mt-3 rounded-xl border border-slate-200 bg-white px-3 py-3">
              <Skeleton className="h-5 w-20 rounded-full" />
              <Skeleton className="mt-2 h-3 w-52" />
            </div>
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-14" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-28" />
          </div>
          <div className="flex items-center justify-end">
            <Skeleton className="h-3.5 w-8" />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No payments match the current filters.</p>
      <p className="mt-2">Clear filters or wait for invoice payment activity to appear in the workspace billing history.</p>
    </div>
  );
}
