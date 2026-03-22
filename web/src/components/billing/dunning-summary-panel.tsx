"use client";

import { LoaderCircle } from "lucide-react";

import { formatExactTimestamp } from "@/lib/format";
import type { DunningSummary } from "@/lib/types";

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

function MetaRow({ label, value, mono = false }: { label: string; value?: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <span className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">{label}</span>
      <span className={`text-right text-sm text-slate-900 ${mono ? "font-mono text-xs" : ""}`}>{value || "-"}</span>
    </div>
  );
}

export function DunningSummaryPanel({
  summary,
  canWrite,
  sendingReminder,
  onSendReminder,
}: {
  summary?: DunningSummary;
  canWrite?: boolean;
  sendingReminder?: boolean;
  onSendReminder?: () => void;
}) {
  if (!summary) {
    return null;
  }

  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Dunning</p>
      <div className="mt-4 grid gap-3">
        <MetaRow label="State" value={formatState(summary.state)} />
        <MetaRow label="Next action" value={formatState(summary.next_action_type)} />
        <MetaRow label="Next action at" value={formatExactTimestamp(summary.next_action_at)} />
        <MetaRow label="Last event" value={formatState(summary.last_event_type)} />
        <MetaRow label="Last notification" value={formatState(summary.last_notification_status)} />
        <MetaRow label="Attempts" value={String(summary.attempt_count)} />
        <MetaRow label="Run id" value={summary.run_id} mono />
      </div>
      {summary.last_notification_error ? (
        <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
          <p className="font-semibold text-amber-900">Last notification error</p>
          <p className="mt-2">{summary.last_notification_error}</p>
        </div>
      ) : null}
      {summary.next_action_type === "collect_payment_reminder" && onSendReminder ? (
        <button
          type="button"
          onClick={onSendReminder}
          disabled={!canWrite || sendingReminder}
          className="mt-4 inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {sendingReminder ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
          Send collect-payment reminder
        </button>
      ) : null}
    </section>
  );
}
