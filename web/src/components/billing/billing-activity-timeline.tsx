
import { Link } from "@tanstack/react-router";

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
    <section className="rounded-2xl border border-border bg-surface p-6 shadow-sm">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-text-muted">Billing timeline</p>
          <h2 className="mt-2 text-xl font-semibold text-text-primary">Correlated events</h2>
          <p className="mt-2 text-sm text-text-muted">
            Follow webhook projection, dunning state changes, and reminder delivery in one operator sequence.
          </p>
          <div className="mt-4 grid gap-3 lg:grid-cols-3">
            <OperatorHint title="Read order" body="Start with the newest operational signal, then compare the source and summary before drilling into detail." />
            <OperatorHint title="Source split" body="Webhook rows explain payment projection. Dunning and reminder rows explain recovery workflow and outreach." />
            <OperatorHint title="Escalation rule" body="Escalate only after the timeline disagrees with the diagnosis or the underlying lifecycle record." />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <span className="rounded-full border border-border bg-surface-secondary px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-text-secondary">
            {entries.length} events
          </span>
          {dunningRunHref ? (
            <Link
              to={dunningRunHref}
              className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm font-medium text-text-secondary transition hover:bg-surface-tertiary"
            >
              Open dunning run
            </Link>
          ) : null}
        </div>
      </div>

      {loading ? (
        <div className="mt-5 animate-pulse space-y-3 rounded-xl border border-border bg-surface-secondary px-4 py-4">
          <div className="h-4 w-full rounded bg-surface-secondary" />
          <div className="h-4 w-3/4 rounded bg-surface-secondary" />
          <div className="h-4 w-1/2 rounded bg-surface-secondary" />
        </div>
      ) : null}

      {!loading && error ? (
        <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">{error}</div>
      ) : null}

      {!loading && !error ? (
        <div className="mt-5 grid gap-3">
          {entries.length > 0 ? (
            entries.map((entry) => (
              <article key={entry.id} className="rounded-xl border border-border bg-surface-secondary px-4 py-4">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-semibold text-text-primary">{entry.title}</p>
                      <span className="rounded-full border border-border bg-surface px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-text-secondary">
                        {entry.badge}
                      </span>
                    </div>
                    <p className="mt-1 text-xs uppercase tracking-[0.14em] text-text-muted">{entry.source}</p>
                    {entry.summary ? <p className="mt-2 text-sm text-text-secondary">{entry.summary}</p> : null}
                    {entry.detail ? <p className="mt-2 break-all font-mono text-xs text-text-muted">{entry.detail}</p> : null}
                  </div>
                  <div className="text-left text-xs text-text-muted lg:text-right">
                    <p>{formatExactTimestamp(entry.timestamp)}</p>
                    {entry.secondaryTimeLabel ? <p className="mt-1">{entry.secondaryTimeLabel}</p> : null}
                  </div>
                </div>
              </article>
            ))
          ) : (
            <div className="rounded-xl border border-dashed border-slate-300 bg-surface-secondary px-5 py-8 text-sm text-text-muted">
              <p className="font-semibold text-text-primary">No correlated billing events yet.</p>
              <p className="mt-2">Webhook projections, dunning state changes, and reminders will appear here once activity is recorded.</p>
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-border bg-surface-secondary px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-text-muted">{title}</p>
      <p className="mt-2 text-sm leading-6 text-text-secondary">{body}</p>
    </div>
  );
}
