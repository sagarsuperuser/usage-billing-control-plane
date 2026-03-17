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
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "pending":
    case "pending_payment_setup":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    case "action_required":
    case "error":
      return "border-rose-400/40 bg-rose-500/10 text-rose-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
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
  const showResend = subscription?.payment_setup_requested_at || subscription?.payment_setup_status === "error";

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1240px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
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
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 text-sm text-slate-300 backdrop-blur-xl">
            <div className="flex items-center gap-2">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading subscription detail
            </div>
          </section>
        ) : detailQuery.isError || !subscription ? (
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Subscription detail</p>
            <h1 className="mt-2 text-3xl font-semibold text-white">Subscription not available</h1>
            <p className="mt-3 text-sm text-slate-300">The requested subscription could not be loaded from the tenant APIs.</p>
            <Link href="/subscriptions" className="mt-5 inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
              <ArrowLeft className="h-4 w-4" />
              Back to subscriptions
            </Link>
          </section>
        ) : (
          <>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Subscription detail</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-white md:text-4xl">{subscription.display_name}</h1>
                  <p className="mt-2 break-all font-mono text-sm text-slate-400">{subscription.code}</p>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <span className={`rounded-full px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] ${tone(subscription.status)}`}>
                    {formatReadinessStatus(subscription.status)}
                  </span>
                  <Link href="/subscriptions" className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
                    <ArrowLeft className="h-4 w-4" />
                    Back to subscriptions
                  </Link>
                </div>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <SummaryStat label="Subscription" value={subscription.status} helper={subscription.plan_name} />
              <SummaryStat
                label="Payment setup"
                value={formatSubscriptionPaymentSetupStatus(subscription.payment_setup_status)}
                helper={subscription.default_payment_method_verified ? "Verified" : "Waiting on payer"}
                raw
              />
              <SummaryStat label="Customer" value={subscription.customer_display_name} helper={subscription.customer_external_id} raw />
              <SummaryStat label="Billing" value={`${(subscription.base_amount_cents / 100).toFixed(2)} ${subscription.currency}`} helper={subscription.billing_interval} raw />
            </section>

            <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Lifecycle</p>
                <h2 className="mt-2 text-2xl font-semibold text-white">What still needs action</h2>
                <div className="mt-5 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                  <p className="text-sm font-semibold text-white">Next actions</p>
                  <div className="mt-3 grid gap-2">
                    {nextActions.length > 0 ? nextActions.map((item) => <ChecklistLine key={item} done={false} text={item} />) : <ChecklistLine done text="Subscription is billing-ready." />}
                  </div>
                </div>

                <div className="mt-4 rounded-2xl border border-white/10 bg-slate-900/60 p-4 text-sm text-slate-200">
                  <p className="font-semibold text-white">Payment setup</p>
                  <p className="mt-2 text-slate-300">
                    Operators request the hosted setup link. The payer completes card or bank setup outside Alpha. Once the default payment method is verified, the subscription becomes active.
                  </p>
                  <div className="mt-4 flex flex-wrap gap-3">
                    {canRequestSetup ? (
                      <button
                        type="button"
                        data-testid="subscription-request-setup"
                        onClick={() => requestMutation.mutate()}
                        disabled={!canWrite || !csrfToken || requestMutation.isPending}
                        className="inline-flex h-10 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
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
                        className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-400/40 bg-amber-500/10 px-3 text-sm text-amber-100 transition hover:bg-amber-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {resendMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                        Resend link
                      </button>
                    ) : null}
                  </div>
                  {requestMutation.data?.checkout_url ? (
                    <a href={requestMutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20">
                      Open latest setup link
                    </a>
                  ) : null}
                  {resendMutation.data?.checkout_url ? (
                    <a href={resendMutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-xl border border-amber-400/40 bg-amber-500/10 px-3 text-sm text-amber-100 transition hover:bg-amber-500/20">
                      Open resent setup link
                    </a>
                  ) : null}
                </div>
              </section>

              <aside className="flex flex-col gap-4">
                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Commercial context</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Customer" value={subscription.customer_display_name} />
                    <MetaItem label="Customer ID" value={subscription.customer_external_id} mono />
                    <MetaItem label="Plan" value={subscription.plan_name} />
                    <MetaItem label="Plan code" value={subscription.plan_code} mono />
                    <MetaItem label="Base price" value={`${(subscription.base_amount_cents / 100).toFixed(2)} ${subscription.currency}`} />
                  </dl>
                </section>

                <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                  <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Billing state</p>
                  <dl className="mt-4 grid gap-3">
                    <MetaItem label="Payment setup status" value={formatSubscriptionPaymentSetupStatus(subscription.payment_setup_status)} />
                    <MetaItem label="Last verified" value={formatExactTimestamp(subscription.payment_setup.last_verified_at)} />
                    <MetaItem label="Last verification result" value={subscription.payment_setup.last_verification_result || "-"} />
                    <MetaItem label="Last verification error" value={subscription.payment_setup.last_verification_error || "-"} />
                    <MetaItem label="Requested at" value={formatExactTimestamp(subscription.payment_setup_requested_at)} />
                    <MetaItem label="Activated at" value={formatExactTimestamp(subscription.activated_at)} />
                  </dl>
                </section>
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function SummaryStat({ label, value, helper, raw }: { label: string; value: string; helper: string; raw?: boolean }) {
  return (
    <div className="min-w-0 rounded-2xl border border-white/10 bg-slate-900/70 px-4 py-4 backdrop-blur-xl">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{raw ? value : formatReadinessStatus(value)}</p>
      <p className="mt-2 break-all text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/10 bg-white/5 px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-500/20 text-emerald-100" : "bg-amber-500/20 text-amber-100"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-200">{text}</p>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-3">
      <dt className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</dt>
      <dd className={`mt-2 break-all text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
