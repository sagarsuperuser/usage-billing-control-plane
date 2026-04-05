
import { Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchDunningRunDetail, pauseDunningRun, resolveDunningRun, resumeDunningRun, retryDunningRunNow, sendCollectPaymentReminder } from "@/lib/api";
import { diagnoseDunningRun, dunningDiagnosisToneClass } from "@/lib/dunning-diagnosis";
import { formatExactTimestamp } from "@/lib/format";
import { showError } from "@/lib/toast";

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

export function DunningRunDetailScreen({ runID }: { runID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [actionsOpen, setActionsOpen] = useState(false);

  const detailQuery = useQuery({
    queryKey: ["dunning-run", apiBaseURL, runID],
    queryFn: () => fetchDunningRunDetail({ runtimeBaseURL: apiBaseURL, runID }),
    enabled: isTenantSession,
  });

  const reminderMutation = useMutation({
    mutationFn: () => sendCollectPaymentReminder({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const retryMutation = useMutation({
    mutationFn: () => retryDunningRunNow({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const pauseMutation = useMutation({
    mutationFn: () => pauseDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const resumeMutation = useMutation({
    mutationFn: () => resumeDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const resolveMutation = useMutation({
    mutationFn: () => resolveDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const detail = detailQuery.data;
  const run = detail?.run;
  const diagnosis = useMemo(() => (run ? diagnoseDunningRun(run) : null), [run]);
  const _anyPending = reminderMutation.isPending || retryMutation.isPending || pauseMutation.isPending || resumeMutation.isPending || resolveMutation.isPending;

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { href: "/dunning", label: "Dunning" },
            { label: runID },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {detailQuery.error ? (
          <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {(detailQuery.error as Error).message}
          </div>
        ) : null}

        {detailQuery.isLoading ? (
          <div className="rounded-lg border border-stone-200 bg-white px-4 py-10 text-center text-sm text-slate-500 shadow-sm">
            Loading dunning run detail...
          </div>
        ) : null}

        {isTenantSession && run ? (
          <>
            {/* Header card */}
            <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
              <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
                <div className="flex items-center gap-3">
                  <h1 className="text-sm font-semibold text-slate-900">Dunning run</h1>
                  <Link to={`/invoices/${encodeURIComponent(run.invoice_id)}`} className="font-mono text-sm text-slate-600 hover:text-slate-900">
                    {run.invoice_id}
                  </Link>
                  {diagnosis ? (
                    <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${dunningDiagnosisToneClass(diagnosis.tone)}`}>
                      {diagnosis.title}
                    </span>
                  ) : null}
                </div>
                <div className="relative">
                  <button
                    type="button"
                    onClick={() => setActionsOpen(!actionsOpen)}
                    className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800"
                  >
                    Actions {actionsOpen ? "▴" : "▾"}
                  </button>
                  {actionsOpen ? (
                    <div className="absolute right-0 top-full z-10 mt-1 w-56 rounded-lg border border-stone-200 bg-white py-1 shadow-lg">
                      {run.next_action_type === "collect_payment_reminder" ? (
                        <ActionMenuItem label="Send reminder" pending={reminderMutation.isPending} disabled={!canWrite || !csrfToken} onClick={() => { reminderMutation.mutate(); setActionsOpen(false); }} />
                      ) : null}
                      {run.next_action_type === "retry_payment" && !run.paused && !run.resolved_at ? (
                        <ActionMenuItem label="Retry payment" pending={retryMutation.isPending} disabled={!canWrite || !csrfToken} onClick={() => { retryMutation.mutate(); setActionsOpen(false); }} />
                      ) : null}
                      {!run.resolved_at && !run.paused ? (
                        <ActionMenuItem label="Pause run" pending={pauseMutation.isPending} disabled={!canWrite || !csrfToken} onClick={() => { pauseMutation.mutate(); setActionsOpen(false); }} />
                      ) : null}
                      {run.paused && !run.resolved_at ? (
                        <ActionMenuItem label="Resume run" pending={resumeMutation.isPending} disabled={!canWrite || !csrfToken} onClick={() => { resumeMutation.mutate(); setActionsOpen(false); }} />
                      ) : null}
                      {!run.resolved_at ? (
                        <ActionMenuItem label="Resolve run" pending={resolveMutation.isPending} disabled={!canWrite || !csrfToken} onClick={() => { resolveMutation.mutate(); setActionsOpen(false); }} />
                      ) : null}
                    </div>
                  ) : null}
                </div>
              </div>
              <div className="border-b border-stone-200 px-5 py-2 text-xs text-slate-500">
                {run.customer_external_id || "-"} · {run.attempt_count} attempt{run.attempt_count === 1 ? "" : "s"} · Next: {formatState(run.next_action_type)} {run.next_action_at ? `at ${formatExactTimestamp(run.next_action_at)}` : ""}
              </div>

              {/* Details section */}
              <div className="border-b border-stone-200 px-5 py-4">
                <h2 className="text-sm font-semibold text-slate-900">Details</h2>
                <dl className="mt-3 grid grid-cols-2 gap-x-8 gap-y-2 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Invoice</dt>
                    <dd className="font-mono text-slate-700">{run.invoice_id}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">State</dt>
                    <dd className="capitalize text-slate-700">{formatState(run.state)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Customer</dt>
                    <dd className="text-slate-700">{run.customer_external_id || "-"}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Attempts</dt>
                    <dd className="text-slate-700">{run.attempt_count}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Created</dt>
                    <dd className="text-slate-700">{formatExactTimestamp(run.created_at)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Next at</dt>
                    <dd className="text-slate-700">{formatExactTimestamp(run.next_action_at)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Next action</dt>
                    <dd className="capitalize text-slate-700">{formatState(run.next_action_type)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-slate-500">Paused</dt>
                    <dd className="text-slate-700">{run.paused ? "Yes" : "No"}</dd>
                  </div>
                </dl>
              </div>

              {/* Events section */}
              <div className="px-5 py-4">
                <h2 className="text-sm font-semibold text-slate-900">Events</h2>
                <table className="mt-3 w-full text-sm">
                  <thead>
                    <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                      <th className="py-2 pr-4 font-semibold">When</th>
                      <th className="py-2 pr-4 font-semibold">Event</th>
                      <th className="py-2 pr-4 font-semibold">State</th>
                      <th className="py-2 font-semibold">Reason</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-100">
                    {detail?.events.length ? (
                      detail.events.map((event) => (
                        <tr key={event.id} className="text-slate-600">
                          <td className="py-2 pr-4">{formatExactTimestamp(event.created_at)}</td>
                          <td className="py-2 pr-4 capitalize">{formatState(event.event_type)}</td>
                          <td className="py-2 pr-4 capitalize">{formatState(event.state)}</td>
                          <td className="py-2">{formatState(event.reason)}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={4} className="py-8 text-center text-slate-500">
                          No dunning events recorded yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        ) : null}
      </main>
    </div>
  );
}

function ActionMenuItem({ label, pending, disabled, onClick }: { label: string; pending: boolean; disabled: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      disabled={disabled || pending}
      onClick={onClick}
      className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-slate-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
      {label}
    </button>
  );
}
