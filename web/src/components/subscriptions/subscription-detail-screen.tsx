"use client";

import Link from "next/link";
import { ArrowLeft, CreditCard, LoaderCircle, Send } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchSubscription, requestSubscriptionPaymentSetup, resendSubscriptionPaymentSetup } from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { describeCustomerMissingStep, formatReadinessStatus } from "@/lib/readiness";
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

  const detailQuery = useQuery({
    queryKey: ["subscription", apiBaseURL, subscriptionID],
    queryFn: () => fetchSubscription({ runtimeBaseURL: apiBaseURL, subscriptionID }),
    enabled: isAuthenticated && scope === "tenant" && subscriptionID.trim().length > 0,
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

  const subscription = detailQuery.data ?? null;
  const nextActions = subscription?.missing_steps.map(describeCustomerMissingStep) ?? [];
  const canRequestSetup = subscription?.payment_setup_status !== "ready";
  const showResend = Boolean(subscription?.payment_setup_requested_at || subscription?.payment_setup_status === "error");

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: subscription?.display_name || subscriptionID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Subscription detail is tenant-scoped. Sign in with a tenant account to inspect readiness and request payment setup."
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
            <p className="mt-3 text-sm text-slate-600">The requested subscription could not be loaded from the tenant APIs.</p>
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
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(subscription.status)}`}>
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

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1.2fr)_420px]">
              <div className="grid gap-5">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Lifecycle</p>
                      <h2 className="mt-2 text-xl font-semibold text-slate-950">What still needs action</h2>
                    </div>
                    <span className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${tone(subscription.status)}`}>
                      {formatReadinessStatus(subscription.status)}
                    </span>
                  </div>
                  <div className="mt-5 grid gap-3">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Subscription is billing-ready." />}
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Payment setup</p>
                  <p className="mt-3 text-sm leading-relaxed text-slate-600">
                    Operators request the hosted setup link. The payer completes card or bank setup outside Alpha. Once the default payment method is verified, the subscription becomes active.
                  </p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    {canRequestSetup ? (
                      <button
                        type="button"
                        data-testid="subscription-request-setup"
                        onClick={() => requestMutation.mutate()}
                        disabled={!canWrite || !csrfToken || requestMutation.isPending}
                        className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {requestMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                        Request payment setup
                      </button>
                    ) : null}
                    {canRequestSetup && showResend ? (
                      <button
                        type="button"
                        data-testid="subscription-resend-setup"
                        onClick={() => resendMutation.mutate()}
                        disabled={!canWrite || !csrfToken || resendMutation.isPending}
                        className="inline-flex h-10 items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 text-sm font-medium text-amber-700 transition hover:bg-amber-100 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {resendMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                        Resend link
                      </button>
                    ) : null}
                  </div>
                  {requestMutation.data?.checkout_url ? (
                    <a href={requestMutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                      Open latest setup link
                    </a>
                  ) : null}
                  {resendMutation.data?.checkout_url ? (
                    <a href={resendMutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                      Open resent setup link
                    </a>
                  ) : null}
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial context</p>
                  <div className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={subscription.customer_display_name} />
                    <MetaItem label="Customer ID" value={subscription.customer_external_id} mono />
                    <MetaItem label="Plan" value={subscription.plan_name} />
                    <MetaItem label="Plan code" value={subscription.plan_code} mono />
                    <MetaItem label="Base price" value={`${(subscription.base_amount_cents / 100).toFixed(2)} ${subscription.currency}`} />
                  </div>
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing state</p>
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

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-900 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
