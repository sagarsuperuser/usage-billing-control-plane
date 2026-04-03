"use client";

import { type ReactNode, useMemo, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Clock3,
  Download,
  LoaderCircle,
  RefreshCw,
  RotateCcw,
  Workflow,
  X,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { createReplayJob, fetchReplayJobDiagnostics, fetchReplayJobs, retryReplayJob } from "@/lib/api";
import { formatExactTimestamp, formatRelativeTimestamp } from "@/lib/format";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";
import { type ReplayJob } from "@/lib/types";

type ReplayStatusFilter = "" | "queued" | "running" | "done" | "failed";
const EMPTY_REPLAY_JOBS: ReplayJob[] = [];

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

function replayBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "queued":
      return "border border-indigo-200 bg-indigo-50 text-indigo-700";
    case "running":
      return "border border-amber-200 bg-amber-50 text-amber-700";
    case "done":
      return "border border-emerald-200 bg-emerald-50 text-emerald-700";
    case "failed":
      return "border border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border border-stone-200 bg-slate-50 text-slate-700";
  }
}

export function ReplayOperationsScreen() {
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, canWrite, role, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [customerFilter, setCustomerFilter] = useState(searchParams.get("customer_id") || "");
  const [meterFilter, setMeterFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<ReplayStatusFilter>((searchParams.get("status") as ReplayStatusFilter) || "");
  const [limit, setLimit] = useState(20);
  const [offset, setOffset] = useState(0);

  const [createCustomerID, setCreateCustomerID] = useState(searchParams.get("customer_id") || "");
  const [createMeterID, setCreateMeterID] = useState("");
  const [createFrom, setCreateFrom] = useState("");
  const [createTo, setCreateTo] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(() => generateReplayIdempotencyKey());
  const [selectedJobID, setSelectedJobID] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [flashMessage, setFlashMessage] = useState<string | null>(null);

  const jobsQuery = useQuery({
    queryKey: ["replay-jobs", apiBaseURL, customerFilter, meterFilter, statusFilter, limit, offset],
    queryFn: () =>
      fetchReplayJobs({
        runtimeBaseURL: apiBaseURL,
        customerID: customerFilter || undefined,
        meterID: meterFilter || undefined,
        status: statusFilter,
        limit,
        offset,
      }),
    enabled: isTenantSession,
  });

  const diagnosticsQuery = useQuery({
    queryKey: ["replay-job-diagnostics", apiBaseURL, selectedJobID],
    queryFn: () =>
      fetchReplayJobDiagnostics({
        runtimeBaseURL: apiBaseURL,
        jobID: selectedJobID,
      }),
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
      setFlashMessage(
        payload.idempotent_replay
          ? `Replay job ${payload.job.id} already existed for idempotency key ${payload.job.idempotency_key}.`
          : `Replay job ${payload.job.id} queued for customer ${payload.job.customer_id || "-"}.`
      );
      setSelectedJobID(payload.job.id);
      setDrawerOpen(true);
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
    mutationFn: (jobID: string) =>
      retryReplayJob({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        jobID,
      }),
    onSuccess: async (job) => {
      setFlashMessage(`Replay job ${job.id} re-queued for recovery.`);
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

  const jobs = jobsQuery.data?.items ?? EMPTY_REPLAY_JOBS;
  const totalJobs = jobsQuery.data?.total ?? jobs.length;
  const queuedCount = jobs.filter((job) => job.status === "queued").length;
  const runningCount = jobs.filter((job) => job.status === "running").length;
  const failedCount = jobs.filter((job) => job.status === "failed").length;
  const doneCount = jobs.filter((job) => job.status === "done").length;
  const selectedJob = useMemo(() => {
    const diagnosticsJob = diagnosticsQuery.data?.job;
    if (diagnosticsJob && diagnosticsJob.id === selectedJobID) {
      return diagnosticsJob;
    }
    return jobs.find((job) => job.id === selectedJobID);
  }, [diagnosticsQuery.data?.job, jobs, selectedJobID]);

  const canGoPrev = offset > 0;
  const canGoNext = jobs.length === limit;
  const createDisabled =
    !isAuthenticated ||
    !canWrite ||
    !csrfToken ||
    createMutation.isPending ||
    !createCustomerID.trim() ||
    !createMeterID.trim() ||
    !idempotencyKey.trim();

  const openDiagnostics = (job: ReplayJob) => {
    setSelectedJobID(job.id);
    setDrawerOpen(true);
  };

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">

        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { label: "Recovery" },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Recovery tools are workspace-scoped. Sign in with a workspace account to inspect replay jobs or queue recovery work."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <>
            <section className="rounded-xl border border-stone-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Recovery</p>
                  <h1 className="mt-2 text-lg font-semibold text-slate-900 md:text-4xl">Replay operations</h1>
                  <p className="mt-2 max-w-3xl text-sm text-slate-600 md:text-base">
                    Queue replay jobs, inspect diagnostics, and recover failed processing from one workspace console.
                  </p>
                </div>
                <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-5">
                  <MetricCard label="Loaded jobs" value={totalJobs} />
                  <MetricCard label="Queued" value={queuedCount} tone="info" />
                  <MetricCard label="Running" value={runningCount} tone="warn" />
                  <MetricCard label="Failed" value={failedCount} tone="danger" />
                  <MetricCard label="Done" value={doneCount} tone="success" />
                </div>
              </div>

              <div className="mt-5 grid gap-3 lg:grid-cols-3">
                <CompactRule title="Queue once" body="Use deterministic idempotency keys to avoid duplicate recovery runs." />
                <CompactRule title="Inspect first" body="Open diagnostics before retrying or creating another replay job." />
                <CompactRule title="Recover narrowly" body="Keep customer, meter, and time window scope as small as possible." />
              </div>
            </section>

            <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
              <section className="rounded-2xl border border-stone-200 bg-white p-4 shadow-sm">
                <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Jobs</p>
                    <h2 className="mt-1 text-lg font-semibold text-slate-900">Job queue</h2>
                    <p className="mt-2 max-w-2xl text-sm text-slate-600">
                      Filter the queue first, then open diagnostics on the exact job that needs recovery.
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => jobsQuery.refetch()}
                    disabled={jobsQuery.isFetching}
                    className="inline-flex h-10 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 text-sm text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {jobsQuery.isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                    Refresh
                  </button>
                </div>

                <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <InputField
                    label="Customer ID"
                    placeholder="cust_123"
                    value={customerFilter}
                    onChange={(value) => {
                      setCustomerFilter(value);
                      setOffset(0);
                    }}
                  />
                  <InputField
                    label="Meter ID"
                    placeholder="meter_abc"
                    value={meterFilter}
                    onChange={(value) => {
                      setMeterFilter(value);
                      setOffset(0);
                    }}
                  />
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-slate-600">Status</label>
                    <select
                      className="h-11 rounded-xl border border-stone-200 bg-stone-50 px-3 text-sm text-slate-700 outline-none ring-emerald-500 transition focus:ring-2"
                      value={statusFilter}
                      onChange={(event) => {
                        setStatusFilter(event.target.value as ReplayStatusFilter);
                        setOffset(0);
                      }}
                    >
                      <option value="">All</option>
                      <option value="queued">Queued</option>
                      <option value="running">Running</option>
                      <option value="done">Done</option>
                      <option value="failed">Failed</option>
                    </select>
                  </div>
                  <InputField
                    label="Limit"
                    placeholder="20"
                    value={String(limit)}
                    onChange={(value) => {
                      const next = Number(value);
                      if (Number.isFinite(next) && next > 0) {
                        setLimit(Math.min(100, Math.floor(next)));
                        setOffset(0);
                      }
                    }}
                  />
                </div>

                <div className="mt-4 overflow-auto">
                  <table className="w-full min-w-[1080px] border-separate border-spacing-y-2 text-sm">
                    <thead>
                      <tr className="text-left text-xs uppercase tracking-wider text-slate-500">
                        <th className="px-3 py-1">Job</th>
                        <th className="px-3 py-1">Summary</th>
                        <th className="px-3 py-1">Created</th>
                        <th className="px-3 py-1">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {jobs.map((job) => (
                        <tr key={job.id} data-testid={`replay-job-row-${job.id}`} className="bg-stone-50/80">
                          <td className="rounded-l-xl px-3 py-3 align-top">
                            <p className="font-medium text-emerald-700">{job.id}</p>
                            <p className={`mt-1 inline-flex rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${replayBadgeClass(job.status)}`}>
                              {job.status}
                            </p>
                          </td>
                          <td className="px-3 py-3 align-top text-xs text-slate-600">
                            <p>Customer: {job.customer_id || "-"}</p>
                            <p className="mt-1">Meter: {job.meter_id || "-"}</p>
                            <p className="mt-1">Step: {job.workflow_telemetry?.current_step || job.status}</p>
                            <p className="mt-1">Processed: {job.workflow_telemetry?.processed_records ?? job.processed_records}</p>
                          </td>
                          <td className="px-3 py-3 align-top text-xs text-slate-600">
                            <p>{formatRelativeTimestamp(job.created_at)}</p>
                            <p className="mt-1 text-slate-500">{formatExactTimestamp(job.created_at)}</p>
                          </td>
                          <td className="rounded-r-xl px-3 py-3 align-top">
                            <div className="flex flex-wrap gap-2">
                              <button
                                type="button"
                                data-testid={`replay-open-diagnostics-${job.id}`}
                                onClick={() => openDiagnostics(job)}
                                className="inline-flex h-9 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 text-xs uppercase tracking-[0.14em] text-emerald-700 transition hover:bg-emerald-100"
                              >
                                <Workflow className="h-3.5 w-3.5" />
                                Diagnostics
                              </button>
                              <button
                                type="button"
                                data-testid={`replay-retry-job-${job.id}`}
                                disabled={!canWrite || !csrfToken || job.status !== "failed" || retryMutation.isPending}
                                onClick={() => retryMutation.mutate(job.id)}
                                className="inline-flex h-9 items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 text-xs uppercase tracking-[0.14em] text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-45"
                              >
                                {retryMutation.isPending && retryMutation.variables === job.id ? (
                                  <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                                ) : (
                                  <RotateCcw className="h-3.5 w-3.5" />
                                )}
                                Retry
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {jobs.length === 0 && !jobsQuery.isLoading ? (
                    <div className="rounded-2xl border border-dashed border-stone-200 bg-stone-50 p-8 text-center text-sm text-slate-500">
                      No replay jobs matched the current filters.
                    </div>
                  ) : null}
                </div>

                <div className="mt-4 flex items-center justify-between gap-3 text-xs text-slate-500">
                  <p>
                    Offset {offset} • showing {jobs.length} rows
                  </p>
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => setOffset((current) => Math.max(0, current - limit))}
                      disabled={!canGoPrev}
                      className="inline-flex h-9 items-center gap-2 rounded-xl border border-stone-200 bg-stone-50 px-3 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <ChevronLeft className="h-4 w-4" />
                      Prev
                    </button>
                    <button
                      type="button"
                      onClick={() => setOffset((current) => current + limit)}
                      disabled={!canGoNext}
                      className="inline-flex h-9 items-center gap-2 rounded-xl border border-stone-200 bg-stone-50 px-3 text-slate-700 transition hover:bg-stone-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      Next
                      <ChevronRight className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              </section>

              <div className="space-y-6">
                <section className="rounded-2xl border border-stone-200 bg-white p-4 shadow-sm">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Policy</p>
                    <h2 className="mt-1 text-lg font-semibold text-slate-900">Guardrails</h2>
                    <p className="mt-2 text-sm text-slate-600">
                      Queue a replay only after diagnostics confirm the exact customer, meter, and time window that need reprocessing.
                    </p>
                  </div>

                  {!canWrite ? (
                    <div
                      data-testid="replay-read-only-notice"
                      className="mt-4 rounded-2xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700"
                    >
                      Current session role {role} is read-only for replay queue and recovery actions.
                    </div>
                  ) : null}

                  <div className="mt-4 rounded-2xl border border-stone-200 bg-stone-50 p-4">
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Operator checklist</p>
                    <ul className="mt-3 space-y-2 text-sm text-slate-600">
                      <li>Confirm scope in diagnostics before creating a new replay request.</li>
                      <li>Reuse an idempotency key only when you intentionally want the same replay job.</li>
                      <li>Prefer retrying an existing failed job instead of queuing a duplicate run.</li>
                    </ul>
                  </div>
                </section>
              </div>
            </section>

            <section className="rounded-lg border border-stone-200 bg-white shadow-sm p-5">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">New job</p>
                  <h2 className="mt-1 text-lg font-semibold text-slate-900">Queue a replay job</h2>
                  <p className="mt-2 max-w-3xl text-sm text-slate-600">
                    Build one replay request with an explicit scope, an optional time window, and a deterministic idempotency key. This keeps recovery controlled and reviewable.
                  </p>
                </div>
                <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm text-slate-600">
                  Local datetime inputs are converted to UTC ISO-8601 before submission.
                </div>
              </div>

              <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                <section className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
                  <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Replay scope</p>
                  <div className="mt-4 grid gap-3 md:grid-cols-2">
                    <InputField
                      label="Customer ID"
                      placeholder="cust_123"
                      value={createCustomerID}
                      onChange={setCreateCustomerID}
                      dataTestID="replay-create-customer-id"
                    />
                    <InputField
                      label="Meter ID"
                      placeholder="meter_abc"
                      value={createMeterID}
                      onChange={setCreateMeterID}
                      dataTestID="replay-create-meter-id"
                    />
                  </div>
                  <p className="mt-3 text-xs text-slate-500">Use both fields when you already know the exact replay scope from diagnostics.</p>
                </section>

                <section className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
                  <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Time window</p>
                  <div className="mt-4 grid gap-3 md:grid-cols-2">
                    <InputField label="From" type="datetime-local" value={createFrom} onChange={setCreateFrom} dataTestID="replay-create-from" />
                    <InputField label="To" type="datetime-local" value={createTo} onChange={setCreateTo} dataTestID="replay-create-to" />
                  </div>
                  <p className="mt-3 text-xs text-slate-500">Leave both empty when you need a full-scope replay for the selected customer or meter.</p>
                </section>
              </div>

              <section className="mt-4 rounded-2xl border border-stone-200 bg-stone-50 p-4">
                <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Request control</p>
                <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_auto] xl:items-end">
                  <InputField
                    label="Idempotency Key"
                    placeholder="replay-..."
                    value={idempotencyKey}
                    onChange={setIdempotencyKey}
                    dataTestID="replay-create-idempotency-key"
                  />
                  <div className="flex flex-wrap gap-3">
                    <button
                      type="button"
                      data-testid="replay-create-submit"
                      disabled={createDisabled}
                      onClick={() => createMutation.mutate()}
                      className="inline-flex h-11 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 text-sm text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-45"
                    >
                      {createMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Clock3 className="h-4 w-4" />}
                      Queue replay job
                    </button>
                    <button
                      type="button"
                      onClick={() => setIdempotencyKey(generateReplayIdempotencyKey())}
                      className="inline-flex h-11 items-center gap-2 rounded-xl border border-stone-200 bg-white px-4 text-sm text-slate-700 transition hover:bg-stone-100"
                    >
                      <RotateCcw className="h-4 w-4" />
                      Regenerate key
                    </button>
                  </div>
                </div>
              </section>
            </section>

            {flashMessage ? (
              <section data-testid="replay-flash-message" className="rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
                {flashMessage}
              </section>
            ) : null}

            {jobsQuery.error ? (
              <section className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
                {(jobsQuery.error as Error).message}
              </section>
            ) : null}
            {createMutation.error ? (
              <section className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
                {(createMutation.error as Error).message}
              </section>
            ) : null}
            {retryMutation.error ? (
              <section className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
                {(retryMutation.error as Error).message}
              </section>
            ) : null}
          </>
        ) : null}

        {isTenantSession && drawerOpen && selectedJobID ? (
          <aside
            data-testid="replay-diagnostics-drawer"
            className="fixed inset-y-0 right-0 z-30 flex w-full max-w-[560px] flex-col border-l border-stone-200 bg-white p-6 shadow-2xl"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.18em] text-emerald-700">Replay Diagnostics</p>
                <h2 className="mt-1 text-xl font-semibold text-slate-900">{selectedJob?.id || selectedJobID}</h2>
                <p className="mt-2 text-sm text-slate-600">
                  Inspect replay telemetry, matched usage/billed counts, and downloadable artifacts.
                </p>
              </div>
              <button
                type="button"
                aria-label="Close replay diagnostics"
                onClick={() => setDrawerOpen(false)}
                className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-stone-200 bg-stone-50 text-slate-700 transition hover:bg-stone-100"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              <button
                type="button"
                data-testid="replay-diagnostics-retry"
                disabled={!canWrite || !csrfToken || !selectedJob || selectedJob.status !== "failed" || retryMutation.isPending}
                onClick={() => {
                  if (selectedJob) {
                    retryMutation.mutate(selectedJob.id);
                  }
                }}
                className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 text-xs uppercase tracking-[0.14em] text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-45"
              >
                {retryMutation.isPending && retryMutation.variables === selectedJob?.id ? (
                  <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RotateCcw className="h-3.5 w-3.5" />
                )}
                Retry selected job
              </button>
              {!canWrite ? (
                <p className="inline-flex items-center rounded-xl border border-amber-200 bg-amber-50 px-3 text-xs uppercase tracking-[0.14em] text-amber-700">
                  Read-only session
                </p>
              ) : null}
            </div>

            {diagnosticsQuery.isLoading ? (
              <div className="mt-6 inline-flex items-center gap-2 text-sm text-slate-600">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading diagnostics...
              </div>
            ) : null}
            {diagnosticsQuery.error ? (
              <div className="mt-6 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
                {(diagnosticsQuery.error as Error).message}
              </div>
            ) : null}

            {diagnosticsQuery.data ? (
              <div className="mt-6 flex-1 space-y-5 overflow-auto pb-8">
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <MetricCard label="Usage events" value={diagnosticsQuery.data.usage_events_count} />
                  <MetricCard label="Usage quantity" value={diagnosticsQuery.data.usage_quantity} />
                  <MetricCard label="Billed entries" value={diagnosticsQuery.data.billed_entries_count} />
                  <MetricCard label="Billed cents" value={diagnosticsQuery.data.billed_amount_cents} />
                </div>

                <section className="rounded-2xl border border-stone-200 bg-stone-50 p-4 text-sm text-slate-700">
                  <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500">Workflow telemetry</h3>
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <MetaRow label="Status" value={diagnosticsQuery.data.job.status} badgeClass={replayBadgeClass(diagnosticsQuery.data.job.status)} />
                    <MetaRow label="Current step" value={diagnosticsQuery.data.job.workflow_telemetry?.current_step || diagnosticsQuery.data.job.status} />
                    <MetaRow label="Progress" value={`${diagnosticsQuery.data.job.workflow_telemetry?.progress_percent ?? 0}%`} />
                    <MetaRow label="Processed records" value={diagnosticsQuery.data.job.workflow_telemetry?.processed_records ?? diagnosticsQuery.data.job.processed_records} />
                    <MetaRow label="Created" value={formatExactTimestamp(diagnosticsQuery.data.job.created_at)} />
                    <MetaRow label="Last attempt" value={diagnosticsQuery.data.job.last_attempt_at ? formatExactTimestamp(diagnosticsQuery.data.job.last_attempt_at) : "-"} />
                    <MetaRow label="Customer" value={diagnosticsQuery.data.job.customer_id || "-"} />
                    <MetaRow label="Meter" value={diagnosticsQuery.data.job.meter_id || "-"} />
                  </div>
                  {diagnosticsQuery.data.job.error ? (
                    <div className="mt-4 rounded-xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">
                      {diagnosticsQuery.data.job.error}
                    </div>
                  ) : null}
                </section>

                <section className="rounded-2xl border border-stone-200 bg-stone-50 p-4 text-sm text-slate-700">
                  <h3 className="text-xs uppercase tracking-[0.18em] text-slate-500">Artifacts</h3>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {diagnosticsQuery.data.job.artifact_links?.report_json ? (
                      <ArtifactLink label="Report JSON" href={diagnosticsQuery.data.job.artifact_links.report_json} />
                    ) : null}
                    {diagnosticsQuery.data.job.artifact_links?.report_csv ? (
                      <ArtifactLink label="Report CSV" href={diagnosticsQuery.data.job.artifact_links.report_csv} />
                    ) : null}
                    {diagnosticsQuery.data.job.artifact_links?.dataset_digest ? (
                      <ArtifactLink label="Dataset Digest" href={diagnosticsQuery.data.job.artifact_links.dataset_digest} />
                    ) : null}
                  </div>
                </section>
              </div>
            ) : null}
          </aside>
        ) : null}
      </main>
    </div>
  );
}

function CompactRule({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
      <p className="text-xs uppercase tracking-[0.18em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-600">{body}</p>
    </div>
  );
}

function ArtifactLink({ href, label }: { href: string; label: string }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="inline-flex h-10 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 text-xs uppercase tracking-[0.14em] text-emerald-700 transition hover:bg-emerald-100"
    >
      <Download className="h-3.5 w-3.5" />
      {label}
    </a>
  );
}

function MetricCard({ label, value, tone = "normal" }: { label: string; value: ReactNode; tone?: "normal" | "info" | "warn" | "danger" | "success" }) {
  const toneClass =
    tone === "danger"
      ? "border-rose-400/35 bg-rose-50 text-rose-700"
      : tone === "warn"
        ? "border-amber-200 bg-amber-50 text-amber-700"
        : tone === "success"
          ? "border-emerald-200 bg-emerald-50 text-emerald-700"
          : tone === "info"
            ? "border-indigo-200 bg-indigo-50 text-indigo-700"
            : "border-stone-200 bg-stone-50 text-slate-900";
  return (
    <div className={`rounded-2xl border px-3 py-3 ${toneClass}`}>
      <p className="text-[11px] uppercase tracking-[0.12em] text-slate-600">{label}</p>
      <p className="mt-2 text-lg font-semibold">{value}</p>
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
  dataTestID,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: "text" | "datetime-local";
  dataTestID?: string;
}) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-medium uppercase tracking-wider text-slate-600">{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        data-testid={dataTestID}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 rounded-xl border border-stone-200 bg-stone-50 px-3 text-sm text-slate-700 outline-none ring-emerald-500 transition focus:ring-2"
      />
    </label>
  );
}

function MetaRow({ label, value, badgeClass }: { label: string; value: ReactNode; badgeClass?: string }) {
  return (
    <div className="rounded-xl border border-stone-200 bg-stone-50 p-3">
      <p className="text-[11px] uppercase tracking-[0.14em] text-slate-500">{label}</p>
      {badgeClass ? (
        <p className={`mt-2 inline-flex rounded-full px-2 py-1 text-xs uppercase tracking-[0.14em] ${badgeClass}`}>{value}</p>
      ) : (
        <p className="mt-2 break-words text-sm text-slate-700">{value}</p>
      )}
    </div>
  );
}
