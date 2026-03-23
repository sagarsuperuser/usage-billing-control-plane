"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle, Mail, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { BillingActivityTimeline } from "@/components/billing/billing-activity-timeline";
import { BillingFailureDiagnosisCard } from "@/components/billing/billing-failure-diagnosis";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { DunningSummaryPanel } from "@/components/billing/dunning-summary-panel";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  fetchInvoiceCreditNotes,
  fetchInvoiceDetail,
  fetchInvoiceEvents,
  fetchInvoicePaymentReceipts,
  fetchDunningRunDetail,
  resendCreditNoteEmail,
  resendInvoiceEmail,
  resendPaymentReceiptEmail,
  sendCollectPaymentReminder,
  retryInvoicePayment,
} from "@/lib/api";
import { billingActionConfig, billingFailureDiagnosis, formatBillingState } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";

export function InvoiceDetailScreen({ invoiceID }: { invoiceID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const invoiceQuery = useQuery({
    queryKey: ["invoice-detail", apiBaseURL, invoiceID],
    queryFn: () => fetchInvoiceDetail({ runtimeBaseURL: apiBaseURL, invoiceID }),
    enabled: isTenantSession && invoiceID.trim().length > 0,
  });
  const paymentReceiptsQuery = useQuery({
    queryKey: ["invoice-payment-receipts", apiBaseURL, invoiceID],
    queryFn: () => fetchInvoicePaymentReceipts({ runtimeBaseURL: apiBaseURL, invoiceID }),
    enabled: isTenantSession && invoiceID.trim().length > 0,
  });
  const creditNotesQuery = useQuery({
    queryKey: ["invoice-credit-notes", apiBaseURL, invoiceID],
    queryFn: () => fetchInvoiceCreditNotes({ runtimeBaseURL: apiBaseURL, invoiceID }),
    enabled: isTenantSession && invoiceID.trim().length > 0,
  });
  const invoiceEventsQuery = useQuery({
    queryKey: ["invoice-events", apiBaseURL, invoiceID, "invoice-detail"],
    queryFn: () =>
      fetchInvoiceEvents({
        runtimeBaseURL: apiBaseURL,
        invoiceID,
        sortBy: "occurred_at",
        order: "desc",
        limit: 15,
        offset: 0,
      }),
    enabled: isTenantSession && invoiceID.trim().length > 0,
  });

  const retryMutation = useMutation({
    mutationFn: () => retryInvoicePayment({ runtimeBaseURL: apiBaseURL, csrfToken, invoiceID }),
    onSuccess: async () => {
      await Promise.all([invoiceQuery.refetch(), invoiceEventsQuery.refetch()]);
    },
  });
  const resendEmailMutation = useMutation({
    mutationFn: () => resendInvoiceEmail({ runtimeBaseURL: apiBaseURL, csrfToken, invoiceID }),
  });
  const reminderMutation = useMutation({
    mutationFn: (runID: string) => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await invoiceQuery.refetch();
    },
  });

  const invoice = invoiceQuery.data;
  const actionConfig = invoice ? billingActionConfig(invoice) : null;
  const diagnosis = invoice ? billingFailureDiagnosis(invoice) : null;
  const dunningRunID = invoice?.dunning?.run_id;
  const dunningDetailQuery = useQuery({
    queryKey: ["dunning-run-detail", apiBaseURL, dunningRunID],
    queryFn: () => fetchDunningRunDetail({ runtimeBaseURL: apiBaseURL, runID: dunningRunID as string }),
    enabled: isTenantSession && Boolean(dunningRunID),
  });
  const timelineLoading = invoiceEventsQuery.isLoading || (Boolean(dunningRunID) && dunningDetailQuery.isLoading);
  const timelineError =
    invoiceEventsQuery.error instanceof Error
      ? invoiceEventsQuery.error.message
      : dunningDetailQuery.error instanceof Error
        ? dunningDetailQuery.error.message
        : undefined;

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
                    <Badge>{formatBillingState(invoice.invoice_status)}</Badge>
                    <Badge>{formatBillingState(invoice.payment_status)}</Badge>
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
                    onClick={() => resendEmailMutation.mutate()}
                    disabled={!canWrite || !csrfToken || resendEmailMutation.isPending}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {resendEmailMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Mail className="h-4 w-4" />}
                    Resend invoice email
                  </button>
                  {actionConfig?.emphasizeRetry ? (
                    <button
                      type="button"
                      onClick={() => retryMutation.mutate()}
                      disabled={!canWrite || !csrfToken || retryMutation.isPending}
                      className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                      Retry payment
                    </button>
                  ) : null}
                  {invoice.customer_external_id && invoice.lifecycle?.recommended_action === "collect_payment" ? (
                    <Link
                      href={`/customers/${encodeURIComponent(invoice.customer_external_id)}#payment-collection`}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
                    >
                      Open customer collection path
                    </Link>
                  ) : null}
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard label="Invoice total" value={formatMoney(invoice.total_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Amount due" value={formatMoney(invoice.total_due_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Amount paid" value={formatMoney(invoice.total_paid_amount_cents, invoice.currency || "USD")} />
              <MetricCard label="Currency" value={invoice.currency || "-"} />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
              <div className="min-w-0 grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Financial state</p>
                  <div className="mt-5 grid gap-3 lg:grid-cols-2">
                    <StatusCard label="Invoice status" value={formatBillingState(invoice.invoice_status)} />
                    <StatusCard label="Payment status" value={formatBillingState(invoice.payment_status)} />
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

                {invoice.lifecycle ? (
                  <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment lifecycle</p>
                    <div className="mt-5 grid gap-3 lg:grid-cols-2">
                      <StatusCard label="Recommended action" value={formatBillingState(invoice.lifecycle.recommended_action)} />
                      <StatusCard label="Requires action" value={invoice.lifecycle.requires_action ? "Yes" : "No"} />
                      <StatusCard label="Last event" value={formatBillingState(invoice.last_event_type || invoice.lifecycle.last_event_type)} />
                      <StatusCard label="Last event at" value={formatExactTimestamp(invoice.last_event_at || invoice.lifecycle.last_event_at)} />
                    </div>
                    <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                      <p className="font-semibold text-slate-950">Operator guidance</p>
                      <p className="mt-2">{invoice.lifecycle.recommended_action_note || "No specific action is currently recommended."}</p>
                    </div>
                  </section>
                ) : null}

                {diagnosis ? <BillingFailureDiagnosisCard diagnosis={diagnosis} /> : null}

                <BillingActivityTimeline
                  webhookEvents={invoiceEventsQuery.data?.items}
                  dunningDetail={dunningDetailQuery.data}
                  dunningRunHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
                  loading={timelineLoading}
                  error={timelineError}
                />

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Linked billing documents</p>
                  <div className="mt-5 grid gap-6">
                    <div>
                      <div className="flex items-center justify-between gap-3">
                        <h2 className="text-sm font-semibold text-slate-950">Payment receipts</h2>
                        <span className="text-xs text-slate-500">{paymentReceiptsQuery.data?.length ?? 0} linked</span>
                      </div>
                      <div className="mt-3 grid gap-3">
                        {paymentReceiptsQuery.isLoading ? (
                          <InlineLoadingState label="Loading linked payment receipts" />
                        ) : paymentReceiptsQuery.isError ? (
                          <InlineErrorState label="Payment receipts could not be loaded." />
                        ) : paymentReceiptsQuery.data && paymentReceiptsQuery.data.length > 0 ? (
                          paymentReceiptsQuery.data.map((item) => (
                            <BillingDocumentRow
                              key={item.id}
                              title={item.number || item.id}
                              subtitle={item.payment_status ? `Payment ${formatBillingState(item.payment_status)}` : "Linked payment receipt"}
                              meta={[
                                item.amount_cents !== undefined ? formatMoney(item.amount_cents, item.currency || invoice.currency || "USD") : undefined,
                                formatExactTimestamp(item.created_at),
                              ]}
                              fileURL={item.file_url}
                              resendLabel="Resend receipt email"
                              canWrite={canWrite && Boolean(csrfToken)}
                              onResend={() => resendPaymentReceiptEmail({ runtimeBaseURL: apiBaseURL, csrfToken, paymentReceiptID: item.id })}
                            />
                          ))
                        ) : (
                          <EmptyLinkedDocumentState label="No payment receipts are linked to this invoice yet." />
                        )}
                      </div>
                    </div>

                    <div>
                      <div className="flex items-center justify-between gap-3">
                        <h2 className="text-sm font-semibold text-slate-950">Credit notes</h2>
                        <span className="text-xs text-slate-500">{creditNotesQuery.data?.length ?? 0} linked</span>
                      </div>
                      <div className="mt-3 grid gap-3">
                        {creditNotesQuery.isLoading ? (
                          <InlineLoadingState label="Loading linked credit notes" />
                        ) : creditNotesQuery.isError ? (
                          <InlineErrorState label="Credit notes could not be loaded." />
                        ) : creditNotesQuery.data && creditNotesQuery.data.length > 0 ? (
                          creditNotesQuery.data.map((item) => (
                            <BillingDocumentRow
                              key={item.id}
                              title={item.number || item.id}
                              subtitle={[
                                item.credit_status ? `Credit ${formatBillingState(item.credit_status)}` : "",
                                item.refund_status ? `Refund ${formatBillingState(item.refund_status)}` : "",
                              ].filter(Boolean).join(" • ") || "Linked credit note"}
                              meta={[
                                item.total_amount_cents !== undefined ? formatMoney(item.total_amount_cents, item.currency || invoice.currency || "USD") : undefined,
                                formatExactTimestamp(item.issuing_date || item.created_at),
                              ]}
                              fileURL={item.file_url}
                              resendLabel="Resend credit note email"
                              canWrite={canWrite && Boolean(csrfToken)}
                              onResend={() => resendCreditNoteEmail({ runtimeBaseURL: apiBaseURL, csrfToken, creditNoteID: item.id })}
                            />
                          ))
                        ) : (
                          <EmptyLinkedDocumentState label="No credit notes are linked to this invoice." />
                        )}
                      </div>
                    </div>
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Advanced actions</p>
                  {resendEmailMutation.isSuccess ? (
                    <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-800">
                      <p className="font-semibold text-emerald-900">Invoice email dispatched</p>
                      <p className="mt-2">Alpha accepted the resend action and delegated document delivery through Lago.</p>
                    </div>
                  ) : null}
                  {resendEmailMutation.isError ? (
                    <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Invoice email could not be dispatched</p>
                      <p className="mt-2">{resendEmailMutation.error instanceof Error ? resendEmailMutation.error.message : "Invoice resend failed."}</p>
                    </div>
                  ) : null}
                  <div className="mt-4 flex flex-wrap gap-3">
                    <Link
                      href={`/invoice-explainability?invoice_id=${encodeURIComponent(invoice.invoice_id)}`}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      Open explainability
                    </Link>
                    <Link
                      href={`/payments/${encodeURIComponent(invoice.invoice_id)}`}
                      className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                    >
                      Open payment operations
                    </Link>
                  </div>
                </section>
              </div>

              <aside className="min-w-0 grid gap-5 self-start">
                <DunningSummaryPanel
                  summary={invoice.dunning}
                  canWrite={canWrite && Boolean(csrfToken)}
                  sendingReminder={reminderMutation.isPending}
                  onSendReminder={dunningRunID ? () => reminderMutation.mutate(dunningRunID) : undefined}
                  runHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
                />
                {invoice.lifecycle ? (
                  <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Retry and recovery</p>
                    <div className="mt-4 grid gap-3">
                      <MetaItem label="Recommended action" value={formatBillingState(invoice.lifecycle.recommended_action)} />
                      <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700">
                        <p className="font-semibold text-slate-950">{actionConfig?.title || "No action required"}</p>
                        <p className="mt-2">{actionConfig?.body || "No payment action guidance is currently available."}</p>
                      </div>
                      {actionConfig?.emphasizeRetry ? (
                        <button
                          type="button"
                          onClick={() => retryMutation.mutate()}
                          disabled={!canWrite || !csrfToken || retryMutation.isPending}
                          className="inline-flex h-10 w-full max-w-full items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                          Retry collection
                        </button>
                      ) : null}
                      {invoice.customer_external_id && invoice.lifecycle.recommended_action === "collect_payment" ? (
                        <Link
                          href={`/customers/${encodeURIComponent(invoice.customer_external_id)}#payment-collection`}
                          className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                        >
                          Open customer collection path
                        </Link>
                      ) : null}
                      {actionConfig?.showRecovery && invoice.customer_external_id ? (
                        <Link
                          href={`/replay-operations?customer_id=${encodeURIComponent(invoice.customer_external_id)}&status=failed`}
                          className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                        >
                          Open recovery tools
                        </Link>
                      ) : null}
                      {actionConfig?.showExplainability ? (
                        <Link
                          href={`/invoice-explainability?invoice_id=${encodeURIComponent(invoice.invoice_id)}`}
                          className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                        >
                          Open explainability
                        </Link>
                      ) : null}
                      <Link
                        href={`/payments/${encodeURIComponent(invoice.invoice_id)}`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                      >
                        Open payment operations
                      </Link>
                    </div>
                  </section>
                ) : null}
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={invoice.customer_display_name || "-"} />
                    <MetaItem label="Customer external ID" value={invoice.customer_external_id || "-"} mono />
                    {invoice.customer_external_id ? (
                      <Link
                        href={`/customers/${encodeURIComponent(invoice.customer_external_id)}`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
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

function InlineLoadingState({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      {label}
    </div>
  );
}

function InlineErrorState({ label }: { label: string }) {
  return (
    <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
      {label}
    </div>
  );
}

function EmptyLinkedDocumentState({ label }: { label: string }) {
  return (
    <div className="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
      {label}
    </div>
  );
}

function BillingDocumentRow({
  title,
  subtitle,
  meta,
  fileURL,
  resendLabel,
  canWrite,
  onResend,
}: {
  title: string;
  subtitle: string;
  meta: Array<string | undefined>;
  fileURL?: string;
  resendLabel: string;
  canWrite: boolean;
  onResend: () => Promise<unknown>;
}) {
  const resendMutation = useMutation({
    mutationFn: onResend,
  });

  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-slate-950">{title}</p>
          <p className="mt-1 text-sm text-slate-600">{subtitle}</p>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
            {meta.filter(Boolean).map((item) => (
              <span key={item}>{item}</span>
            ))}
          </div>
          {resendMutation.isSuccess ? (
            <p className="mt-2 text-xs text-emerald-700">Email dispatch accepted.</p>
          ) : null}
          {resendMutation.isError ? (
            <p className="mt-2 text-xs text-amber-700">
              {resendMutation.error instanceof Error ? resendMutation.error.message : "Dispatch failed."}
            </p>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          {fileURL ? (
            <a
              href={fileURL}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-9 items-center rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              Open file
            </a>
          ) : null}
          <button
            type="button"
            onClick={() => resendMutation.mutate()}
            disabled={!canWrite || resendMutation.isPending}
            className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {resendMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Mail className="h-4 w-4" />}
            {resendLabel}
          </button>
        </div>
      </div>
    </div>
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
