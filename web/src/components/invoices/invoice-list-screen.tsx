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
import { fetchInvoices } from "@/lib/api";
import { billingFailureDiagnosis } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { type InvoiceSummary, type InvoiceStatusFilters } from "@/lib/types";
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

export function InvoiceListScreen() {
  const searchParams = useSearchParams();
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [customerExternalID, setCustomerExternalID] = useState(searchParams.get("customer_external_id") || "");
  const [invoiceStatus, setInvoiceStatus] = useState(searchParams.get("invoice_status") || "");
  const [paymentStatus, setPaymentStatus] = useState(searchParams.get("payment_status") || "");
  const [paymentOverdue, setPaymentOverdue] = useState<"all" | "true" | "false">("all");
  const [sortBy, setSortBy] = useState<InvoiceStatusFilters["sort_by"]>("last_event_at");
  const [order, setOrder] = useState<InvoiceStatusFilters["order"]>("desc");

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
  const stats = useMemo(
    () => ({
      total: items.length,
      paid: items.filter((item) => (item.payment_status || "").toLowerCase() === "succeeded").length,
      overdue: items.filter((item) => Boolean(item.payment_overdue)).length,
      actionRequired: items.filter((item) => ["failed", "pending"].includes((item.payment_status || "").toLowerCase())).length,
    }),
    [items],
  );

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Workspace" }, { label: "Invoices" }]} />

        {isTenantSession ? <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Invoices</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Invoices</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                View invoice status, amounts due, and payment state across all customers.
              </p>
            </div>
          </div>
        </section> : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Invoices are workspace-scoped. Sign in with a workspace account to view invoice and payment status."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <>
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="Visible invoices" value={stats.total} />
              <MetricCard label="Paid" value={stats.paid} />
              <MetricCard label="Overdue" value={stats.overdue} />
              <MetricCard label="Needs attention" value={stats.actionRequired} />
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Invoices</p>
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
                value={invoiceStatus}
                onChange={(event) => setInvoiceStatus(event.target.value)}
                placeholder="Invoice status"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
              <input
                value={paymentStatus}
                onChange={(event) => setPaymentStatus(event.target.value)}
                placeholder="Payment status"
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>
            <div className="grid gap-3 lg:grid-cols-3">
              <select
                value={paymentOverdue}
                onChange={(event) => setPaymentOverdue(event.target.value as "all" | "true" | "false")}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="all">All due states</option>
                <option value="true">Overdue only</option>
                <option value="false">Not overdue</option>
              </select>
              <select
                value={sortBy}
                onChange={(event) => setSortBy(event.target.value as InvoiceStatusFilters["sort_by"])}
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
                onChange={(event) => setOrder(event.target.value as InvoiceStatusFilters["order"])}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                {orderOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>
          </div>

              <div className="mt-5 grid gap-3">
                {invoicesQuery.isLoading ? (
                  <LoadingState />
                ) : items.length === 0 ? (
                  <EmptyState />
                ) : (
                  items.map((item) => <InvoiceRow key={item.invoice_id} item={item} />)
                )}
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}

function InvoiceRow({ item }: { item: InvoiceSummary }) {
  const diagnosis = billingFailureDiagnosis(item);
  const primaryLabel = item.customer_display_name || item.customer_external_id || "Unlinked customer";

  return (
    <Link
      href={`/invoices/${encodeURIComponent(item.invoice_id)}`}
      className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-white lg:grid-cols-[minmax(0,1.6fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_110px] lg:items-start"
    >
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{item.invoice_number || item.invoice_id}</h3>
        <p className="mt-1 text-sm text-slate-600">
          {primaryLabel} · {formatMoney(item.total_amount_cents, item.currency || "USD")}
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
      <InventoryCell label="Invoice" value={formatInvoiceState(item.invoice_status)} />
      <InventoryCell label="Payment" value={formatInvoiceState(item.payment_status)} />
      <InventoryCell label="Due state" value={item.payment_overdue ? "Overdue" : "Current"} />
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
        <div key={i} className="grid gap-3 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_minmax(0,0.8fr)_110px] lg:items-start">
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
            <Skeleton className="h-3 w-14" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-16" />
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
      <p className="font-semibold text-slate-950">No invoices yet.</p>
      <p className="mt-2">Invoices appear once a customer has an active subscription and a billing cycle completes. Make sure customers have a billing profile and payment setup done.</p>
      <div className="mt-4 flex flex-wrap gap-3">
        <Link href="/subscriptions" className="inline-flex h-9 items-center rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-stone-50">View subscriptions</Link>
        <Link href="/customers" className="inline-flex h-9 items-center rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-stone-50">View customers</Link>
      </div>
    </div>
  );
}
