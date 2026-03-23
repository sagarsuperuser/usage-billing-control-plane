"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { DunningSummaryPanel } from "@/components/billing/dunning-summary-panel";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPaymentDetail, fetchPaymentEvents, retryPayment, sendCollectPaymentReminder } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { useUISession } from "@/hooks/use-ui-session";

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

type PaymentActionInput = {
  lifecycle: {
    recommended_action: "none" | "monitor_processing" | "retry_payment" | "collect_payment" | "investigate";
    recommended_action_note: string;
  };
};

function paymentActionConfig(payment: PaymentActionInput) {
  switch (payment.lifecycle.recommended_action) {
    case "retry_payment":
      return {
        title: "Retry payment collection",
        body:
          payment.lifecycle.recommended_action_note ||
          "Recent failures look retryable. Retry collection first, then escalate to recovery or explainability only if the state does not move.",
        emphasizeRetry: true,
        showRecovery: false,
        showExplainability: false,
      };
    case "collect_payment":
      return {
        title: "Collect payment before replaying",
        body:
          payment.lifecycle.recommended_action_note ||
          "The main issue is missing or incomplete payment collection. Use customer and invoice flows to collect payment before running deeper recovery.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
    case "investigate":
      return {
        title: "Investigate before retrying",
        body:
          payment.lifecycle.recommended_action_note ||
          "The signal points to a state or projection issue rather than a simple transient failure. Use explainability and replay recovery before retrying collection.",
        emphasizeRetry: false,
        showRecovery: true,
        showExplainability: true,
      };
    case "monitor_processing":
      return {
        title: "Monitor processing state",
        body:
          payment.lifecycle.recommended_action_note ||
          "Payment processing is still in flight. Monitor the event timeline before taking manual recovery action.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
    default:
      return {
        title: "No action required",
        body:
          payment.lifecycle.recommended_action_note ||
          "Payment activity currently looks healthy. Use linked invoice, customer, and event timelines for normal inspection.",
        emphasizeRetry: false,
        showRecovery: false,
        showExplainability: false,
      };
  }
}

export function PaymentDetailScreen({ paymentID }: { paymentID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const [eventLimit, setEventLimit] = useState(25);

  const paymentQuery = useQuery({
    queryKey: ["payment-detail", apiBaseURL, paymentID],
    queryFn: () => fetchPaymentDetail({ runtimeBaseURL: apiBaseURL, paymentID }),
    enabled: isAuthenticated && scope === "tenant" && paymentID.trim().length > 0,
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
    enabled: isAuthenticated && scope === "tenant" && paymentID.trim().length > 0,
  });

  const retryMutation = useMutation({
    mutationFn: () => retryPayment({ runtimeBaseURL: apiBaseURL, csrfToken, paymentID }),
    onSuccess: async () => {
      await Promise.all([paymentQuery.refetch(), eventsQuery.refetch()]);
    },
  });
  const reminderMutation = useMutation({
    mutationFn: (runID: string) => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await Promise.all([paymentQuery.refetch(), eventsQuery.refetch()]);
    },
  });

  const payment = paymentQuery.data;
  const actionConfig = payment ? paymentActionConfig(payment) : null;
  const dunningRunID = payment?.dunning?.run_id;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Tenant" },
            { href: "/payments", label: "Payments" },
            { label: payment?.invoice_number || paymentID },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Payments are tenant-scoped. Sign in with a tenant account to inspect payment state, lifecycle signals, and retry actions."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {paymentQuery.isLoading ? (
          <LoadingPanel label="Loading payment detail" />
        ) : paymentQuery.isError || !payment ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Payment not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested payment detail could not be loaded from the tenant APIs.</p>
            <Link
              href="/payments"
              className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to payments
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{payment.invoice_number || payment.invoice_id}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{payment.invoice_id}</span>
                    <Badge>{formatState(payment.payment_status)}</Badge>
                    <Badge>{formatState(payment.invoice_status)}</Badge>
                    {payment.payment_overdue ? <Badge>Overdue</Badge> : null}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <Link
                    href="/payments"
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Back to payments
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
              <MetricCard label="Amount due" value={formatMoney(payment.total_due_amount_cents, payment.currency || "USD")} />
              <MetricCard label="Amount paid" value={formatMoney(payment.total_paid_amount_cents, payment.currency || "USD")} />
              <MetricCard label="Failure signals" value={String(payment.lifecycle.failure_event_count)} />
              <MetricCard label="Overdue signals" value={String(payment.lifecycle.overdue_signal_count)} />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
              <div className="min-w-0 grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Lifecycle summary</p>
                  <div className="mt-5 grid gap-3 lg:grid-cols-2">
                    <StatusCard label="Recommended action" value={formatState(payment.lifecycle.recommended_action)} />
                    <StatusCard label="Requires action" value={payment.lifecycle.requires_action ? "Yes" : "No"} />
                    <StatusCard label="Last event" value={formatState(payment.last_event_type)} />
                    <StatusCard label="Last event at" value={formatExactTimestamp(payment.last_event_at)} />
                  </div>
                  <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-700">
                    <p className="font-semibold text-slate-950">Operator guidance</p>
                    <p className="mt-2">{payment.lifecycle.recommended_action_note || "No specific action is currently recommended."}</p>
                  </div>
                  {payment.last_payment_error ? (
                    <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                      <p className="font-semibold text-amber-900">Last payment error</p>
                      <p className="mt-2">{payment.last_payment_error}</p>
                    </div>
                  ) : null}
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Recent payment events</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">Timeline</h2>
                    </div>
                    <select
                      value={String(eventLimit)}
                      onChange={(event) => setEventLimit(Number(event.target.value))}
                      className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      <option value="10">10</option>
                      <option value="25">25</option>
                      <option value="50">50</option>
                    </select>
                  </div>

                  <div className="mt-4 grid gap-3">
                    {eventsQuery.isLoading ? (
                      <LoadingPanel label="Loading payment events" compact />
                    ) : (eventsQuery.data?.items || []).length === 0 ? (
                      <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
                        <p className="font-semibold text-slate-950">No payment events found yet.</p>
                        <p className="mt-2">The payment timeline will populate as invoice payment webhooks are received and projected.</p>
                      </div>
                    ) : (
                      (eventsQuery.data?.items || []).map((event) => (
                        <article key={event.id} className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <p className="text-sm font-semibold text-slate-950">{event.webhook_type}</p>
                              <p className="mt-1 text-xs text-slate-500">Occurred {formatExactTimestamp(event.occurred_at)}</p>
                              <p className="text-[11px] text-slate-400">Received {formatExactTimestamp(event.received_at)}</p>
                            </div>
                            <Badge>{formatState(event.payment_status)}</Badge>
                          </div>
                        </article>
                      ))
                    )}
                  </div>
                </section>
              </div>

              <aside className="min-w-0 grid gap-5 self-start">
                <DunningSummaryPanel
                  summary={payment.dunning}
                  canWrite={canWrite && Boolean(csrfToken)}
                  sendingReminder={reminderMutation.isPending}
                  onSendReminder={dunningRunID ? () => reminderMutation.mutate(dunningRunID) : undefined}
                  runHref={dunningRunID ? `/dunning/${encodeURIComponent(dunningRunID)}` : undefined}
                />
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Retry and recovery</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Recommended action" value={formatState(payment.lifecycle.recommended_action)} />
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
                    {payment.customer_external_id && actionConfig?.showRecovery ? (
                      <Link
                        href={`/replay-operations?customer_id=${encodeURIComponent(payment.customer_external_id)}&status=failed`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                      >
                        Open recovery tools
                      </Link>
                    ) : null}
                    {actionConfig?.showExplainability ? (
                      <Link
                        href={`/invoice-explainability?invoice_id=${encodeURIComponent(payment.invoice_id)}`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                      >
                        Open explainability
                      </Link>
                    ) : null}
                    {payment.customer_external_id && payment.lifecycle.recommended_action === "collect_payment" ? (
                      <Link
                        href={`/customers/${encodeURIComponent(payment.customer_external_id)}#payment-collection`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                      >
                        Open customer payment setup
                      </Link>
                    ) : null}
                    {!actionConfig?.emphasizeRetry &&
                    payment.lifecycle.recommended_action !== "collect_payment" &&
                    payment.customer_external_id ? (
                      <Link
                        href={`/customers/${encodeURIComponent(payment.customer_external_id)}`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                      >
                        Open customer payment context
                      </Link>
                    ) : null}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={payment.customer_display_name || "-"} />
                    <MetaItem label="Customer external ID" value={payment.customer_external_id || "-"} mono />
                    {payment.customer_external_id ? (
                      <Link
                        href={`/customers/${encodeURIComponent(payment.customer_external_id)}`}
                        className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
                      >
                        Open customer
                      </Link>
                    ) : null}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Linked invoice</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Invoice" value={payment.invoice_number || payment.invoice_id} />
                    <MetaItem label="Organization" value={payment.organization_id || "-"} />
                    <MetaItem label="Updated" value={formatExactTimestamp(payment.updated_at)} />
                    <Link
                      href={`/invoices/${encodeURIComponent(payment.invoice_id)}`}
                      className="inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                    >
                      Open invoice
                    </Link>
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

function LoadingPanel({ label, compact }: { label: string; compact?: boolean }) {
  return (
    <section className={`rounded-2xl border border-slate-200 bg-white text-sm text-slate-600 shadow-sm ${compact ? "p-4" : "p-6"}`}>
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
