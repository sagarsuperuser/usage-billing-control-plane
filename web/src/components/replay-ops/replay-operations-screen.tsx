import { useMemo, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Download,
  LoaderCircle,
  Plus,
  RotateCcw,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { DateTimePicker } from "@/components/ui/date-picker";
import { StatusChip } from "@/components/ui/status-chip";
import { createReplayJob, fetchReplayJobDiagnostics, fetchReplayJobs, retryReplayJob } from "@/lib/api";
import { statusTone } from "@/lib/badge";
import { formatExactTimestamp, formatRelativeTimestamp } from "@/lib/format";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

type ReplayStatusFilter = "" | "queued" | "running" | "done" | "failed";

function generateReplayIdempotencyKey(): string {
  return `replay-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function normalizeDateTimeToISOString(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) return undefined;
  return date.toISOString();
}

export function ReplayOperationsScreen() {
  const searchParams = useSearchParamsCompat();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isLoading: sessionLoading, canWrite, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [customerFilter, _setCustomerFilter] = useState(searchParams.get("customer_id") || "");
  const [statusFilter, setStatusFilter] = useState<ReplayStatusFilter>((searchParams.get("status") as ReplayStatusFilter) || "");
  const [limit] = useState(20);
  const [offset, setOffset] = useState(0);
  const [search, setSearch] = useState("");

  // Create modal state
  const [createOpen, setCreateOpen] = useState(false);
  const [createCustomerID, setCreateCustomerID] = useState(searchParams.get("customer_id") || "");
  const [createMeterID, setCreateMeterID] = useState("");
  const [createFrom, setCreateFrom] = useState("");
  const [createTo, setCreateTo] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(() => generateReplayIdempotencyKey());

  // Diagnostics drawer
  const [selectedJobID, setSelectedJobID] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);

  const jobsQuery = useQuery({
    queryKey: ["replay-jobs", apiBaseURL, customerFilter, statusFilter, limit, offset],
    queryFn: () =>
      fetchReplayJobs({
        runtimeBaseURL: apiBaseURL,
        customerID: customerFilter || undefined,
        status: statusFilter,
        limit,
        offset,
      }),
    enabled: isTenantSession,
  });

  const diagnosticsQuery = useQuery({
    queryKey: ["replay-job-diagnostics", apiBaseURL, selectedJobID],
    queryFn: () => fetchReplayJobDiagnostics({ runtimeBaseURL: apiBaseURL, jobID: selectedJobID }),
    enabled: isTenantSession && drawerOpen && selectedJobID.length > 0,
  });

  const createMutation = useMutation({
    mutationFn: () =>
      createReplayJob({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        customerID: createCustomerID.trim(),
        meterID: createMeterID.trim(),
        from: normalizeDateTimeToISOString(createFrom),
        to: normalizeDateTimeToISOString(createTo),
        idempotencyKey: idempotencyKey.trim(),
      }),
    onSuccess: async (payload) => {
      setSelectedJobID(payload.job.id);
      setDrawerOpen(true);
      setCreateOpen(false);
      setIdempotencyKey(generateReplayIdempotencyKey());
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["replay-jobs"] }),
        queryClient.invalidateQueries({ queryKey: ["replay-job-diagnostics", apiBaseURL, payload.job.id] }),
      ]);
    },
    onError: (err: Error) => {
      showError("Replay queue failed", err.message || "Could not queue replay job.");
    },
  });

  const retryMutation = useMutation({
    mutationFn: (jobID: string) => retryReplayJob({ runtimeBaseURL: apiBaseURL, csrfToken, jobID }),
    onSuccess: async (job) => {
      setSelectedJobID(job.id);
      setDrawerOpen(true);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["replay-jobs"] }),
        queryClient.invalidateQueries({ queryKey: ["replay-job-diagnostics", apiBaseURL, job.id] }),
      ]);
    },
    onError: (err: Error) => {
      showError("Retry failed", err.message || "Could not retry replay job.");
    },
  });

  const jobs = jobsQuery.data?.items ?? [];
  const filteredJobs = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return jobs;
    return jobs.filter((j) => j.id.toLowerCase().includes(term) || (j.customer_id || "").toLowerCase().includes(term) || (j.meter_id || "").toLowerCase().includes(term));
  }, [jobs, search]);

  const selectedJob = useMemo(() => {
    const diagnosticsJob = diagnosticsQuery.data?.job;
    if (diagnosticsJob && diagnosticsJob.id === selectedJobID) return diagnosticsJob;
    return jobs.find((job) => job.id === selectedJobID);
  }, [diagnosticsQuery.data?.job, jobs, selectedJobID]);

  const canGoPrev = offset > 0;
  const canGoNext = jobs.length === limit;
  const createDisabled = !canWrite || !csrfToken || createMutation.isPending || !createCustomerID.trim() || !createMeterID.trim() || !idempotencyKey.trim();

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Recovery" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Replay jobs</h1>
              <button
                type="button"
                onClick={() => setCreateOpen(true)}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800"
              >
                <Plus className="h-3.5 w-3.5" />
                New job
              </button>
            </div>

            {/* Filters */}
            <div className="flex items-center gap-2 border-b border-stone-200 px-5 py-2">
              <select
                value={statusFilter}
                onChange={(e) => { setStatusFilter(e.target.value as ReplayStatusFilter); setOffset(0); }}
                className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="">All statuses</option>
                <option value="queued">Queued</option>
                <option value="running">Running</option>
                <option value="done">Done</option>
                <option value="failed">Failed</option>
              </select>
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                aria-label="Search" placeholder="Search..."
                className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>

            {sessionLoading || jobsQuery.isLoading ? (
              <div className="px-5 py-10 text-center text-sm text-slate-500">Loading replay jobs...</div>
            ) : filteredJobs.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
                <p className="text-sm font-medium text-slate-700">No replay jobs</p>
                <p className="text-xs text-slate-500">No jobs matched the current filters.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">ID</th>
                    <th className="px-4 py-2.5 font-semibold">Customer</th>
                    <th className="px-4 py-2.5 font-semibold">Meter</th>
                    <th className="px-4 py-2.5 font-semibold">Status</th>
                    <th className="px-4 py-2.5 font-semibold">Created</th>
                    <th className="px-4 py-2.5 font-semibold"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {filteredJobs.map((job) => (
                    <tr
                      key={job.id}
                      data-testid={`replay-job-row-${job.id}`}
                      className="cursor-pointer transition hover:bg-stone-50"
                      onClick={() => { setSelectedJobID(job.id); setDrawerOpen(true); }}
                    >
                      <td className="px-5 py-3 font-mono text-xs text-slate-600">{job.id}</td>
                      <td className="px-4 py-3 text-slate-600">{job.customer_id || "-"}</td>
                      <td className="px-4 py-3 text-slate-600">{job.meter_id || "-"}</td>
                      <td className="px-4 py-3">
                        <StatusChip tone={statusTone(job.status)}>{job.status}</StatusChip>
                      </td>
                      <td className="px-4 py-3 text-slate-600">{formatRelativeTimestamp(job.created_at)}</td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          data-testid={`replay-retry-job-${job.id}`}
                          disabled={!canWrite || !csrfToken || job.status !== "failed" || retryMutation.isPending}
                          onClick={(e) => { e.stopPropagation(); retryMutation.mutate(job.id); }}
                          className="inline-flex h-7 items-center gap-1.5 rounded-md border border-stone-200 px-2 text-xs text-slate-600 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          {retryMutation.isPending && retryMutation.variables === job.id ? (
                            <LoaderCircle className="h-3 w-3 animate-spin" />
                          ) : (
                            <RotateCcw className="h-3 w-3" />
                          )}
                          Retry
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}

            {/* Pagination */}
            <div className="flex items-center justify-between border-t border-stone-200 px-5 py-2 text-xs text-slate-500">
              <span>Showing {filteredJobs.length} job{filteredJobs.length === 1 ? "" : "s"}</span>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setOffset((c) => Math.max(0, c - limit))}
                  disabled={!canGoPrev}
                  className="inline-flex h-7 items-center gap-1 rounded-md border border-stone-200 px-2 text-slate-600 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ChevronLeft className="h-3.5 w-3.5" /> Prev
                </button>
                <button
                  type="button"
                  onClick={() => setOffset((c) => c + limit)}
                  disabled={!canGoNext}
                  className="inline-flex h-7 items-center gap-1 rounded-md border border-stone-200 px-2 text-slate-600 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Next <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>

            {jobsQuery.error ? (
              <div className="border-t border-stone-200 px-5 py-3 text-sm text-rose-700">
                {(jobsQuery.error as Error).message}
              </div>
            ) : null}
          </div>
      </main>

      {/* Create job modal */}
      {createOpen ? (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/30">
          <div className="w-full max-w-lg rounded-lg border border-stone-200 bg-white shadow-lg">
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h2 className="text-sm font-semibold text-slate-900">New replay job</h2>
              <button type="button" onClick={() => setCreateOpen(false)} className="text-slate-400 hover:text-slate-700">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="grid gap-4 px-5 py-4">
              <div className="grid grid-cols-2 gap-3">
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-slate-600">Customer ID</span>
                  <input
                    value={createCustomerID}
                    onChange={(e) => setCreateCustomerID(e.target.value)}
                    placeholder="cust_123"
                    data-testid="replay-create-customer-id"
                    className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm outline-none ring-slate-400 transition focus:ring-2"
                  />
                </label>
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-slate-600">Meter ID</span>
                  <input
                    value={createMeterID}
                    onChange={(e) => setCreateMeterID(e.target.value)}
                    placeholder="meter_abc"
                    data-testid="replay-create-meter-id"
                    className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm outline-none ring-slate-400 transition focus:ring-2"
                  />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-slate-600">From</span>
                  <DateTimePicker value={createFrom} onChange={setCreateFrom} placeholder="Select start" aria-label="Replay from time" />
                </label>
                <label className="grid gap-1.5">
                  <span className="text-xs font-medium text-slate-600">To</span>
                  <DateTimePicker value={createTo} onChange={setCreateTo} placeholder="Select end" aria-label="Replay to time" />
                </label>
              </div>
              <label className="grid gap-1.5">
                <span className="text-xs font-medium text-slate-600">Idempotency key</span>
                <div className="flex gap-2">
                  <input
                    value={idempotencyKey}
                    onChange={(e) => setIdempotencyKey(e.target.value)}
                    data-testid="replay-create-idempotency-key"
                    className="h-9 flex-1 rounded-lg border border-stone-200 bg-white px-3 text-sm outline-none ring-slate-400 transition focus:ring-2"
                  />
                  <button
                    type="button"
                    onClick={() => setIdempotencyKey(generateReplayIdempotencyKey())}
                    className="h-9 rounded-lg border border-stone-200 px-2 text-slate-500 transition hover:bg-stone-50"
                  >
                    <RotateCcw className="h-3.5 w-3.5" />
                  </button>
                </div>
              </label>

              {createMutation.error ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                  {(createMutation.error as Error).message}
                </div>
              ) : null}
            </div>
            <div className="flex justify-end gap-2 border-t border-stone-200 px-5 py-3">
              <button
                type="button"
                onClick={() => setCreateOpen(false)}
                className="h-8 rounded-lg border border-stone-200 px-3 text-sm font-medium text-slate-700 transition hover:bg-stone-50"
              >
                Cancel
              </button>
              <button
                type="button"
                data-testid="replay-create-submit"
                disabled={createDisabled}
                onClick={() => createMutation.mutate()}
                className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {createMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                Queue job
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* Diagnostics drawer */}
      {isTenantSession && drawerOpen && selectedJobID ? (
        <aside
          data-testid="replay-diagnostics-drawer"
          className="fixed inset-y-0 right-0 z-30 flex w-full max-w-[480px] flex-col border-l border-stone-200 bg-white shadow-lg"
        >
          <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
            <h2 className="text-sm font-semibold text-slate-900">{selectedJob?.id || selectedJobID}</h2>
            <button type="button" aria-label="Close" onClick={() => setDrawerOpen(false)} className="text-slate-400 hover:text-slate-700">
              <X className="h-4 w-4" />
            </button>
          </div>

          <div className="flex items-center gap-2 border-b border-stone-200 px-5 py-2">
            <button
              type="button"
              data-testid="replay-diagnostics-retry"
              disabled={!canWrite || !csrfToken || !selectedJob || selectedJob.status !== "failed" || retryMutation.isPending}
              onClick={() => { if (selectedJob) retryMutation.mutate(selectedJob.id); }}
              className="inline-flex h-7 items-center gap-1.5 rounded-md border border-stone-200 px-2 text-xs text-slate-600 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
            >
              {retryMutation.isPending && retryMutation.variables === selectedJob?.id ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <RotateCcw className="h-3 w-3" />}
              Retry
            </button>
            {selectedJob ? (
              <StatusChip tone={statusTone(selectedJob.status)}>{selectedJob.status}</StatusChip>
            ) : null}
          </div>

          <div className="flex-1 overflow-auto px-5 py-4">
            {diagnosticsQuery.isLoading ? (
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" /> Loading...
              </div>
            ) : null}
            {diagnosticsQuery.error ? (
              <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {(diagnosticsQuery.error as Error).message}
              </div>
            ) : null}

            {diagnosticsQuery.data ? (
              <div className="space-y-5">
                <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
                  <div className="flex justify-between"><dt className="text-slate-500">Customer</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.customer_id || "-"}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Meter</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.meter_id || "-"}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Step</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.workflow_telemetry?.current_step || diagnosticsQuery.data.job.status}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Progress</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.workflow_telemetry?.progress_percent ?? 0}%</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Records</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.workflow_telemetry?.processed_records ?? diagnosticsQuery.data.job.processed_records}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Created</dt><dd className="text-slate-700">{formatExactTimestamp(diagnosticsQuery.data.job.created_at)}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Last attempt</dt><dd className="text-slate-700">{diagnosticsQuery.data.job.last_attempt_at ? formatExactTimestamp(diagnosticsQuery.data.job.last_attempt_at) : "-"}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Usage events</dt><dd className="text-slate-700">{diagnosticsQuery.data.usage_events_count}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Usage qty</dt><dd className="text-slate-700">{diagnosticsQuery.data.usage_quantity}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Billed entries</dt><dd className="text-slate-700">{diagnosticsQuery.data.billed_entries_count}</dd></div>
                  <div className="flex justify-between"><dt className="text-slate-500">Billed cents</dt><dd className="text-slate-700">{diagnosticsQuery.data.billed_amount_cents}</dd></div>
                </dl>

                {diagnosticsQuery.data.job.error ? (
                  <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                    {diagnosticsQuery.data.job.error}
                  </div>
                ) : null}

                {diagnosticsQuery.data.job.artifact_links ? (
                  <div>
                    <h3 className="text-xs font-semibold uppercase tracking-[0.1em] text-slate-400">Artifacts</h3>
                    <div className="mt-2 flex flex-wrap gap-2">
                      {diagnosticsQuery.data.job.artifact_links.report_json ? (
                        <ArtifactLink label="Report JSON" href={diagnosticsQuery.data.job.artifact_links.report_json} />
                      ) : null}
                      {diagnosticsQuery.data.job.artifact_links.report_csv ? (
                        <ArtifactLink label="Report CSV" href={diagnosticsQuery.data.job.artifact_links.report_csv} />
                      ) : null}
                      {diagnosticsQuery.data.job.artifact_links.dataset_digest ? (
                        <ArtifactLink label="Dataset Digest" href={diagnosticsQuery.data.job.artifact_links.dataset_digest} />
                      ) : null}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        </aside>
      ) : null}
    </div>
  );
}

function ArtifactLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="inline-flex h-7 items-center gap-1.5 rounded-md border border-stone-200 px-2 text-xs text-slate-600 transition hover:bg-stone-50"
    >
      <Download className="h-3 w-3" />
      {label}
    </a>
  );
}
