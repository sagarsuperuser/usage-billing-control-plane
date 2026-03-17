"use client";

import Link from "next/link";
import { ArrowRight, CheckCircle2, LoaderCircle } from "lucide-react";
import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createSubscription, fetchCustomers, fetchPlans } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { useUISession } from "@/hooks/use-ui-session";

export function SubscriptionNewScreen() {
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [displayName, setDisplayName] = useState("");
  const [code, setCode] = useState("");
  const [customerExternalID, setCustomerExternalID] = useState("");
  const [planID, setPlanID] = useState("");
  const [requestPaymentSetup, setRequestPaymentSetup] = useState(true);
  const [paymentMethodType, setPaymentMethodType] = useState("card");

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, "subscriptions-new"],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isAuthenticated && scope === "tenant",
  });
  const plansQuery = useQuery({
    queryKey: ["plans", apiBaseURL, "subscriptions-new"],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const customers = useMemo(() => customersQuery.data ?? [], [customersQuery.data]);
  const plans = useMemo(() => plansQuery.data ?? [], [plansQuery.data]);

  const mutation = useMutation({
    mutationFn: () =>
      createSubscription({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          code,
          display_name: displayName,
          customer_external_id: customerExternalID,
          plan_id: planID,
          request_payment_setup: requestPaymentSetup,
          payment_method_type: paymentMethodType,
        },
      }),
  });

  const canSubmit = Boolean(csrfToken && customerExternalID && planID);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#14532d_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1120px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: "New" }]} />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Subscriptions</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Create subscription</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
            Keep the first version simple: pick the customer, choose the plan, and decide whether to request payment setup immediately.
          </p>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Subscriptions are tenant-scoped. Sign in with a tenant account to create them."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {mutation.isSuccess ? (
          <section className="rounded-3xl border border-emerald-400/30 bg-emerald-500/10 p-6 backdrop-blur-xl">
            <div className="flex items-start gap-3">
              <CheckCircle2 className="mt-0.5 h-5 w-5 text-emerald-200" />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-emerald-100">Subscription created</p>
                <p className="mt-2 text-sm text-emerald-50/90">
                  {mutation.data.subscription.display_name} is now {formatReadinessStatus(mutation.data.subscription.status)}.
                </p>
                {mutation.data.checkout_url ? (
                  <a
                    href={mutation.data.checkout_url}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-4 inline-flex h-11 items-center rounded-xl border border-emerald-200/40 bg-emerald-100/10 px-4 text-sm font-medium text-emerald-50 transition hover:bg-emerald-100/20"
                  >
                    Open payment setup link
                  </a>
                ) : null}
                <div className="mt-4 flex flex-wrap gap-3">
                  <Link href={`/subscriptions/${encodeURIComponent(mutation.data.subscription.id)}`} className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/15 bg-white/10 px-4 text-sm text-white transition hover:bg-white/15">
                    Open subscription
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link href="/subscriptions" className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
                    Back to subscriptions
                  </Link>
                </div>
              </div>
            </div>
          </section>
        ) : null}

        <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
          <form
            className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl"
            onSubmit={(event) => {
              event.preventDefault();
              if (!canSubmit || mutation.isPending) return;
              mutation.mutate();
            }}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Subscription name" hint="Optional. Alpha can generate a good default.">
                <input data-testid="subscription-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="Acme Growth" className={inputClass} />
              </Field>
              <Field label="Code" hint="Optional. Used for stable internal reference.">
                <input data-testid="subscription-code" value={code} onChange={(event) => setCode(event.target.value)} placeholder="acme_growth" className={inputClass} />
              </Field>
              <Field label="Customer" hint="The account that is subscribing.">
                <select data-testid="subscription-customer" value={customerExternalID} onChange={(event) => setCustomerExternalID(event.target.value)} className={inputClass}>
                  <option value="">Select customer</option>
                  {customers.map((customer) => (
                    <option key={customer.id} value={customer.external_id}>
                      {customer.display_name} ({customer.external_id})
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Plan" hint="The commercial package this customer is signing up for.">
                <select data-testid="subscription-plan" value={planID} onChange={(event) => setPlanID(event.target.value)} className={inputClass}>
                  <option value="">Select plan</option>
                  {plans.map((plan) => (
                    <option key={plan.id} value={plan.id}>
                      {plan.name} ({plan.code})
                    </option>
                  ))}
                </select>
              </Field>
            </div>

            <div className="mt-6 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
              <div className="flex flex-col gap-3">
                <label className="flex items-start gap-3 text-sm text-slate-200">
                  <input
                    data-testid="subscription-request-payment-setup"
                    type="checkbox"
                    checked={requestPaymentSetup}
                    onChange={(event) => setRequestPaymentSetup(event.target.checked)}
                    className="mt-1 h-4 w-4 rounded border-white/20 bg-slate-950 text-cyan-300"
                  />
                  <span>
                    <span className="font-semibold text-white">Request payment setup now</span>
                    <span className="mt-1 block text-slate-400">
                      Alpha will generate a hosted payer link. The operator initiates the step; the payer completes card or bank setup.
                    </span>
                  </span>
                </label>
                <Field label="Payment method type" hint="Defaults to card." compact>
                  <input data-testid="subscription-payment-method-type" value={paymentMethodType} onChange={(event) => setPaymentMethodType(event.target.value)} placeholder="card" className={inputClass} />
                </Field>
              </div>
            </div>

            {mutation.isError ? <p className="mt-4 text-sm text-rose-200">{mutation.error.message}</p> : null}

            <div className="mt-6 flex flex-wrap gap-3">
              <button
                type="submit"
                data-testid="subscription-submit"
                disabled={!canSubmit || mutation.isPending}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                Create subscription
              </button>
              <Link href="/subscriptions" className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10">
                Cancel
              </Link>
            </div>
          </form>

          <aside className="flex flex-col gap-4">
            <InfoCard title="Before you start" body="You need at least one customer and one plan. The subscription flow stays intentionally simple in Wave 1." />
            <InfoCard title="Customer-owned payment setup" body="Operators should not be collecting card details. Use the generated hosted link so the payer completes billing setup directly." />
            <InfoCard title="Current inventory" body={`${customers.length} customers available · ${plans.length} plans available`} />
          </aside>
        </section>
      </main>
    </div>
  );
}

function Field({ label, hint, compact, children }: { label: string; hint: string; compact?: boolean; children: ReactNode }) {
  return (
    <label className={compact ? "grid gap-2" : "grid gap-2"}>
      <span className="text-xs uppercase tracking-[0.16em] text-slate-400">{label}</span>
      {children}
      <span className="text-xs text-slate-500">{hint}</span>
    </label>
  );
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-5 backdrop-blur-xl">
      <p className="text-sm font-semibold text-white">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-300">{body}</p>
    </section>
  );
}

const inputClass =
  "h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2";
