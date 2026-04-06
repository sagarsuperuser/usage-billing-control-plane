
import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { FileText, Mail, RefreshCw, X } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";

import { BillingActivityTimeline } from "@/components/billing/billing-activity-timeline";
import { BillingFailureDiagnosisCard } from "@/components/billing/billing-failure-diagnosis";
import { BillingFailureEvidence } from "@/components/billing/billing-failure-evidence";
import { DunningSummaryPanel } from "@/components/billing/dunning-summary-panel";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { LoadingSkeleton } from "@/components/ui/loading-skeleton";
import { PageContainer } from "@/components/ui/page-container";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
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
import { billingActionConfig, billingFailureDiagnosis, billingFailureEvidence, formatBillingState } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

export function InvoiceDetailScreen({ invoiceID }: { invoiceID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, isLoading: _sessionLoading, scope } = useUISession();
  const [showPDF, setShowPDF] = useState(false);
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
      showSuccess("Payment retry initiated");
      await Promise.all([invoiceQuery.refetch(), invoiceEventsQuery.refetch()]);
    },
    onError: (err: Error) => showError(err.message),
  });
  const resendEmailMutation = useMutation({
    mutationFn: () => resendInvoiceEmail({ runtimeBaseURL: apiBaseURL, csrfToken, invoiceID }),
    onSuccess: () => showSuccess("Invoice email resent"),
    onError: (err: Error) => showError(err.message),
  });
  const reminderMutation = useMutation({
    mutationFn: (runID: string) => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      showSuccess("Payment reminder sent");
      await invoiceQuery.refetch();
    },
    onError: (err: Error) => showError(err.message),
  });

  const invoice = invoiceQuery.data;
  const actionConfig = invoice ? billingActionConfig(invoice) : null;
  const diagnosis = invoice ? billingFailureDiagnosis(invoice) : null;
  const diagnosisEvidence = invoice ? billingFailureEvidence(invoice) : [];
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
    <PageContainer>
        <AppBreadcrumbs
          items={[
            { href: "/invoices", label: "Invoices" },
            { label: invoice?.invoice_number || invoiceID },
          ]}
        />


        {/* PDF Viewer Modal */}
        {showPDF && (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-6" onClick={(e) => { if (e.target === e.currentTarget) setShowPDF(false); }}>
            <div className="relative flex h-[85vh] w-full max-w-4xl flex-col overflow-hidden rounded-xl bg-surface shadow-2xl">
              <div className="flex items-center justify-between border-b border-border px-5 py-3">
                <p className="text-sm font-semibold text-text-primary">Invoice PDF</p>
                <div className="flex items-center gap-2">
                  <a
                    href={`${apiBaseURL || ""}/v1/invoices/${encodeURIComponent(invoiceID)}/pdf`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex h-7 items-center gap-1.5 rounded border border-border px-2 text-xs font-medium text-text-muted transition hover:bg-surface-tertiary"
                  >
                    Download
                  </a>
                  <Button variant="ghost" size="xs" onClick={() => setShowPDF(false)}>
                    <X className="h-4 w-4" />
                  </Button>
                </div>
              </div>
              <iframe
                src={`${apiBaseURL || ""}/v1/invoices/${encodeURIComponent(invoiceID)}/pdf`}
                className="flex-1 border-none"
                title="Invoice PDF"
              />
            </div>
          </div>
        )}

        {isTenantSession ? (
          invoiceQuery.isLoading ? (
            <LoadingSkeleton variant="card" />
          ) : invoiceQuery.isError || !invoice ? (
            <section className="rounded-lg border border-border bg-surface shadow-sm p-5">
              <p className="text-sm font-semibold text-text-primary">Invoice not available</p>
              <p className="mt-1 text-sm text-text-muted">The requested invoice could not be loaded from the workspace APIs.</p>
            </section>
          ) : (
          <SectionErrorBoundary>
            <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm divide-y divide-border">
              {/* ---- Header ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-text-primary truncate">{invoice.invoice_number || invoice.invoice_id}</h1>
                    <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide border-border bg-surface-secondary text-text-secondary`}>
                      {formatBillingState(invoice.invoice_status)}
                    </span>
                    <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide border-border bg-surface-secondary text-text-secondary`}>
                      {formatBillingState(invoice.payment_status)}
                    </span>
                    {invoice.payment_overdue ? (
                      <span className="shrink-0 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-rose-700">
                        Overdue
                      </span>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      variant="secondary"
                      onClick={() => setShowPDF(true)}
                    >
                      <FileText className="h-3.5 w-3.5" />
                      View PDF
                    </Button>
                    <Button
                      variant="secondary"
                      onClick={() => resendEmailMutation.mutate()}
                      disabled={!canWrite || !csrfToken}
                      loading={resendEmailMutation.isPending}
                    >
                      {!resendEmailMutation.isPending ? <Mail className="h-3.5 w-3.5" /> : null}
                      Resend email
                    </Button>
                    {actionConfig?.emphasizeRetry ? (
                      <Button
                        variant="primary"
                        onClick={() => retryMutation.mutate()}
                        disabled={!canWrite || !csrfToken}
                        loading={retryMutation.isPending}
                      >
                        {!retryMutation.isPending ? <RefreshCw className="h-3.5 w-3.5" /> : null}
                        Retry payment
                      </Button>
                    ) : null}
                    {invoice.customer_external_id && invoice.lifecycle?.recommended_action === "collect_payment" ? (
                      <Link
                        to={`/customers/${encodeURIComponent(invoice.customer_external_id)}#payment-collection`}
                        className="inline-flex h-8 items-center rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800"
                      >
                        Collect payment
                      </Link>
                    ) : null}
                  </div>
                </div>
                <p className="mt-1.5 text-xs text-text-muted">{invoice.invoice_id}</p>
              </div>

              {/* ---- Details ---- */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-text-faint">Invoice total</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{formatMoney(invoice.total_amount_cents, invoice.currency || "USD")}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Amount due</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{formatMoney(invoice.total_due_amount_cents, invoice.currency || "USD")}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Amount paid</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{formatMoney(invoice.total_paid_amount_cents, invoice.currency || "USD")}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Customer</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{invoice.customer_display_name || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Customer ID</dt>
                    <dd className="mt-0.5 text-sm font-mono text-text-secondary">{invoice.customer_external_id || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Currency</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{invoice.currency || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Issued</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{formatExactTimestamp(invoice.issuing_date || invoice.created_at)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Payment due</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{formatExactTimestamp(invoice.payment_due_date)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Invoice type</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{invoice.invoice_type || "-"}</dd>
                  </div>
                </dl>
              </div>

              {/* ---- Last payment error ---- */}
              {invoice.last_payment_error ? (
                <div className="px-5 py-0">
                  <div className="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 my-4 text-sm text-amber-800">
                    <p className="font-medium">Last payment error</p>
                    <p className="mt-0.5 text-xs">{invoice.last_payment_error}</p>
                  </div>
                </div>
              ) : null}

              {/* ---- Lifecycle / diagnosis ---- */}
              {invoice.lifecycle ? (
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-text-faint mb-3">Lifecycle</p>
                  <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs text-text-faint">Action</dt>
                      <dd className="mt-0.5 text-sm text-text-secondary">{formatBillingState(invoice.lifecycle.recommended_action)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-text-faint">Requires action</dt>
                      <dd className="mt-0.5 text-sm text-text-secondary">{invoice.lifecycle.requires_action ? "Yes" : "No"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-text-faint">Last event</dt>
                      <dd className="mt-0.5 text-sm text-text-secondary">{formatBillingState(invoice.last_event_type || invoice.lifecycle.last_event_type)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-text-faint">Last event at</dt>
                      <dd className="mt-0.5 text-sm text-text-secondary">{formatExactTimestamp(invoice.last_event_at || invoice.lifecycle.last_event_at)}</dd>
                    </div>
                  </dl>
                  <p className="mt-3 text-sm text-text-muted">{invoice.lifecycle.recommended_action_note || "No specific action is currently recommended."}</p>
                </div>
              ) : null}
            </div>

            {/* ---- Dunning ---- */}
            <DunningSummaryPanel
              summary={invoice.dunning}
              canWrite={canWrite && Boolean(csrfToken)}
              sendingReminder={reminderMutation.isPending}
              onSendReminder={dunningRunID ? () => reminderMutation.mutate(dunningRunID) : undefined}
              runHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
            />

            {/* ---- Diagnosis ---- */}
            {diagnosis ? <BillingFailureDiagnosisCard diagnosis={diagnosis} /> : null}
            {diagnosisEvidence.length > 0 ? <BillingFailureEvidence items={diagnosisEvidence} /> : null}

            {/* ---- Timeline ---- */}
            <BillingActivityTimeline
              webhookEvents={invoiceEventsQuery.data?.items}
              dunningDetail={dunningDetailQuery.data}
              dunningRunHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
              loading={timelineLoading}
              error={timelineError}
            />

            {/* ---- Linked billing documents ---- */}
            <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm divide-y divide-border">
              <div className="px-5 py-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-xs font-medium text-text-faint">Payment receipts</p>
                  <span className="text-xs text-text-muted">{paymentReceiptsQuery.data?.length ?? 0} linked</span>
                </div>
                <div className="mt-3 grid gap-2">
                  {paymentReceiptsQuery.isLoading ? (
                    <div className="animate-pulse space-y-2"><div className="h-4 w-40 rounded bg-surface-secondary" /><div className="h-4 w-28 rounded bg-surface-secondary" /></div>
                  ) : paymentReceiptsQuery.isError ? (
                    <p className="text-sm text-amber-700">Payment receipts could not be loaded.</p>
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
                        resendLabel="Resend receipt"
                        canWrite={canWrite && Boolean(csrfToken)}
                        onResend={() => resendPaymentReceiptEmail({ runtimeBaseURL: apiBaseURL, csrfToken, paymentReceiptID: item.id })}
                      />
                    ))
                  ) : (
                    <p className="text-sm text-text-muted">No payment receipts linked yet.</p>
                  )}
                </div>
              </div>

              <div className="px-5 py-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-xs font-medium text-text-faint">Credit notes</p>
                  <span className="text-xs text-text-muted">{creditNotesQuery.data?.length ?? 0} linked</span>
                </div>
                <div className="mt-3 grid gap-2">
                  {creditNotesQuery.isLoading ? (
                    <div className="animate-pulse space-y-2"><div className="h-4 w-40 rounded bg-surface-secondary" /><div className="h-4 w-28 rounded bg-surface-secondary" /></div>
                  ) : creditNotesQuery.isError ? (
                    <p className="text-sm text-amber-700">Credit notes could not be loaded.</p>
                  ) : creditNotesQuery.data && creditNotesQuery.data.length > 0 ? (
                    creditNotesQuery.data.map((item) => (
                      <BillingDocumentRow
                        key={item.id}
                        title={item.number || item.id}
                        subtitle={[
                          item.credit_status ? `Credit ${formatBillingState(item.credit_status)}` : "",
                          item.refund_status ? `Refund ${formatBillingState(item.refund_status)}` : "",
                        ].filter(Boolean).join(" / ") || "Linked credit note"}
                        meta={[
                          item.total_amount_cents !== undefined ? formatMoney(item.total_amount_cents, item.currency || invoice.currency || "USD") : undefined,
                          formatExactTimestamp(item.issuing_date || item.created_at),
                        ]}
                        fileURL={item.file_url}
                        resendLabel="Resend credit note"
                        canWrite={canWrite && Boolean(csrfToken)}
                        onResend={() => resendCreditNoteEmail({ runtimeBaseURL: apiBaseURL, csrfToken, creditNoteID: item.id })}
                      />
                    ))
                  ) : (
                    <p className="text-sm text-text-muted">No credit notes linked.</p>
                  )}
                </div>
              </div>

              {/* ---- Document actions ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-text-faint mb-3">Document actions</p>
                {resendEmailMutation.isSuccess ? (
                  <div className="mb-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                    Invoice email dispatched.
                  </div>
                ) : null}
                {resendEmailMutation.isError ? (
                  <div className="mb-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    {resendEmailMutation.error instanceof Error ? resendEmailMutation.error.message : "Invoice resend failed."}
                  </div>
                ) : null}
                <div className="flex flex-wrap gap-2">
                  <Link
                    to={`/invoice-explainability?invoice_id=${encodeURIComponent(invoice.invoice_id)}`}
                    className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary"
                  >
                    Open explainability
                  </Link>
                  <Link
                    to={`/payments/${encodeURIComponent(invoice.invoice_id)}`}
                    className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary"
                  >
                    Open payment operations
                  </Link>
                  {invoice.customer_external_id ? (
                    <Link
                      to={`/customers/${encodeURIComponent(invoice.customer_external_id)}`}
                      className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary"
                    >
                      Open customer
                    </Link>
                  ) : null}
                  {actionConfig?.showRecovery && invoice.customer_external_id ? (
                    <Link
                      to={`/replay-operations?customer_id=${encodeURIComponent(invoice.customer_external_id)}&status=failed`}
                      className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary"
                    >
                      Open recovery tools
                    </Link>
                  ) : null}
                </div>
              </div>
            </div>
          </SectionErrorBoundary>
          )
        ) : null}
    </PageContainer>
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
    onError: (err: Error) => showError(err.message),
  });

  return (
    <div className="flex items-center justify-between gap-3 rounded-md border border-border bg-surface-secondary px-4 py-3">
      <div className="min-w-0">
        <p className="text-sm font-medium text-text-primary">{title}</p>
        <p className="text-xs text-text-muted">{subtitle}</p>
        <div className="flex flex-wrap gap-2 text-xs text-text-faint mt-0.5">
          {meta.filter(Boolean).map((item) => <span key={item}>{item}</span>)}
        </div>
        {resendMutation.isSuccess ? <p className="mt-1 text-xs text-emerald-700">Email dispatch accepted.</p> : null}
        {resendMutation.isError ? <p className="mt-1 text-xs text-amber-700">{resendMutation.error instanceof Error ? resendMutation.error.message : "Dispatch failed."}</p> : null}
      </div>
      <div className="flex shrink-0 gap-2">
        {fileURL ? (
          <a href={fileURL} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 text-xs text-text-secondary transition hover:bg-surface-secondary">
            Open file
          </a>
        ) : null}
        <Button
          variant="secondary"
          onClick={() => resendMutation.mutate()}
          disabled={!canWrite}
          loading={resendMutation.isPending}
        >
          {!resendMutation.isPending ? <Mail className="h-3.5 w-3.5" /> : null}
          {resendLabel}
        </Button>
      </div>
    </div>
  );
}
