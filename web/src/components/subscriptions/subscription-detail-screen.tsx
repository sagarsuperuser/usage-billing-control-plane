
import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { CreditCard, LoaderCircle, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchPlans, fetchSubscription, requestSubscriptionPaymentSetup, resendSubscriptionPaymentSetup, updateSubscription } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { showError, showSuccess } from "@/lib/toast";
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
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [selectedPlanID, setSelectedPlanID] = useState("");

  const detailQuery = useQuery({
    queryKey: ["subscription", apiBaseURL, subscriptionID],
    queryFn: () => fetchSubscription({ runtimeBaseURL: apiBaseURL, subscriptionID }),
    enabled: isTenantSession && subscriptionID.trim().length > 0,
  });

  const plansQuery = useQuery({
    queryKey: ["plans", apiBaseURL],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const requestMutation = useMutation({
    mutationFn: () => requestSubscriptionPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, subscriptionID }),
    onSuccess: async () => {
      showSuccess("Payment setup requested", "The customer will receive an email with the checkout link.");
      await detailQuery.refetch();
    },
    onError: (err: Error) => {
      showError("Request failed", err.message || "Could not request payment setup.");
    },
  });

  const resendMutation = useMutation({
    mutationFn: () => resendSubscriptionPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, subscriptionID }),
    onSuccess: async () => {
      showSuccess("Payment setup request resent", "A new checkout link has been sent to the customer.");
      await detailQuery.refetch();
    },
    onError: (err: Error) => {
      showError("Resend failed", err.message || "Could not resend payment setup request.");
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
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: subscription?.display_name || subscriptionID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? (
          detailQuery.isLoading ? (
            <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading subscription detail
              </div>
            </section>
          ) : detailQuery.isError || !subscription ? (
            <section className="rounded-lg border border-stone-200 bg-white shadow-sm p-5">
              <p className="text-sm font-semibold text-slate-900">Subscription not available</p>
              <p className="mt-1 text-sm text-slate-500">The requested subscription could not be loaded from the workspace APIs.</p>
            </section>
          ) : (
          <SectionErrorBoundary>
            <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm divide-y divide-stone-200">
              {/* ---- Header ---- */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900 truncate">{subscription.display_name}</h1>
                    <span className="font-mono text-xs text-slate-400">{subscription.code}</span>
                    <span data-testid="subscription-status-badge" className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${tone(subscription.status)}`}>
                      {formatReadinessStatus(subscription.status)}
                    </span>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {canRequestSetup ? (
                      <button
                        type="button"
                        data-testid="subscription-request-setup"
                        onClick={() => (showResend ? resendMutation.mutate() : requestMutation.mutate())}
                        disabled={!canWrite || !csrfToken || requestMutation.isPending || resendMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {requestMutation.isPending || resendMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : showResend ? <Send className="h-3.5 w-3.5" /> : <CreditCard className="h-3.5 w-3.5" />}
                        {setupActionLabel}
                      </button>
                    ) : null}
                    <button
                      type="button"
                      data-testid="subscription-archive"
                      onClick={() => updateMutation.mutate({ status: "archived" })}
                      disabled={!canWrite || !csrfToken || updateMutation.isPending || !canArchive}
                      className="inline-flex h-8 items-center rounded-md border border-rose-200 bg-rose-50 px-3 text-xs font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      Cancel subscription
                    </button>
                  </div>
                </div>
              </div>

              {/* ---- Details ---- */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Plan</dt>
                    <dd data-testid="subscription-plan-name" className="mt-0.5 text-sm text-slate-700">{subscription.plan_name}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Plan code</dt>
                    <dd data-testid="subscription-plan-code" className="mt-0.5 text-sm font-mono text-slate-700">{subscription.plan_code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Customer</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{subscription.customer_display_name}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Customer ID</dt>
                    <dd className="mt-0.5 text-sm font-mono text-slate-700">{subscription.customer_external_id}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Base price</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{(subscription.base_amount_cents / 100).toFixed(2)} {subscription.currency}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Billing interval</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{subscription.billing_interval}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Payment setup</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatSubscriptionPaymentSetupStatus(subscription.payment_setup_status)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Payment method</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{subscription.default_payment_method_verified ? "Verified" : "Waiting on payer"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Activated</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(subscription.activated_at)}</dd>
                  </div>
                </dl>
              </div>

              {/* ---- Payment setup state ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-3">Payment setup state</p>
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Last verified</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(subscription.payment_setup.last_verified_at)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last result</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{subscription.payment_setup.last_verification_result || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Last error</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{subscription.payment_setup.last_verification_error || "-"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Requested at</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{formatExactTimestamp(subscription.payment_setup_requested_at)}</dd>
                  </div>
                </dl>
                {latestSetupCheckoutURL ? (
                  <div className="mt-3">
                    <a href={latestSetupCheckoutURL} target="_blank" rel="noreferrer" className="inline-flex h-8 items-center rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                      Open latest setup link
                    </a>
                  </div>
                ) : null}
              </div>

              {/* ---- Lifecycle ---- */}
              {nextActions.length > 0 ? (
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-slate-400 mb-3">Open actions</p>
                  <div className="grid gap-2">
                    {nextActions.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-sm text-slate-700">
                        <span className="mt-0.5 inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-amber-100 text-[10px] font-semibold text-amber-700">!</span>
                        {item}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {/* ---- Change plan ---- */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-slate-400 mb-3">Change plan</p>
                <div className="flex flex-wrap items-end gap-3">
                  <label className="grid gap-1 text-sm min-w-[200px] flex-1">
                    <span className="text-xs text-slate-400">Target plan</span>
                    <select
                      id="subscription-plan-select"
                      data-testid="subscription-plan-select"
                      value={selectedPlanIDValue}
                      onChange={(event) => setSelectedPlanID(event.target.value)}
                      disabled={!canWrite || !csrfToken || updateMutation.isPending || plansQuery.isLoading || subscription.status === "archived"}
                      className="h-9 rounded-md border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 disabled:cursor-not-allowed disabled:bg-slate-50"
                    >
                      {activePlans.map((plan) => (
                        <option key={plan.id} value={plan.id}>
                          {plan.name} ({plan.code})
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    type="button"
                    data-testid="subscription-change-plan"
                    onClick={() => updateMutation.mutate({ plan_id: selectedPlanIDValue })}
                    disabled={!canWrite || !csrfToken || updateMutation.isPending || !canChangePlan}
                    className="inline-flex h-9 items-center gap-1.5 rounded-md border border-slate-900 bg-slate-900 px-4 text-xs font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {updateMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    Apply plan change
                  </button>
                </div>
                {selectedPlan ? <p className="mt-2 text-xs text-slate-500">Selected plan code: {selectedPlan.code}</p> : null}
                {updateMutation.isError ? (
                  <p className="mt-2 text-sm text-rose-700">{updateMutation.error instanceof Error ? updateMutation.error.message : "Subscription update failed."}</p>
                ) : null}
              </div>
            </div>
          </SectionErrorBoundary>
          )
        ) : null}
      </main>
    </div>
  );
}
