"use client";

import Link from "next/link";
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
  runHref,
}: {
  summary?: DunningSummary;
  canWrite?: boolean;
  sendingReminder?: boolean;
  onSendReminder?: () => void;
  runHref?: string;
}) {
  if (!summary) {
    return null;
  }

  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Dunning</p>
      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <OperatorHint title="Workflow posture" body="Use dunning as the recovery control surface after payment failure. The state and next action should tell you whether reminder or retry is appropriate." />
        <OperatorHint title="Reminder rule" body="Use reminder dispatch only when the customer still needs outreach or setup recovery. Do not treat it as a substitute for payment setup readiness." />
      </div>
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
      {runHref ? (
        <Link
          href={runHref}
          className="mt-4 inline-flex h-10 w-full max-w-full items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
        >
          Open dunning run
        </Link>
      ) : null}
      {summary.next_action_type === "collect_payment_reminder" && onSendReminder ? (
        <button
          type="button"
          onClick={onSendReminder}
          disabled={!canWrite || sendingReminder}
          className="mt-4 inline-flex h-10 w-full max-w-full items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {sendingReminder ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
          Send collect-payment reminder
        </button>
      ) : null}
    </section>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}
