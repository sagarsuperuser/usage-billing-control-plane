"use client";

import Link from "next/link";
import { Skeleton } from "@/components/ui/skeleton";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
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
  const [page, setPage] = useState(1);

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

  const PAGE_SIZE = 20;
  const paginatedItems = useMemo(() => items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [items, page]);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Payments" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Payments are workspace-scoped. Sign in with a workspace account to view payment status."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Payments{items.length > 0 ? ` (${items.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <input
                  value={customerExternalID}
                  onChange={(event) => { setCustomerExternalID(event.target.value); setPage(1); }}
                  placeholder="Customer ID..."
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <select
                  value={invoiceStatus}
                  onChange={(event) => { setInvoiceStatus(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All invoice statuses</option>
                  {invoiceStatusOptions.map((option) => (
                    <option key={option} value={option}>{formatState(option)}</option>
                  ))}
                </select>
                <select
                  value={paymentStatus}
                  onChange={(event) => { setPaymentStatus(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All payment statuses</option>
                  {paymentStatusOptions.map((option) => (
                    <option key={option} value={option}>{formatState(option)}</option>
                  ))}
                </select>
                <select
                  value={sortBy}
                  onChange={(event) => setSortBy(event.target.value as PaymentFilters["sort_by"])}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {sortOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
                <select
                  value={order}
                  onChange={(event) => setOrder(event.target.value as PaymentFilters["order"])}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {orderOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
                <a
                  href={exportURL || undefined}
                  download="payments.csv"
                  className={`inline-flex h-8 items-center rounded-lg border px-3 text-sm font-medium transition ${
                    exportURL
                      ? "border-slate-900 bg-slate-900 text-white hover:bg-slate-800"
                      : "cursor-not-allowed border-slate-200 bg-slate-100 text-slate-400"
                  }`}
                >
                  Export CSV
                </a>
              </div>
            </div>
            {paymentsQuery.isLoading ? (
              <LoadingState />
            ) : items.length === 0 ? (
              <EmptyState />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Invoice #</th>
                    <th className="px-4 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Amount Due</th>
                    <th className="px-4 py-2.5 font-semibold">Payment Status</th>
                    <th className="px-4 py-2.5 font-semibold">Last Event</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {paginatedItems.map((item) => {
                    const diagnosis = billingFailureDiagnosis(item);
                    return (
                      <tr key={item.invoice_id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link href={`/payments/${encodeURIComponent(item.invoice_id)}`} className="block">
                            <p className="font-medium text-slate-900">{item.invoice_number || item.invoice_id}</p>
                          </Link>
                        </td>
                        <td className="px-4 py-3 text-slate-600">{item.customer_display_name || item.customer_external_id || "—"}</td>
                        <td className="px-4 py-3 text-slate-900 font-medium">{formatMoney(item.total_due_amount_cents, item.currency || "USD")}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${diagnosisToneClass(diagnosis.tone)}`}>
                            {formatState(item.payment_status)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-slate-500 text-xs">{formatExactTimestamp(item.last_event_at)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <Pagination page={page} pageSize={PAGE_SIZE} total={items.length} onPageChange={setPage} />
              </>
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
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 px-5 py-3">
          <div className="flex-1"><Skeleton className="h-4 w-28" /></div>
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-3 w-16" />
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-20" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No payments</p>
      <p className="text-xs text-slate-500">Payments appear once invoices are finalized and collection begins.</p>
      <Link href="/invoices" className="inline-flex h-9 items-center rounded-lg border border-stone-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-stone-50">
        View invoices
      </Link>
    </div>
  );
}
