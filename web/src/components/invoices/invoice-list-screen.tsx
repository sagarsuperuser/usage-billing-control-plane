import { Link } from "@tanstack/react-router";

import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { fetchInvoices } from "@/lib/api";
import { billingFailureDiagnosis } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { type InvoiceStatusFilters } from "@/lib/types";
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

function formatInvoiceState(value?: string): string {
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

function paymentTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "succeeded":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending":
    case "processing":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "failed":
    case "requires_action":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-stone-200 bg-slate-50 text-slate-700";
  }
}

export function InvoiceListScreen() {
  const searchParams = useSearchParamsCompat();
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [customerExternalID, setCustomerExternalID] = useState(searchParams.get("customer_external_id") || "");
  const [invoiceStatus, setInvoiceStatus] = useState(searchParams.get("invoice_status") || "");
  const [paymentStatus, setPaymentStatus] = useState(searchParams.get("payment_status") || "");
  const [paymentOverdue, setPaymentOverdue] = useState<"all" | "true" | "false">("all");
  const [sortBy, setSortBy] = useState<InvoiceStatusFilters["sort_by"]>("last_event_at");
  const [order, setOrder] = useState<InvoiceStatusFilters["order"]>("desc");
  const [page, setPage] = useState(1);

  const filters = useMemo<InvoiceStatusFilters>(
    () => ({
      customer_external_id: customerExternalID.trim() || undefined,
      invoice_status: invoiceStatus.trim() || undefined,
      payment_status: paymentStatus.trim() || undefined,
      payment_overdue: paymentOverdue === "all" ? undefined : paymentOverdue === "true",
      sort_by: sortBy,
      order,
      limit: 100,
      offset: 0,
    }),
    [customerExternalID, invoiceStatus, paymentStatus, paymentOverdue, sortBy, order],
  );

  const invoicesQuery = useQuery({
    queryKey: ["invoices", apiBaseURL, filters],
    queryFn: () => fetchInvoices({ runtimeBaseURL: apiBaseURL, filters }),
    enabled: isTenantSession,
  });

  const items = useMemo(() => invoicesQuery.data?.items ?? [], [invoicesQuery.data?.items]);

  const PAGE_SIZE = 20;
  const paginatedItems = useMemo(() => items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [items, page]);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Invoices" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Invoices{items.length > 0 ? ` (${items.length})` : ""}</h1>
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
                  <option value="finalized">Finalized</option>
                  <option value="draft">Draft</option>
                  <option value="voided">Voided</option>
                  <option value="pending">Pending</option>
                </select>
                <select
                  value={paymentStatus}
                  onChange={(event) => { setPaymentStatus(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All payment statuses</option>
                  <option value="succeeded">Succeeded</option>
                  <option value="pending">Pending</option>
                  <option value="failed">Failed</option>
                  <option value="processing">Processing</option>
                  <option value="requires_action">Requires action</option>
                </select>
                <select
                  value={paymentOverdue}
                  onChange={(event) => { setPaymentOverdue(event.target.value as "all" | "true" | "false"); setPage(1); }}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="all">All due states</option>
                  <option value="true">Overdue only</option>
                  <option value="false">Not overdue</option>
                </select>
                <select
                  value={sortBy}
                  onChange={(event) => setSortBy(event.target.value as InvoiceStatusFilters["sort_by"])}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {sortOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
                <select
                  value={order}
                  onChange={(event) => setOrder(event.target.value as InvoiceStatusFilters["order"])}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  {orderOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              </div>
            </div>
            {sessionLoading || invoicesQuery.isLoading ? (
              <LoadingState />
            ) : items.length === 0 ? (
              <EmptyState title="No invoices yet" description="Invoices are generated automatically from subscriptions." />
            ) : (
              <>
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Invoice #</th>
                    <th className="px-4 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Amount</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Payment</th>
                    <th className="px-4 py-2.5 font-semibold">Date</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {paginatedItems.map((item) => {
                    const diagnosis = billingFailureDiagnosis(item);
                    return (
                      <tr key={item.invoice_id} className="transition hover:bg-stone-50">
                        <td className="px-5 py-3">
                          <Link to={`/invoices/${encodeURIComponent(item.invoice_id)}`} className="block">
                            <p className="font-medium text-slate-900">{item.invoice_number || item.invoice_id}</p>
                          </Link>
                        </td>
                        <td className="px-4 py-3 text-slate-600">{item.customer_display_name || item.customer_external_id || "—"}</td>
                        <td className="px-4 py-3 text-slate-900 font-medium">{formatMoney(item.total_amount_cents, item.currency || "USD")}</td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${diagnosisToneClass(diagnosis.tone)}`}>
                            {formatInvoiceState(item.invoice_status)}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${paymentTone(item.payment_status)}`}>
                            {formatInvoiceState(item.payment_status)}
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
          <Skeleton className="h-4 w-14 rounded-full" />
          <Skeleton className="h-3 w-20" />
        </div>
      ))}
    </div>
  );
}
