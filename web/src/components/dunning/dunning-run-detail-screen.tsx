"use client";

import Link from "next/link";
import { useMemo } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchDunningRunDetail, pauseDunningRun, resolveDunningRun, resumeDunningRun, retryDunningRunNow, sendCollectPaymentReminder } from "@/lib/api";
import { diagnoseDunningRun, dunningDiagnosisToneClass } from "@/lib/dunning-diagnosis";
import { formatExactTimestamp } from "@/lib/format";

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <p className="text-xs uppercase tracking-[0.16em] text-slate-500">{label}</p>
      <p className="mt-2 text-sm font-medium text-slate-900">{value}</p>
    </div>
  );
}

export function DunningRunDetailScreen({ runID }: { runID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const detailQuery = useQuery({
    queryKey: ["dunning-run", apiBaseURL, runID],
    queryFn: () => fetchDunningRunDetail({ runtimeBaseURL: apiBaseURL, runID }),
    enabled: isTenantSession,
  });

  const reminderMutation = useMutation({
    mutationFn: () => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const retryMutation = useMutation({
    mutationFn: () => retryDunningRunNow({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const pauseMutation = useMutation({
    mutationFn: () => pauseDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const resumeMutation = useMutation({
    mutationFn: () => resumeDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const resolveMutation = useMutation({
    mutationFn: () => resolveDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const detail = detailQuery.data;
  const run = detail?.run;
  const diagnosis = useMemo(() => (run ? diagnoseDunningRun(run) : null), [run]);
  const latestIntent = useMemo(() => {
    const items = detail?.notification_intents ?? [];
    return items.length > 0 ? items[0] : undefined;
  }, [detail?.notification_intents]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { href: "/dunning", label: "Dunning" },
            { label: runID },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Dunning run detail is tenant-scoped. Sign in with a tenant account to inspect collection workflow history."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-emerald-700">Dunning Run Detail</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900 md:text-4xl">Invoice-level collections workflow</h1>
              <p className="mt-2 max-w-3xl text-sm text-slate-600 md:text-base">
                Inspect the active run, event history, and notification intents before retrying payment collection or escalating into deeper recovery work.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => detailQuery.refetch()}
                disabled={detailQuery.isFetching || !isTenantSession}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-stone-300 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {detailQuery.isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                Refresh
              </button>
              {run?.next_action_type === "collect_payment_reminder" ? (
                <button
                  type="button"
                  onClick={() => reminderMutation.mutate()}
                  disabled={!canWrite || !csrfToken || reminderMutation.isPending}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {reminderMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Send collect-payment reminder
                </button>
              ) : null}
              {run?.next_action_type === "retry_payment" && !run?.paused && !run?.resolved_at ? (
                <button
                  type="button"
                  onClick={() => retryMutation.mutate()}
                  disabled={!canWrite || !csrfToken || retryMutation.isPending}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-indigo-200 bg-indigo-50 px-4 text-sm font-medium text-indigo-700 transition hover:bg-indigo-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Retry payment now
                </button>
              ) : null}
              {!run?.resolved_at && !run?.paused ? (
                <button
                  type="button"
                  onClick={() => pauseMutation.mutate()}
                  disabled={!canWrite || !csrfToken || pauseMutation.isPending}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 text-sm font-medium text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {pauseMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Pause run
                </button>
              ) : null}
              {run?.paused && !run?.resolved_at ? (
                <button
                  type="button"
                  onClick={() => resumeMutation.mutate()}
                  disabled={!canWrite || !csrfToken || resumeMutation.isPending}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-4 text-sm font-medium text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {resumeMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Resume run
                </button>
              ) : null}
              {!run?.resolved_at ? (
                <button
                  type="button"
                  onClick={() => resolveMutation.mutate()}
                  disabled={!canWrite || !csrfToken || resolveMutation.isPending}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 text-sm font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {resolveMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Resolve run
                </button>
              ) : null}
            </div>
          </div>
        </section>

        {detailQuery.error ? (
          <section className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-4 text-sm text-rose-700">
            {(detailQuery.error as Error).message}
          </section>
        ) : null}

        {detailQuery.isLoading ? (
          <section className="rounded-2xl border border-stone-200 bg-white px-4 py-10 text-center text-sm text-slate-500 shadow-sm">
            Loading dunning run detail
          </section>
        ) : null}

        {run ? (
          <>
            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-6">
              <StatCard label="Invoice" value={run.invoice_id} />
              <StatCard label="Customer" value={run.customer_external_id || "-"} />
              <StatCard label="State" value={formatState(run.state)} />
              <StatCard label="Next action" value={formatState(run.next_action_type)} />
              <StatCard label="Next action at" value={formatExactTimestamp(run.next_action_at)} />
              <StatCard label="Attempts" value={String(run.attempt_count)} />
            </section>

            {diagnosis ? (
              <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
                <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Current blocker</p>
                    <h2 className="mt-1 text-lg font-semibold text-slate-900">{diagnosis.title}</h2>
                    <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">{diagnosis.summary}</p>
                  </div>
                  <span className={`inline-flex rounded-full border px-3 py-1.5 text-xs font-semibold uppercase tracking-[0.14em] ${dunningDiagnosisToneClass(diagnosis.tone)}`}>
                    {diagnosis.title}
                  </span>
                </div>

                <div className="mt-5 grid gap-4 md:grid-cols-2">
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Next step</p>
                    <p className="mt-2 text-sm leading-6 text-slate-700">{diagnosis.nextStep}</p>
                  </div>
                  <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Workflow state</p>
                    <p className="mt-2 text-sm leading-6 text-slate-700">
                      {run.paused
                        ? "Resume or resolve this run before expecting automation to continue."
                        : run.resolved_at
                          ? "No immediate collection action is required unless you are auditing the resolution."
                          : run.next_action_type === "collect_payment_reminder"
                            ? "Use reminder dispatch only when the customer still needs a payment setup or nudge path."
                            : run.next_action_type === "retry_payment"
                              ? "Check invoice and payment detail before forcing retry, especially after repeated failures."
                              : "Monitor this workflow and open linked billing surfaces if state progression stalls."}
                    </p>
                  </div>
                </div>
              </section>
            ) : null}

            <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
              <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
                <div>
                  <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Event timeline</p>
                  <h2 className="mt-1 text-lg font-semibold text-slate-900">Run state transitions</h2>
                </div>
                <div className="mt-5 overflow-hidden rounded-2xl border border-stone-200">
                  <table className="min-w-full divide-y divide-stone-200 text-left">
                    <thead className="bg-stone-50 text-xs uppercase tracking-[0.14em] text-slate-500">
                      <tr>
                        <th className="px-4 py-3 font-medium">When</th>
                        <th className="px-4 py-3 font-medium">Event</th>
                        <th className="px-4 py-3 font-medium">State</th>
                        <th className="px-4 py-3 font-medium">Reason</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-stone-200 bg-white">
                      {detail?.events.length ? (
                        detail.events.map((event) => (
                          <tr key={event.id} className="text-sm text-slate-700">
                            <td className="px-4 py-3">{formatExactTimestamp(event.created_at)}</td>
                            <td className="px-4 py-3 capitalize">{formatState(event.event_type)}</td>
                            <td className="px-4 py-3 capitalize">{formatState(event.state)}</td>
                            <td className="px-4 py-3">{formatState(event.reason)}</td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={4} className="px-4 py-8 text-center text-sm text-slate-500">
                            No dunning events recorded yet.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </section>

              <aside className="grid gap-6 self-start">
                <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
                  <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Notification intents</p>
                  <h2 className="mt-1 text-lg font-semibold text-slate-900">Dispatch status</h2>
                  <div className="mt-4 grid gap-3">
                    {detail?.notification_intents.length ? (
                      detail.notification_intents.map((intent) => (
                        <div key={intent.id} className="rounded-xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-slate-700">
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <p className="font-semibold text-slate-900">{formatState(intent.intent_type)}</p>
                              <p className="mt-1 text-xs uppercase tracking-[0.14em] text-slate-500">{formatState(intent.status)}</p>
                            </div>
                            <span className="font-mono text-[11px] text-slate-500">{intent.id}</span>
                          </div>
                          <dl className="mt-3 grid gap-2 text-xs text-slate-500">
                            <div className="flex items-start justify-between gap-3">
                              <dt>Recipient</dt>
                              <dd className="text-right text-slate-700">{intent.recipient_email || "-"}</dd>
                            </div>
                            <div className="flex items-start justify-between gap-3">
                              <dt>Backend</dt>
                              <dd className="text-right text-slate-700">{intent.delivery_backend || "-"}</dd>
                            </div>
                            <div className="flex items-start justify-between gap-3">
                              <dt>Created</dt>
                              <dd className="text-right text-slate-700">{formatExactTimestamp(intent.created_at)}</dd>
                            </div>
                            <div className="flex items-start justify-between gap-3">
                              <dt>Dispatched</dt>
                              <dd className="text-right text-slate-700">{formatExactTimestamp(intent.dispatched_at)}</dd>
                            </div>
                          </dl>
                          {intent.last_error ? <p className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">{intent.last_error}</p> : null}
                        </div>
                      ))
                    ) : (
                      <div className="rounded-xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-slate-500">No notification intents recorded yet.</div>
                    )}
                  </div>
                </section>

                <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
                  <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Linked surfaces</p>
                  <div className="mt-4 grid gap-3">
                    <Link href={`/payments/${encodeURIComponent(run.invoice_id)}`} className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100">
                      Open payment detail
                    </Link>
                    <Link href={`/invoices/${encodeURIComponent(run.invoice_id)}`} className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm font-medium text-slate-700 transition hover:bg-slate-100">
                      Open invoice detail
                    </Link>
                    {latestIntent?.recipient_email ? <p className="text-xs text-slate-500">Latest reminder target: {latestIntent.recipient_email}</p> : null}
                  </div>
                </section>
              </aside>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}
