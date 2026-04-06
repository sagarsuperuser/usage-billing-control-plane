import { Link } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { LoaderCircle, Save, X } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusChip } from "@/components/ui/status-chip";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchDunningPolicy, fetchDunningRuns, updateDunningPolicy } from "@/lib/api";
import { statusTone } from "@/lib/badge";
import { formatExactTimestamp } from "@/lib/format";
import { showError } from "@/lib/toast";
import type { DunningPolicy } from "@/lib/types";

type DunningPolicyDraftState = {
  sourceKey: string;
  policyName: string;
  enabled: boolean;
  retrySchedule: string;
  collectSchedule: string;
  maxRetryAttempts: string;
  gracePeriodDays: string;
  finalAction: "manual_review" | "pause" | "write_off_later";
};

const finalActionOptions = [
  { value: "manual_review", label: "Manual review" },
  { value: "pause", label: "Pause" },
  { value: "write_off_later", label: "Write off later" },
] as const;

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}


const PAGE_SIZE = 20;

export function DunningConsoleScreen() {
  const searchParams = useSearchParamsCompat();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [state, setState] = useState(searchParams.get("state") || "");
  const [page, setPage] = useState(1);
  const [policyOpen, setPolicyOpen] = useState(false);

  const policyQuery = useQuery({
    queryKey: ["dunning-policy", apiBaseURL],
    queryFn: () => fetchDunningPolicy({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const [policyDraftState, setPolicyDraftState] = useState<DunningPolicyDraftState>(defaultPolicyDraftState(""));
  const policy = policyQuery.data ?? null;
  const policySourceKey = policy ? `${policy.id}:${policy.updated_at}` : "";
  const policyDraft =
    policy && policyDraftState.sourceKey === policySourceKey
      ? policyDraftState
      : policy
        ? policyDraftStateFromPolicy(policy, policySourceKey)
        : defaultPolicyDraftState(policySourceKey);

  const filters = useMemo(
    () => ({
      state: state.trim() || undefined,
      activeOnly: true,
      limit: 100,
      offset: 0,
    }),
    [state],
  );

  const runsQuery = useQuery({
    queryKey: ["dunning-runs", apiBaseURL, filters],
    queryFn: () => fetchDunningRuns({ runtimeBaseURL: apiBaseURL, ...filters }),
    enabled: isTenantSession,
  });

  const policyMutation = useMutation({
    mutationFn: () =>
      updateDunningPolicy({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name: policyDraft.policyName.trim(),
          enabled: policyDraft.enabled,
          retry_schedule: policyDraft.retrySchedule.split(",").map((item) => item.trim()).filter(Boolean),
          max_retry_attempts: Number(policyDraft.maxRetryAttempts),
          collect_payment_reminder_schedule: policyDraft.collectSchedule.split(",").map((item) => item.trim()).filter(Boolean),
          final_action: policyDraft.finalAction,
          grace_period_days: Number(policyDraft.gracePeriodDays),
        },
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["dunning-policy", apiBaseURL] });
      setPolicyOpen(false);
    },
    onError: (err: Error) => showError(err.message),
  });

  const runs = runsQuery.data?.items ?? [];
  const paginatedRuns = useMemo(() => runs.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [runs, page]);

  const updatePolicyDraft = (patch: Partial<Omit<DunningPolicyDraftState, "sourceKey">>) => {
    setPolicyDraftState({ ...policyDraft, ...patch, sourceKey: policySourceKey });
  };

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Dunning" }]} />


        <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h1 className="text-sm font-semibold text-text-primary">Dunning runs{runs.length > 0 ? ` (${runs.length})` : ""}</h1>
              <div className="flex items-center gap-2">
                <select
                  value={state}
                  onChange={(event) => { setState(event.target.value); setPage(1); }}
                  className="h-8 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All states</option>
                  <option value="retry_due">Retry due</option>
                  <option value="awaiting_payment_setup">Awaiting setup</option>
                  <option value="escalated">Escalated</option>
                  <option value="resolved">Resolved</option>
                </select>
                <button
                  type="button"
                  onClick={() => setPolicyOpen(true)}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700"
                >
                  Edit policy
                </button>
              </div>
            </div>

            {sessionLoading || runsQuery.isLoading ? (
              <div className="divide-y divide-border-light">
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4 px-5 py-3">
                    <div className="flex-1"><Skeleton className="h-4 w-32" /><Skeleton className="mt-1 h-3 w-20" /></div>
                    <Skeleton className="h-4 w-14 rounded-full" />
                    <Skeleton className="h-3 w-16" />
                  </div>
                ))}
              </div>
            ) : paginatedRuns.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
                <p className="text-sm font-medium text-text-secondary">No dunning runs</p>
                <p className="text-xs text-text-muted">No runs matched the current filter.</p>
              </div>
            ) : (
              <>
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                      <th className="px-5 py-2.5 font-semibold">Invoice</th>
                      <th className="px-4 py-2.5 font-semibold">Customer</th>
                      <th className="px-4 py-2.5 font-semibold">State</th>
                      <th className="px-4 py-2.5 font-semibold">Next action</th>
                      <th className="px-4 py-2.5 font-semibold">Next action at</th>
                      <th className="px-4 py-2.5 font-semibold">Attempts</th>
                      <th className="px-4 py-2.5 font-semibold"></th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border-light">
                    {paginatedRuns.map((run) => (
                      <tr key={run.id} className="transition hover:bg-surface-secondary">
                        <td className="px-5 py-3 font-mono text-xs text-text-muted">{run.invoice_id}</td>
                        <td className="px-4 py-3 text-text-muted">{run.customer_external_id || "-"}</td>
                        <td className="px-4 py-3">
                          <StatusChip tone={statusTone(run.state)}>{formatState(run.state)}</StatusChip>
                        </td>
                        <td className="px-4 py-3 capitalize text-text-muted">{formatState(run.next_action_type)}</td>
                        <td className="px-4 py-3 text-text-muted">{formatExactTimestamp(run.next_action_at)}</td>
                        <td className="px-4 py-3 text-text-muted">{run.attempt_count}</td>
                        <td className="px-4 py-3">
                          <Link
                            to={`/dunning/${encodeURIComponent(run.id)}`}
                            className="text-sm font-medium text-text-muted hover:text-text-primary"
                          >
                            View →
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination page={page} pageSize={PAGE_SIZE} total={runs.length} onPageChange={setPage} />
              </>
            )}

            {runsQuery.error ? (
              <div className="border-t border-border px-5 py-3 text-sm text-rose-700">
                {(runsQuery.error as Error).message}
              </div>
            ) : null}
          </div>
      </main>

      {/* Policy edit modal */}
      {policyOpen ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-lg rounded-lg border border-border bg-surface shadow-lg">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h2 className="text-sm font-semibold text-text-primary">Edit dunning policy</h2>
              <button type="button" onClick={() => setPolicyOpen(false)} className="text-text-faint hover:text-text-secondary">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="grid gap-4 px-5 py-4">
              {policyQuery.error ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                  {(policyQuery.error as Error).message}
                </div>
              ) : null}

              <label className="grid gap-1.5">
                <span className="text-xs font-medium text-text-muted">Policy name</span>
                <input
                  value={policyDraft.policyName}
                  onChange={(e) => updatePolicyDraft({ policyName: e.target.value })}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                />
              </label>

              <label className="flex items-center gap-2.5 text-sm text-text-secondary">
                <input
                  type="checkbox"
                  checked={policyDraft.enabled}
                  onChange={(e) => updatePolicyDraft({ enabled: e.target.checked })}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-4 w-4 rounded border-stone-300"
                />
                Dunning enabled
              </label>

              <label className="grid gap-1.5">
                <span className="text-xs font-medium text-text-muted">Retry schedule</span>
                <input
                  value={policyDraft.retrySchedule}
                  onChange={(e) => updatePolicyDraft({ retrySchedule: e.target.value })}
                  disabled={!canWrite || policyMutation.isPending}
                  placeholder="1d, 3d, 5d"
                  className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                />
              </label>

              <label className="grid gap-1.5">
                <span className="text-xs font-medium text-text-muted">Collect-payment reminders</span>
                <input
                  value={policyDraft.collectSchedule}
                  onChange={(e) => updatePolicyDraft({ collectSchedule: e.target.value })}
                  disabled={!canWrite || policyMutation.isPending}
                  placeholder="0d, 2d, 5d"
                  className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                />
              </label>

              <div className="grid grid-cols-2 gap-3">
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-text-muted">Max retry attempts</span>
                  <input
                    value={policyDraft.maxRetryAttempts}
                    onChange={(e) => updatePolicyDraft({ maxRetryAttempts: e.target.value })}
                    disabled={!canWrite || policyMutation.isPending}
                    inputMode="numeric"
                    className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                  />
                </label>
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-text-muted">Grace period days</span>
                  <input
                    value={policyDraft.gracePeriodDays}
                    onChange={(e) => updatePolicyDraft({ gracePeriodDays: e.target.value })}
                    disabled={!canWrite || policyMutation.isPending}
                    inputMode="numeric"
                    className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                  />
                </label>
              </div>

              <label className="grid gap-1.5">
                <span className="text-xs font-medium text-text-muted">Final action</span>
                <select
                  value={policyDraft.finalAction}
                  onChange={(e) => updatePolicyDraft({ finalAction: e.target.value as "manual_review" | "pause" | "write_off_later" })}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-9 rounded-lg border border-border bg-surface px-3 text-sm outline-none ring-slate-400 transition focus:ring-2 disabled:bg-surface-secondary"
                >
                  {finalActionOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>{opt.label}</option>
                  ))}
                </select>
              </label>

              {policyMutation.error ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                  {(policyMutation.error as Error).message}
                </div>
              ) : null}
            </div>
            <div className="flex justify-end gap-2 border-t border-border px-5 py-3">
              <button
                type="button"
                onClick={() => setPolicyOpen(false)}
                className="h-8 rounded-lg border border-border px-3 text-sm font-medium text-text-secondary transition hover:bg-surface-secondary"
              >
                Cancel
              </button>
              <button
                type="button"
                data-testid="dunning-policy-save"
                onClick={() => policyMutation.mutate()}
                disabled={!canWrite || !csrfToken || policyMutation.isPending}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {policyMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
                Save policy
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function defaultPolicyDraftState(sourceKey: string): DunningPolicyDraftState {
  return {
    sourceKey,
    policyName: "",
    enabled: true,
    retrySchedule: "",
    collectSchedule: "",
    maxRetryAttempts: "3",
    gracePeriodDays: "0",
    finalAction: "manual_review",
  };
}

function policyDraftStateFromPolicy(policy: DunningPolicy, sourceKey: string): DunningPolicyDraftState {
  return {
    sourceKey,
    policyName: policy.name,
    enabled: policy.enabled,
    retrySchedule: policy.retry_schedule.join(", "),
    collectSchedule: policy.collect_payment_reminder_schedule.join(", "),
    maxRetryAttempts: String(policy.max_retry_attempts),
    gracePeriodDays: String(policy.grace_period_days),
    finalAction: policy.final_action,
  };
}
