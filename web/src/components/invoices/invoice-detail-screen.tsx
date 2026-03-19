"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchInvoiceDetail, retryInvoicePayment } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

export function InvoiceDetailScreen({ invoiceID }: { invoiceID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();

  const invoiceQuery = useQuery({
    queryKey: ["invoice-detail", apiBaseURL, invoiceID],
    queryFn: () => fetchInvoiceDetail({ runtimeBaseURL: apiBaseURL, invoiceID }),
    enabled: isAuthenticated && scope === "tenant" && invoiceID.trim().length > 0,
  });

  const retryMutation = useMutation({
    mutationFn: () => retryInvoicePayment({ runtimeBaseURL: apiBaseURL, csrfToken, invoiceID }),
    onSuccess: async () => {
      await invoiceQuery.refetch();
    },
  });

  const invoice = invoiceQuery.data;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Tenant" },
            { href: "/invoices", label: "Invoices" },
            { label: invoice?.invoice_number || invoiceID },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Invoice detail is tenant-scoped. Sign in with a tenant account to inspect financial state and invoice actions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {invoiceQuery.isLoading ? (
          <LoadingPanel label="Loading invoice detail" />
        ) : invoiceQuery.isError || !invoice ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Invoice</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Invoice not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested invoice could not be loaded from the tenant APIs.</p>
            <Link
              href="/invoices"
              className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to invoices
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Invoice</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{invoice.invoice_number || invoice.invoice_id}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{invoice.invoice_id}</span>
                    <Badge>{formatState(invoice.invoice_status)}</Badge>
                    <Badge>{formatState(invoice.payment_status)}</Badge>
                    {invoice.payment_overdue ? <Badge>Overdue</Badge> : null}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Link
                    href="/invoices"
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Back to invoices
                  </Link>
                  <button
                    type="button"
                    onClick={() => retryMutation.mutate()}
                    disabled={!canWrite || !csrfToken || retryMutation.isPending}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                    Retry payment
                  </button>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="Invoice total" value={formatMoney(invoice.total_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Amount due" value={formatMoney(invoice.total_due_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Amount paid" value={formatMoney(invoice.total_paid_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Currency" value={invoice.currency || "-"} />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_420px]">
              <div className="grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Financial state</p>
                  <div className="mt-5 grid gap-3 lg:grid-cols-2">
                    <StatusCard label="Invoice status" value={formatState(invoice.invoice_status)} />
                    <StatusCard label="Payment status" value={formatState(invoice.payment_status)} />
                    <StatusCard label="Issued" value={formatExactTimestamp(invoice.issuing_date || invoice.created_at)} />
                    <StatusCard label="Payment due" value={formatExactTimestamp(invoice.payment_due_date)} />
                  </div>
                  {invoice.last_payment_error ? (
                    <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Last payment error</p>
                      <p className="mt-2">{invoice.last_payment_error}</p>
                    </div>
                  ) : null}
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Advanced actions</p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    <Link
                      href={`/invoice-explainability?invoice_id=${encodeURIComponent(invoice.invoice_id)}`}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      Open explainability
                    </Link>
                    <Link
                      href="/payment-operations"
                      className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      Open payment operations
                    </Link>
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={invoice.customer_display_name || "-"} />
                    <MetaItem label="Customer external ID" value={invoice.customer_external_id || "-"} mono />
                    {invoice.customer_external_id ? (
                      <Link
                        href={`/customers/${encodeURIComponent(invoice.customer_external_id)}`}
                        className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
                      >
                        Open customer
                      </Link>
                    ) : null}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Invoice metadata</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Billing entity" value={invoice.billing_entity_code || "-"} />
                    <MetaItem label="Invoice type" value={invoice.invoice_type || "-"} />
                    <MetaItem label="Updated" value={formatExactTimestamp(invoice.updated_at)} />
                    <MetaItem label="File URL" value={invoice.file_url || "-"} />
                  </div>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function StatusCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-semibold text-slate-950">{value || "-"}</p>
    </div>
  );
}

function Badge({ children }: { children: string }) {
  return (
    <span className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
      {children}
    </span>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
