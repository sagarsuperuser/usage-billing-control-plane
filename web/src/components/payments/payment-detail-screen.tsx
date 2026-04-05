
import { Link } from "@tanstack/react-router";
import { LoaderCircle, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { BillingActivityTimeline } from "@/components/billing/billing-activity-timeline";
import { BillingFailureDiagnosisCard } from "@/components/billing/billing-failure-diagnosis";
import { BillingFailureEvidence } from "@/components/billing/billing-failure-evidence";
import { DunningSummaryPanel } from "@/components/billing/dunning-summary-panel";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchDunningRunDetail, fetchPaymentDetail, fetchPaymentEvents, retryPayment, sendCollectPaymentReminder } from "@/lib/api";
import { billingActionConfig, billingFailureDiagnosis, billingFailureEvidence, formatBillingState } from "@/lib/billing-lifecycle";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

export function PaymentDetailScreen({ paymentID }: { paymentID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const [eventLimit, setEventLimit] = useState(25);
  const isTenantSession = isAuthenticated && scope === "tenant";

  const paymentQuery = useQuery({
    queryKey: ["payment-detail", apiBaseURL, paymentID],
    queryFn: () => fetchPaymentDetail({ runtimeBaseURL: apiBaseURL, paymentID }),
    enabled: isTenantSession && paymentID.trim().length > 0,
  });

  const eventsQuery = useQuery({
    queryKey: ["payment-events", apiBaseURL, paymentID, eventLimit],
    queryFn: () =>
      fetchPaymentEvents({
        runtimeBaseURL: apiBaseURL,
        paymentID,
        sortBy: "received_at",
        order: "desc",
        limit: eventLimit,
        offset: 0,
      }),
    enabled: isTenantSession && paymentID.trim().length > 0,
  });

  const retryMutation = useMutation({
    mutationFn: () => retryPayment({ runtimeBaseURL: apiBaseURL, csrfToken, paymentID }),
    onSuccess: async () => {
      await Promise.all([paymentQuery.refetch(), eventsQuery.refetch()]);
    },
    onError: (err: Error) => showError(err.message),
  });
  const reminderMutation = useMutation({
    mutationFn: (runID: string) => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await Promise.all([paymentQuery.refetch(), eventsQuery.refetch()]);
    },
    onError: (err: Error) => showError(err.message),
  });

  const payment = paymentQuery.data;
  const actionConfig = payment ? billingActionConfig(payment) : null;
  const diagnosis = payment ? billingFailureDiagnosis(payment) : null;
  const diagnosisEvidence = payment ? billingFailureEvidence(payment) : [];
  const dunningRunID = payment?.dunning?.run_id;
  const dunningDetailQuery = useQuery({
    queryKey: ["payment-dunning-run-detail", apiBaseURL, dunningRunID],
    queryFn: () => fetchDunningRunDetail({ runtimeBaseURL: apiBaseURL, runID: dunningRunID as string }),
    enabled: isTenantSession && Boolean(dunningRunID),
  });
  const timelineLoading = eventsQuery.isLoading || (Boolean(dunningRunID) && dunningDetailQuery.isLoading);
  const timelineError =
    eventsQuery.error instanceof Error
      ? eventsQuery.error.message
      : dunningDetailQuery.error instanceof Error
        ? dunningDetailQuery.error.message
        : undefined;

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { href: "/payments", label: "Payments" },
            { label: payment?.invoice_number || paymentID },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? (
          paymentQuery.isLoading ? (
            <section className="rounded-lg border border-stone-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading payment detail
              </div>
            </section>
          ) : paymentQuery.isError || !payment ? (
            <section className="rounded-lg border border-stone-200 bg-white shadow-sm p-5">
              <p className="text-sm font-semibold text-slate-900">Payment not available</p>
              <p className="mt-1 text-sm text-slate-500">The requested payment detail could not be loaded from the workspace APIs.</p>
            </section>
          ) : (
          <SectionErrorBoundary>
            <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
              {/* ---- Header ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900 truncate">{payment.invoice_number || payment.invoice_id}</h1>
                    <span className="shrink-0 rounded-full border border-stone-200 bg-slate-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-slate-700">
                      {formatBillingState(payment.payment_status)}
                    </span>
                    <span className="shrink-0 rounded-full border border-stone-200 bg-slate-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-slate-700">
                      {formatBillingState(payment.invoice_status)}
                    </span>
                    {payment.payment_overdue ? (
                      <span className="shrink-0 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-rose-700">Overdue</span>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {actionConfig?.emphasizeRetry ? (
                      <button
                        type="button"
                        onClick={() => retryMutation.mutate()}
                        disabled={!canWrite || !csrfToken || retryMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {retryMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                        Retry payment
                      </button>
                    ) : null}
                    {payment.customer_external_id && payment.lifecycle.recommended_action === "collect_payment" ? (
                      <Link
                        to={`/customers/${encodeURIComponent(payment.customer_external_id)}#payment-collection`}
                        className="inline-flex h-8 items-center rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800"
                      >
                        Open customer setup
                      </Link>
                    ) : null}
                  </div>
                </div>
                <p className="mt-1.5 text-xs text-slate-500">{payment.invoice_id}</p>
              </div>

              {/* ---- Details ---- */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Amount due</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatMoney(payment.total_due_amount_cents, payment.currency || "USD")}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Amount paid</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatMoney(payment.total_paid_amount_cents, payment.currency || "USD")}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Customer</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{payment.customer_display_name || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Customer ID</dt>
                    <dd className="mt-0.5 text-sm font-mono text-slate-700">{payment.customer_external_id || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last event</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatBillingState(payment.last_event_type)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last event at</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(payment.last_event_at)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Failure signals</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{payment.lifecycle.failure_event_count}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Overdue signals</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{payment.lifecycle.overdue_signal_count}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Updated</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(payment.updated_at)}</dd>
                  </div>
                </dl>
              </div>

              {/* ---- Lifecycle ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-3">Lifecycle</p>
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Action</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatBillingState(payment.lifecycle.recommended_action)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Requires action</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{payment.lifecycle.requires_action ? "Yes" : "No"}</dd>
                  </div>
                </dl>
                <p className="mt-3 text-sm text-slate-600">{payment.lifecycle.recommended_action_note || "No specific action is currently recommended."}</p>
                {payment.last_payment_error ? (
                  <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                    <p className="font-medium">Last payment error</p>
                    <p className="mt-0.5 text-xs">{payment.last_payment_error}</p>
                  </div>
                ) : null}
              </div>

              {/* ---- Links ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-wrap gap-2">
                  {payment.customer_external_id ? (
                    <Link
                      to={`/customers/${encodeURIComponent(payment.customer_external_id)}`}
                      className="inline-flex h-8 items-center rounded-md border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                    >
                      Open customer
                    </Link>
                  ) : null}
                  <Link
                    to={`/invoices/${encodeURIComponent(payment.invoice_id)}`}
                    className="inline-flex h-8 items-center rounded-md border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                  >
                    Open invoice
                  </Link>
                  {actionConfig?.showExplainability ? (
                    <Link
                      to={`/invoice-explainability?invoice_id=${encodeURIComponent(payment.invoice_id)}`}
                      className="inline-flex h-8 items-center rounded-md border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                    >
                      Open explainability
                    </Link>
                  ) : null}
                  {actionConfig?.showRecovery && payment.customer_external_id ? (
                    <Link
                      to={`/replay-operations?customer_id=${encodeURIComponent(payment.customer_external_id)}&status=failed`}
                      className="inline-flex h-8 items-center rounded-md border border-stone-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50"
                    >
                      Open recovery tools
                    </Link>
                  ) : null}
                </div>
              </div>
            </div>

            {/* ---- Dunning ---- */}
            <DunningSummaryPanel
              summary={payment.dunning}
              canWrite={canWrite && Boolean(csrfToken)}
              sendingReminder={reminderMutation.isPending}
              onSendReminder={dunningRunID ? () => reminderMutation.mutate(dunningRunID) : undefined}
              runHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
            />

            {/* ---- Diagnosis ---- */}
            {diagnosis ? <BillingFailureDiagnosisCard diagnosis={diagnosis} /> : null}
            {diagnosisEvidence.length > 0 ? <BillingFailureEvidence items={diagnosisEvidence} /> : null}

            {/* ---- Timeline controls ---- */}
            <div className="flex items-center justify-between gap-3 rounded-lg border border-stone-200 bg-white px-5 py-3 shadow-sm">
              <p className="text-xs font-medium text-slate-400">Event range</p>
              <select
                value={String(eventLimit)}
                onChange={(event) => setEventLimit(Number(event.target.value))}
                className="h-8 rounded-md border border-stone-200 bg-white px-3 text-xs text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="10">10</option>
                <option value="25">25</option>
                <option value="50">50</option>
              </select>
            </div>

            {/* ---- Timeline ---- */}
            <BillingActivityTimeline
              webhookEvents={eventsQuery.data?.items}
              dunningDetail={dunningDetailQuery.data}
              dunningRunHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
              loading={timelineLoading}
              error={timelineError}
            />
          </SectionErrorBoundary>
          )
        ) : null}
      </main>
    </div>
  );
}
