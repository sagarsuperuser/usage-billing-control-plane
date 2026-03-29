"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { ArrowLeft, CreditCard, LoaderCircle, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchPlans, fetchSubscription, requestSubscriptionPaymentSetup, resendSubscriptionPaymentSetup, updateSubscription } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

function tone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "active":
    case "ready":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending":
    case "pending_payment_setup":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "action_required":
    case "error":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

function formatSubscriptionPaymentSetupStatus(status: string): string {
  switch (status) {
    case "missing":
      return "Not requested";
    case "pending":
      return "Pending";
    case "ready":
      return "Ready";
    case "error":
      return "Action required";
    default:
      return formatReadinessStatus(status);
  }
}

export function SubscriptionDetailScreen({ subscriptionID }: { subscriptionID: string }) {
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, scope } = useUISession();
  const [selectedPlanID, setSelectedPlanID] = useState("");

  const detailQuery = useQuery({
    queryKey: ["subscription", apiBaseURL, subscriptionID],
    queryFn: () => fetchSubscription({ runtimeBaseURL: apiBaseURL, subscriptionID }),
    enabled: isAuthenticated && scope === "tenant" && subscriptionID.trim().length > 0,
  });

  const plansQuery = useQuery({
    queryKey: ["plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const requestMutation = useMutation({
    mutationFn: () => requestSubscriptionPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, subscriptionID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const resendMutation = useMutation({
    mutationFn: () => resendSubscriptionPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, subscriptionID }),
    onSuccess: async () => {
      await detailQuery.refetch();
    },
  });

  const updateMutation = useMutation({
    mutationFn: (body: { plan_id?: string; status?: string }) =>
      updateSubscription({ runtimeBaseURL: apiBaseURL, csrfToken, subscriptionID, body }),
    onSuccess: async (updated) => {
      setSelectedPlanID(updated.plan_id);
      await detailQuery.refetch();
    },
  });

  const subscription = detailQuery.data ?? null;
  const nextActions = normalizeMissingSteps(subscription?.missing_steps).map(describeCustomerMissingStep);
  const canRequestSetup = subscription?.payment_setup_status !== "ready" && subscription?.status !== "archived";
  const showResend = Boolean(subscription?.payment_setup_requested_at || subscription?.payment_setup_status === "error");
  const setupActionLabel = showResend ? "Resend payment setup" : "Request payment setup";
  const latestSetupCheckoutURL = resendMutation.data?.checkout_url || requestMutation.data?.checkout_url;

  const activePlans = useMemo(
    () => (plansQuery.data ?? []).filter((plan) => plan.status === "active"),
    [plansQuery.data],
  );
  const selectedPlanIDValue =
    selectedPlanID && activePlans.some((plan) => plan.id === selectedPlanID)
      ? selectedPlanID
      : subscription?.plan_id || "";
  const selectedPlan = activePlans.find((plan) => plan.id === selectedPlanIDValue) ?? null;
  const canChangePlan = Boolean(
    subscription &&
      subscription.status !== "archived" &&
      selectedPlanIDValue.trim().length > 0 &&
      selectedPlanIDValue !== subscription.plan_id,
  );
  const canArchive = Boolean(subscription && subscription.status !== "archived");

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: subscription?.display_name || subscriptionID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Subscription detail is workspace-scoped. Sign in with a workspace account to inspect readiness and request payment setup."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {detailQuery.isLoading ? (
          <LoadingPanel label="Loading subscription detail" />
        ) : detailQuery.isError || !subscription ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Subscription</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Subscription not available</h1>
            <p className="mt-3 text-sm text-slate-600">The requested subscription could not be loaded from the workspace APIs.</p>
            <Link href="/subscriptions" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
              <ArrowLeft className="h-4 w-4" />
              Back to subscriptions
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Subscription</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{subscription.display_name}</h1>
                  <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-600">
                    <span className="font-mono text-xs text-slate-500">{subscription.code}</span>
                    <span data-testid="subscription-status-badge" className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(subscription.status)}`}>
                      {formatReadinessStatus(subscription.status)}
                    </span>
                  </div>
                </div>
                <Link href="/subscriptions" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                  <ArrowLeft className="h-4 w-4" />
                  Back to subscriptions
                </Link>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Subscription" value={subscription.status} helper={subscription.plan_name} />
              <SummaryStat label="Payment setup" value={formatSubscriptionPaymentSetupStatus(subscription.payment_setup_status)} helper={subscription.default_payment_method_verified ? "Verified" : "Waiting on payer"} raw />
              <SummaryStat label="Customer" value={subscription.customer_display_name} helper={subscription.customer_external_id} raw />
              <SummaryStat label="Billing" value={`${(subscription.base_amount_cents / 100).toFixed(2)} ${subscription.currency}`} helper={subscription.billing_interval} raw />
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(320px,400px)]">
              <div className="min-w-0 grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Lifecycle</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">What still needs action</h2>
                    </div>
                    <span className={`self-start rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] sm:shrink-0 ${tone(subscription.status)}`}>
                      {formatReadinessStatus(subscription.status)}
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Subscription is billing-ready." />}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial controls</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Change plan or cancel billing</h2>
                  <p className="mt-3 text-sm leading-relaxed text-slate-600">
                    Change the billed plan here or cancel the subscription when billing should stop.
                  </p>
                  <div className="mt-5 grid gap-4">
                    <div className="grid gap-2">
                      <label htmlFor="subscription-plan-select" className="text-sm font-medium text-slate-800">Target plan</label>
                      <select
                        id="subscription-plan-select"
                        data-testid="subscription-plan-select"
                        value={selectedPlanIDValue}
                        onChange={(event) => setSelectedPlanID(event.target.value)}
                        disabled={!canWrite || !csrfToken || updateMutation.isPending || plansQuery.isLoading || subscription.status === "archived"}
                        className="h-11 rounded-xl border border-slate-200 bg-white px-3 text-sm text-slate-900 shadow-sm disabled:cursor-not-allowed disabled:bg-slate-50"
                      >
                        {activePlans.map((plan) => (
                          <option key={plan.id} value={plan.id}>
                            {plan.name} ({plan.code})
                          </option>
                        ))}
                      </select>
                      <p className="text-xs text-slate-500">
                        {selectedPlan ? `Selected plan code: ${selectedPlan.code}` : "No active target plan is available."}
                      </p>
                      <div className="flex flex-wrap gap-3">
                        <button
                          type="button"
                          data-testid="subscription-change-plan"
                          onClick={() => updateMutation.mutate({ plan_id: selectedPlanIDValue })}
                          disabled={!canWrite || !csrfToken || updateMutation.isPending || !canChangePlan}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {updateMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                          Apply plan change
                        </button>
                        <button
                          type="button"
                          data-testid="subscription-archive"
                          onClick={() => updateMutation.mutate({ status: "archived" })}
                          disabled={!canWrite || !csrfToken || updateMutation.isPending || !canArchive}
                          className="inline-flex h-10 items-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-4 text-sm font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {updateMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                          Cancel subscription
                        </button>
                      </div>
                      {updateMutation.isError ? (
                        <p className="text-sm text-rose-700">{updateMutation.error instanceof Error ? updateMutation.error.message : "Subscription update failed."}</p>
                      ) : null}
                    </div>
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment setup</p>
                  <p className="mt-3 text-sm leading-relaxed text-slate-600">
                    Request the setup path here, then verify that the payer completed it before treating the subscription as ready.
                  </p>
                  <div className="mt-4 rounded-xl border border-slate-200 bg-slate-50 p-4">
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Operator action</p>
                        <p className="mt-2 max-w-2xl text-sm text-slate-600">
                          Use one setup request action here. If the payer already received a setup email, resend that same path.
                        </p>
                      </div>
                      <div className="flex flex-wrap gap-3">
                        {canRequestSetup ? (
                          <button
                            type="button"
                            data-testid="subscription-request-setup"
                            onClick={() => (showResend ? resendMutation.mutate() : requestMutation.mutate())}
                            disabled={!canWrite || !csrfToken || requestMutation.isPending || resendMutation.isPending}
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {requestMutation.isPending || resendMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : showResend ? <Send className="h-4 w-4" /> : <CreditCard className="h-4 w-4" />}
                            {setupActionLabel}
                          </button>
                        ) : null}
                        {latestSetupCheckoutURL ? (
                          <a href={latestSetupCheckoutURL} target="_blank" rel="noreferrer" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-white px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                            Open latest setup link
                          </a>
                        ) : null}
                      </div>
                    </div>
                  </div>
                </section>
              </div>

              <aside className="min-w-0 grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial context</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={subscription.customer_display_name} />
                    <MetaItem label="Customer ID" value={subscription.customer_external_id} mono />
                    <MetaItem label="Plan" value={subscription.plan_name} testID="subscription-plan-name" />
                    <MetaItem label="Plan code" value={subscription.plan_code} mono testID="subscription-plan-code" />
                    <MetaItem label="Base price" value={`${(subscription.base_amount_cents / 100).toFixed(2)} ${subscription.currency}`} />
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment setup state</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Payment setup status" value={formatSubscriptionPaymentSetupStatus(subscription.payment_setup_status)} />
                    <MetaItem label="Last verified" value={formatExactTimestamp(subscription.payment_setup.last_verified_at)} />
                    <MetaItem label="Last verification result" value={subscription.payment_setup.last_verification_result || "-"} />
                    <MetaItem label="Last verification error" value={subscription.payment_setup.last_verification_error || "-"} />
                    <MetaItem label="Requested at" value={formatExactTimestamp(subscription.payment_setup_requested_at)} />
                    <MetaItem label="Activated at" value={formatExactTimestamp(subscription.activated_at)} />
                  </div>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function LoadingPanel({ label }: { label: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm">
      <div className="flex items-center gap-2">
        <LoaderCircle className="h-4 w-4 animate-spin" />
        {label}
      </div>
    </section>
  );
}

function SummaryStat({ label, value, helper, raw }: { label: string; value: string; helper: string; raw?: boolean }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 break-words text-base font-semibold text-slate-950">{raw ? value : formatReadinessStatus(value)}</p>
      <p className="mt-2 break-all text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-slate-50 px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono, testID }: { label: string; value: string; mono?: boolean; testID?: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd data-testid={testID} className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
