
import { Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Card } from "@/components/ui/card";
import { LoadingSkeleton } from "@/components/ui/loading-skeleton";
import { PageContainer } from "@/components/ui/page-container";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchDunningRunDetail, pauseDunningRun, resolveDunningRun, resumeDunningRun, retryDunningRunNow, sendCollectPaymentReminder } from "@/lib/api";
import { diagnoseDunningRun, dunningDiagnosisToneClass } from "@/lib/dunning-diagnosis";
import { formatExactTimestamp } from "@/lib/format";
import { showError, showSuccess } from "@/lib/toast";

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
    onSuccess: async () => { showSuccess("Reminder sent"); await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const retryMutation = useMutation({
    mutationFn: () => retryDunningRunNow({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { showSuccess("Retry initiated"); await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const pauseMutation = useMutation({
    mutationFn: () => pauseDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { showSuccess("Run paused"); await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const resumeMutation = useMutation({
    mutationFn: () => resumeDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { showSuccess("Run resumed"); await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const resolveMutation = useMutation({
    mutationFn: () => resolveDunningRun({ runtimeBaseURL: apiBaseURL, csrfToken, runID }),
    onSuccess: async () => { showSuccess("Run resolved"); await detailQuery.refetch(); },
    onError: (err: Error) => showError(err.message),
  });

  const detail = detailQuery.data;
  const run = detail?.run;
  const diagnosis = useMemo(() => (run ? diagnoseDunningRun(run) : null), [run]);
  const _anyPending = reminderMutation.isPending || retryMutation.isPending || pauseMutation.isPending || resumeMutation.isPending || resolveMutation.isPending;

  return (
    <PageContainer>
        <AppBreadcrumbs
          items={[
            { href: "/dunning", label: "Dunning" },
            { label: runID },
          ]}
        />


        {detailQuery.error ? (
          <Alert tone="danger">
            {(detailQuery.error as Error).message}
          </Alert>
        ) : null}

        {detailQuery.isLoading ? (
          <LoadingSkeleton variant="card" />
        ) : null}

        {isTenantSession && run ? (
          <>
            {/* Header card */}
            <Card>
              <div className="flex items-center justify-between border-b border-border px-5 py-3">
                <div className="flex items-center gap-3">
                  <h1 className="text-sm font-semibold text-text-primary">Dunning run</h1>
                  <Link to={`/invoices/${encodeURIComponent(run.invoice_id)}`} className="font-mono text-sm text-text-muted hover:text-text-primary">
                    {run.invoice_id}
                  </Link>
                  {diagnosis ? (
                    <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] ${dunningDiagnosisToneClass(diagnosis.tone)}`}>
                      {diagnosis.title}
                    </span>
                  ) : null}
                </div>
                <div className="relative">
                  <Button
                    variant="primary"
                    onClick={() => setActionsOpen(!actionsOpen)}
                  >
                    Actions {actionsOpen ? "▴" : "▾"}
                  </Button>
                  {actionsOpen ? (
                    <div className="absolute right-0 top-full z-10 mt-1 w-56 rounded-lg border border-border bg-surface py-1 shadow-lg">
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
              <div className="border-b border-border px-5 py-2 text-xs text-text-muted">
                {run.customer_external_id || "-"} · {run.attempt_count} attempt{run.attempt_count === 1 ? "" : "s"} · Next: {formatState(run.next_action_type)} {run.next_action_at ? `at ${formatExactTimestamp(run.next_action_at)}` : ""}
              </div>

              {/* Details section */}
              <div className="border-b border-border px-5 py-4">
                <h2 className="text-sm font-semibold text-text-primary">Details</h2>
                <dl className="mt-3 grid grid-cols-2 gap-x-8 gap-y-2 text-sm">
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Invoice</dt>
                    <dd className="font-mono text-text-secondary">{run.invoice_id}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">State</dt>
                    <dd className="capitalize text-text-secondary">{formatState(run.state)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Customer</dt>
                    <dd className="text-text-secondary">{run.customer_external_id || "-"}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Attempts</dt>
                    <dd className="text-text-secondary">{run.attempt_count}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Created</dt>
                    <dd className="text-text-secondary">{formatExactTimestamp(run.created_at)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Next at</dt>
                    <dd className="text-text-secondary">{formatExactTimestamp(run.next_action_at)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Next action</dt>
                    <dd className="capitalize text-text-secondary">{formatState(run.next_action_type)}</dd>
                  </div>
                  <div className="flex justify-between">
                    <dt className="text-text-muted">Paused</dt>
                    <dd className="text-text-secondary">{run.paused ? "Yes" : "No"}</dd>
                  </div>
                </dl>
              </div>

              {/* Events section */}
              <div className="px-5 py-4">
                <h2 className="text-sm font-semibold text-text-primary">Events</h2>
                <table className="mt-3 w-full text-sm">
                  <thead>
                    <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                      <th className="py-2 pr-4 font-semibold">When</th>
                      <th className="py-2 pr-4 font-semibold">Event</th>
                      <th className="py-2 pr-4 font-semibold">State</th>
                      <th className="py-2 font-semibold">Reason</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-light">
                    {detail?.events.length ? (
                      detail.events.map((event) => (
                        <tr key={event.id} className="text-text-muted">
                          <td className="py-2 pr-4">{formatExactTimestamp(event.created_at)}</td>
                          <td className="py-2 pr-4 capitalize">{formatState(event.event_type)}</td>
                          <td className="py-2 pr-4 capitalize">{formatState(event.state)}</td>
                          <td className="py-2">{formatState(event.reason)}</td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={4} className="py-8 text-center text-text-muted">
                          No dunning events recorded yet.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </Card>
          </>
        ) : null}
    </PageContainer>
  );
}

function ActionMenuItem({ label, pending, disabled, onClick }: { label: string; pending: boolean; disabled: boolean; onClick: () => void }) {
  return (
    <Button
      variant="ghost"
      size="sm"
      disabled={disabled}
      loading={pending}
      onClick={onClick}
      className="w-full justify-start rounded-none px-3 py-2"
    >
      {label}
    </Button>
  );
}
