"use client";

import Link from "next/link";

import type { DunningRunDetail, LagoWebhookEvent } from "@/lib/types";
import { formatExactTimestamp } from "@/lib/format";

type BillingTimelineEntry = {
  id: string;
  timestamp: string;
  title: string;
  badge: string;
  source: string;
  summary?: string;
  detail?: string;
  secondaryTimeLabel?: string;
};

function normalizeLabel(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

function buildTimelineEntries(params: {
  webhookEvents?: LagoWebhookEvent[];
  dunningDetail?: DunningRunDetail;
}): BillingTimelineEntry[] {
  const webhookEntries = (params.webhookEvents || []).map((event) => ({
    id: `webhook-${event.id}`,
    timestamp: event.occurred_at || event.received_at,
    title: normalizeLabel(event.webhook_type),
    badge: "Webhook",
    source: "payment status projection",
    summary: [
      event.payment_status ? `Payment ${normalizeLabel(event.payment_status)}` : "",
      event.invoice_status ? `Invoice ${normalizeLabel(event.invoice_status)}` : "",
      event.last_payment_error ? `Error ${event.last_payment_error}` : "",
    ]
      .filter(Boolean)
      .join(" • "),
    detail: event.webhook_key,
    secondaryTimeLabel: event.received_at ? `Received ${formatExactTimestamp(event.received_at)}` : undefined,
  }));

  const dunningEntries = (params.dunningDetail?.events || []).map((event) => ({
    id: `dunning-${event.id}`,
    timestamp: event.created_at,
    title: normalizeLabel(event.event_type),
    badge: "Dunning",
    source: "collection workflow",
    summary: [
      event.state ? `State ${normalizeLabel(event.state)}` : "",
      event.reason ? `Reason ${normalizeLabel(event.reason)}` : "",
      event.action_type ? `Action ${normalizeLabel(event.action_type)}` : "",
    ]
      .filter(Boolean)
      .join(" • "),
    detail: event.attempt_count > 0 ? `Attempt ${event.attempt_count}` : undefined,
  }));

  const notificationEntries = (params.dunningDetail?.notification_intents || []).map((intent) => ({
    id: `intent-${intent.id}`,
    timestamp: intent.dispatched_at || intent.created_at,
    title: normalizeLabel(intent.intent_type),
    badge: "Reminder",
    source: "notification dispatch",
    summary: [
      intent.status ? `Status ${normalizeLabel(intent.status)}` : "",
      intent.recipient_email ? `Recipient ${intent.recipient_email}` : "",
      intent.last_error ? `Error ${intent.last_error}` : "",
    ]
      .filter(Boolean)
      .join(" • "),
    detail: intent.delivery_backend ? `Backend ${intent.delivery_backend}` : undefined,
    secondaryTimeLabel: intent.dispatched_at ? `Created ${formatExactTimestamp(intent.created_at)}` : undefined,
  }));

  return [...webhookEntries, ...dunningEntries, ...notificationEntries].sort((left, right) => {
    return new Date(right.timestamp).getTime() - new Date(left.timestamp).getTime();
  });
}

export function BillingActivityTimeline({
  webhookEvents,
  dunningDetail,
  dunningRunHref,
  loading,
  error,
}: {
  webhookEvents?: LagoWebhookEvent[];
  dunningDetail?: DunningRunDetail;
  dunningRunHref?: string;
  loading?: boolean;
  error?: string;
}) {
  const entries = buildTimelineEntries({ webhookEvents, dunningDetail });

  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing timeline</p>
          <h2 className="mt-2 text-xl font-semibold text-slate-950">Correlated events</h2>
          <p className="mt-2 text-sm text-slate-600">
            Follow webhook projection, dunning state changes, and reminder delivery in one operator sequence.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <span className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
            {entries.length} events
          </span>
          {dunningRunHref ? (
            <Link
              href={dunningRunHref}
              className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
            >
              Open dunning run
            </Link>
          ) : null}
        </div>
      </div>

      {loading ? (
        <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-600">
          Loading timeline events.
        </div>
      ) : null}

      {!loading && error ? (
        <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">{error}</div>
      ) : null}

      {!loading && !error ? (
        <div className="mt-5 grid gap-3">
          {entries.length > 0 ? (
            entries.map((entry) => (
              <article key={entry.id} className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-semibold text-slate-950">{entry.title}</p>
                      <span className="rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
                        {entry.badge}
                      </span>
                    </div>
                    <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-500">{entry.source}</p>
                    {entry.summary ? <p className="mt-2 text-sm text-slate-700">{entry.summary}</p> : null}
                    {entry.detail ? <p className="mt-2 break-all font-mono text-xs text-slate-500">{entry.detail}</p> : null}
                  </div>
                  <div className="text-left text-xs text-slate-500 lg:text-right">
                    <p>{formatExactTimestamp(entry.timestamp)}</p>
                    {entry.secondaryTimeLabel ? <p className="mt-1">{entry.secondaryTimeLabel}</p> : null}
                  </div>
                </div>
              </article>
            ))
          ) : (
            <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
              <p className="font-semibold text-slate-950">No correlated billing events yet.</p>
              <p className="mt-2">Webhook projections, dunning state changes, and reminders will appear here once activity is recorded.</p>
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}
