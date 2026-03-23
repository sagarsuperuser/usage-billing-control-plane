"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { LoaderCircle, Save, Search } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchDunningPolicy, fetchDunningRuns, updateDunningPolicy } from "@/lib/api";
import { formatExactTimestamp, formatRelativeTimestamp } from "@/lib/format";
import type { DunningRun } from "@/lib/types";

const finalActionOptions = [
  { value: "manual_review", label: "Manual review" },
  { value: "pause", label: "Pause" },
  { value: "write_off_later", label: "Write off later" },
] as const;

function formatState(value?: string): string {
  if (!value) return "-";
  return value.replaceAll("_", " ");
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
      <p className="text-xs uppercase tracking-[0.16em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-600">{body}</p>
    </div>
  );
}

function MetricCard({ label, value, tone = "default" }: { label: string; value: string | number; tone?: "default" | "warn" | "danger" | "info" }) {
  const toneClass =
    tone === "danger"
      ? "border-rose-200 bg-rose-50 text-rose-700"
      : tone === "warn"
        ? "border-amber-200 bg-amber-50 text-amber-700"
        : tone === "info"
          ? "border-indigo-200 bg-indigo-50 text-indigo-700"
          : "border-stone-200 bg-stone-50 text-slate-700";
  return (
    <div className={`rounded-2xl border px-4 py-4 ${toneClass}`}>
      <p className="text-xs uppercase tracking-[0.16em]">{label}</p>
      <p className="mt-2 text-2xl font-semibold tracking-tight">{value}</p>
    </div>
  );
}

function FieldLabel({ children }: { children: string }) {
  return <label className="text-xs font-medium uppercase tracking-[0.16em] text-slate-500">{children}</label>;
}

type DunningRunDiagnosis = {
  title: string;
  nextStep: string;
  tone: "healthy" | "warning" | "danger";
};

function diagnosisToneClass(tone: DunningRunDiagnosis["tone"]): string {
  switch (tone) {
    case "healthy":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-800";
    default:
      return "border-rose-200 bg-rose-50 text-rose-800";
  }
}

function diagnoseDunningRun(run: DunningRun): DunningRunDiagnosis {
  if (run.paused) {
    return {
      title: "Run is paused",
      nextStep: "Resume or resolve this run before expecting retries or reminders to continue.",
      tone: "warning",
    };
  }
  if (run.resolved_at) {
    return {
      title: "Run resolved",
      nextStep: "Monitor only. Open run detail if you need the exact resolution trail.",
      tone: "healthy",
    };
  }
  switch (run.state) {
    case "awaiting_payment_setup":
      return {
        title: "Awaiting payment setup",
        nextStep: "Collect or refresh customer payment setup before expecting retry success.",
        tone: "danger",
      };
    case "retry_due":
      return {
        title: "Retry is due",
        nextStep: "Open the run and invoice timeline before manually retrying or overriding schedule.",
        tone: "warning",
      };
    case "escalated":
      return {
        title: "Manual review required",
        nextStep: "Open run detail and decide whether to pause, resolve, or move the invoice into deeper recovery.",
        tone: "danger",
      };
    default:
      if (run.next_action_type === "collect_payment_reminder") {
        return {
          title: "Reminder path active",
          nextStep: "Confirm the reminder goes out and that the customer can complete payment setup.",
          tone: "warning",
        };
      }
      return {
        title: "Collections active",
        nextStep: "Monitor the next action timing and open the run if the state stops progressing.",
        tone: "healthy",
      };
  }
}

function RunRow({ run }: { run: DunningRun }) {
  const diagnosis = diagnoseDunningRun(run);

  return (
    <tr className="border-t border-stone-200 text-sm text-slate-700">
      <td className="px-4 py-3 font-mono text-xs text-slate-500">{run.invoice_id}</td>
      <td className="px-4 py-3">{run.customer_external_id || "-"}</td>
      <td className="px-4 py-3 capitalize">{formatState(run.state)}</td>
      <td className="px-4 py-3 capitalize">{formatState(run.next_action_type)}</td>
      <td className="px-4 py-3">{formatExactTimestamp(run.next_action_at)}</td>
      <td className="px-4 py-3">{run.attempt_count}</td>
      <td className="px-4 py-3">
        <div className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
          <span className={`inline-flex rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${diagnosisToneClass(diagnosis.tone)}`}>
            {diagnosis.title}
          </span>
          <p className="mt-2 text-xs leading-relaxed text-slate-600">{diagnosis.nextStep}</p>
        </div>
      </td>
      <td className="px-4 py-3">
        <Link
          href={`/dunning/${encodeURIComponent(run.id)}`}
          className="inline-flex h-9 items-center rounded-lg border border-slate-200 bg-slate-50 px-3 text-xs font-semibold uppercase tracking-[0.12em] text-slate-700 transition hover:bg-slate-100"
        >
          Open run
        </Link>
      </td>
    </tr>
  );
}

export function DunningConsoleScreen() {
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [invoiceID, setInvoiceID] = useState(searchParams.get("invoice_id") || "");
  const [customerExternalID, setCustomerExternalID] = useState(searchParams.get("customer_external_id") || "");
  const [state, setState] = useState(searchParams.get("state") || "");
  const [activeOnly, setActiveOnly] = useState(searchParams.get("active_only") !== "false");

  const policyQuery = useQuery({
    queryKey: ["dunning-policy", apiBaseURL],
    queryFn: () => fetchDunningPolicy({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const [policyName, setPolicyName] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [retrySchedule, setRetrySchedule] = useState("");
  const [collectSchedule, setCollectSchedule] = useState("");
  const [maxRetryAttempts, setMaxRetryAttempts] = useState("3");
  const [gracePeriodDays, setGracePeriodDays] = useState("0");
  const [finalAction, setFinalAction] = useState<"manual_review" | "pause" | "write_off_later">("manual_review");

  useEffect(() => {
    const policy = policyQuery.data;
    if (!policy) return;
    setPolicyName(policy.name);
    setEnabled(policy.enabled);
    setRetrySchedule(policy.retry_schedule.join(", "));
    setCollectSchedule(policy.collect_payment_reminder_schedule.join(", "));
    setMaxRetryAttempts(String(policy.max_retry_attempts));
    setGracePeriodDays(String(policy.grace_period_days));
    setFinalAction(policy.final_action);
  }, [policyQuery.data]);

  const filters = useMemo(
    () => ({
      invoiceID: invoiceID.trim() || undefined,
      customerExternalID: customerExternalID.trim() || undefined,
      state: state.trim() || undefined,
      activeOnly,
      limit: 100,
      offset: 0,
    }),
    [activeOnly, customerExternalID, invoiceID, state],
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
          name: policyName.trim(),
          enabled,
          retry_schedule: retrySchedule.split(",").map((item) => item.trim()).filter(Boolean),
          max_retry_attempts: Number(maxRetryAttempts),
          collect_payment_reminder_schedule: collectSchedule.split(",").map((item) => item.trim()).filter(Boolean),
          final_action: finalAction,
          grace_period_days: Number(gracePeriodDays),
        },
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["dunning-policy", apiBaseURL] });
    },
  });

  const runs = runsQuery.data?.items ?? [];
  const stats = useMemo(
    () => ({
      total: runs.length,
      awaiting: runs.filter((item) => item.state === "awaiting_payment_setup").length,
      retryDue: runs.filter((item) => item.state === "retry_due").length,
      escalated: runs.filter((item) => item.state === "escalated").length,
    }),
    [runs],
  );

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <AppBreadcrumbs items={[{ href: "/control-plane", label: "Workspace" }, { label: "Dunning" }]} />

        <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-emerald-700">Collections Console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900 md:text-4xl">Dunning Policy + Run Inventory</h1>
              <p className="mt-2 max-w-3xl text-sm text-slate-600 md:text-base">
                Operate payment collection policy from Alpha, inspect active dunning runs, and open the exact invoice workflow before escalating into recovery.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
              <MetricCard label="Visible runs" value={stats.total} />
              <MetricCard label="Awaiting setup" value={stats.awaiting} tone="warn" />
              <MetricCard label="Retry due" value={stats.retryDue} tone="info" />
              <MetricCard label="Escalated" value={stats.escalated} tone="danger" />
            </div>
          </div>

          <div className="mt-5 grid gap-3 lg:grid-cols-3">
            <InfoCard
              title="Product ownership"
              body="Alpha owns the dunning policy and operator workflow. The billing backend is now an implementation detail, not the place operators need to configure collections."
            />
            <InfoCard
              title="Run visibility"
              body="Each row maps to one invoice-level dunning workflow, including state, next action, and the exact run detail timeline."
            />
            <InfoCard
              title="Manual actions"
              body="Operators can still trigger collect-payment reminders from payment, invoice, or run detail when they need to override the normal schedule."
            />
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Dunning is tenant-scoped. Sign in with a tenant account to inspect collection policy and invoice-level runs."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-6 xl:grid-cols-[380px_minmax(0,1fr)]">
          <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Policy</p>
                <h2 className="mt-1 text-lg font-semibold text-slate-900">Workspace dunning defaults</h2>
                <p className="mt-2 text-sm text-slate-600">Control the retry cadence and final action from Alpha instead of pushing operators into the billing backend.</p>
              </div>
              {policyQuery.isFetching ? <LoaderCircle className="h-5 w-5 animate-spin text-slate-400" /> : null}
            </div>

            {policyQuery.error ? (
              <div className="mt-4 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-4 text-sm text-rose-700">
                {(policyQuery.error as Error).message}
              </div>
            ) : null}

            <div className="mt-5 grid gap-4">
              <div className="grid gap-2">
                <FieldLabel>Policy name</FieldLabel>
                <input
                  value={policyName}
                  onChange={(event) => setPolicyName(event.target.value)}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                />
              </div>

              <label className="flex items-center gap-3 rounded-xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm text-slate-700">
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={(event) => setEnabled(event.target.checked)}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-4 w-4 rounded border-stone-300"
                />
                Dunning enabled
              </label>

              <div className="grid gap-2">
                <FieldLabel>Retry schedule</FieldLabel>
                <input
                  value={retrySchedule}
                  onChange={(event) => setRetrySchedule(event.target.value)}
                  disabled={!canWrite || policyMutation.isPending}
                  placeholder="1d, 3d, 5d"
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                />
              </div>

              <div className="grid gap-2">
                <FieldLabel>Collect-payment reminders</FieldLabel>
                <input
                  value={collectSchedule}
                  onChange={(event) => setCollectSchedule(event.target.value)}
                  disabled={!canWrite || policyMutation.isPending}
                  placeholder="0d, 2d, 5d"
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                />
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <div className="grid gap-2">
                  <FieldLabel>Max retry attempts</FieldLabel>
                  <input
                    value={maxRetryAttempts}
                    onChange={(event) => setMaxRetryAttempts(event.target.value)}
                    disabled={!canWrite || policyMutation.isPending}
                    inputMode="numeric"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                  />
                </div>
                <div className="grid gap-2">
                  <FieldLabel>Grace period days</FieldLabel>
                  <input
                    value={gracePeriodDays}
                    onChange={(event) => setGracePeriodDays(event.target.value)}
                    disabled={!canWrite || policyMutation.isPending}
                    inputMode="numeric"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <FieldLabel>Final action</FieldLabel>
                <select
                  value={finalAction}
                  onChange={(event) => setFinalAction(event.target.value as "manual_review" | "pause" | "write_off_later")}
                  disabled={!canWrite || policyMutation.isPending}
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2 disabled:bg-stone-50"
                >
                  {finalActionOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>

              <button
                type="button"
                data-testid="dunning-policy-save"
                onClick={() => policyMutation.mutate()}
                disabled={!canWrite || !csrfToken || policyMutation.isPending}
                className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {policyMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                Save policy
              </button>
            </div>
          </section>

          <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-4">
              <div>
                <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Run inventory</p>
                <h2 className="mt-1 text-lg font-semibold text-slate-900">Invoice-level dunning workflows</h2>
                <p className="mt-2 text-sm text-slate-600">Filter active runs by invoice, customer, or state before opening the detailed event and intent timeline.</p>
              </div>

              <div className="grid gap-3 lg:grid-cols-4">
                <input
                  value={invoiceID}
                  onChange={(event) => setInvoiceID(event.target.value)}
                  placeholder="Invoice ID"
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2"
                />
                <input
                  value={customerExternalID}
                  onChange={(event) => setCustomerExternalID(event.target.value)}
                  placeholder="Customer external ID"
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2"
                />
                <input
                  value={state}
                  onChange={(event) => setState(event.target.value)}
                  placeholder="State"
                  className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2"
                />
                <label className="flex h-11 items-center gap-3 rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-700">
                  <input type="checkbox" checked={activeOnly} onChange={(event) => setActiveOnly(event.target.checked)} className="h-4 w-4 rounded border-stone-300" />
                  Active only
                </label>
              </div>

              <div className="flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  onClick={() => runsQuery.refetch()}
                  disabled={runsQuery.isFetching || !isTenantSession}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {runsQuery.isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
                  Refresh runs
                </button>
                <span className="text-sm text-slate-500">Showing {runs.length} run{runs.length === 1 ? "" : "s"}</span>
              </div>
            </div>

            {runsQuery.error ? (
              <div className="mt-5 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-4 text-sm text-rose-700">
                {(runsQuery.error as Error).message}
              </div>
            ) : null}

            <div className="mt-5 overflow-hidden rounded-2xl border border-stone-200">
              <table className="min-w-full divide-y divide-stone-200 text-left">
                <thead className="bg-stone-50 text-xs uppercase tracking-[0.14em] text-slate-500">
                  <tr>
                    <th className="px-4 py-3 font-medium">Invoice</th>
                    <th className="px-4 py-3 font-medium">Customer</th>
                    <th className="px-4 py-3 font-medium">State</th>
                    <th className="px-4 py-3 font-medium">Next action</th>
                    <th className="px-4 py-3 font-medium">Due</th>
                    <th className="px-4 py-3 font-medium">Attempts</th>
                    <th className="px-4 py-3 font-medium">Diagnosis</th>
                    <th className="px-4 py-3 font-medium">Open</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-200 bg-white">
                  {runs.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="px-4 py-8 text-center text-sm text-slate-500">
                        No dunning runs matched the current filter.
                      </td>
                    </tr>
                  ) : (
                    runs.map((run) => <RunRow key={run.id} run={run} />)
                  )}
                </tbody>
              </table>
            </div>

            {runs.length > 0 ? (
              <div className="mt-4 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-slate-600">
                Oldest visible next action: {formatRelativeTimestamp(runs[runs.length - 1]?.next_action_at)}. Open a run to inspect event history and notification dispatch state.
              </div>
            ) : null}
          </section>
        </section>
      </main>
    </div>
  );
}
