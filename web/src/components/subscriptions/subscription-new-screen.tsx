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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: "New" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Subscription</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create subscription</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Pick the customer, choose the plan, and decide whether to request payment setup immediately.</p>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Workspace session required" body="Subscriptions are workspace-scoped. Sign in with a workspace account to create them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        {mutation.isSuccess ? (
          <section className="rounded-2xl border border-emerald-200 bg-emerald-50 p-6 shadow-sm">
            <div className="flex items-start gap-3">
              <CheckCircle2 className="mt-0.5 h-5 w-5 text-emerald-700" />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-emerald-800">Subscription created</p>
                <p className="mt-2 text-sm text-emerald-700">{mutation.data.subscription.display_name} is now {formatReadinessStatus(mutation.data.subscription.status)}.</p>
                {mutation.data.checkout_url ? (
                  <a href={mutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-lg border border-emerald-200 bg-white px-4 text-sm font-medium text-emerald-700 transition hover:bg-emerald-100">
                    Open payment setup link
                  </a>
                ) : null}
                <div className="mt-4 flex flex-wrap gap-3">
                  <Link href={`/subscriptions/${encodeURIComponent(mutation.data.subscription.id)}`} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                    Open subscription
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link href="/subscriptions" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                    Back to subscriptions
                  </Link>
                </div>
              </div>
            </div>
          </section>
        ) : null}

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <form
            className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm"
            onSubmit={(event) => {
              event.preventDefault();
              if (!canSubmit || mutation.isPending) return;
              mutation.mutate();
            }}
          >
            <div className="grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <div className="grid gap-4 md:grid-cols-2">
                  <Field label="Subscription name" hint="Optional. Alpha can generate a default.">
                    <input data-testid="subscription-name" value={displayName} onChange={(event) => setDisplayName(event.target.value)} placeholder="Acme Growth" className={inputClass} />
                  </Field>
                  <Field label="Code" hint="Optional stable internal reference.">
                    <input data-testid="subscription-code" value={code} onChange={(event) => setCode(event.target.value)} placeholder="acme_growth" className={inputClass} />
                  </Field>
                  <Field label="Customer" hint="The account that is subscribing.">
                    <select data-testid="subscription-customer" value={customerExternalID} onChange={(event) => setCustomerExternalID(event.target.value)} className={inputClass}>
                      <option value="">Select customer</option>
                      {customers.map((customer) => (
                        <option key={customer.id} value={customer.external_id}>{customer.display_name} ({customer.external_id})</option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Plan" hint="The commercial package this customer is signing up for.">
                    <select data-testid="subscription-plan" value={planID} onChange={(event) => setPlanID(event.target.value)} className={inputClass}>
                      <option value="">Select plan</option>
                      {plans.map((plan) => (
                        <option key={plan.id} value={plan.id}>{plan.name} ({plan.code})</option>
                      ))}
                    </select>
                  </Field>
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <label className="flex items-start gap-3 text-sm text-slate-700">
                  <input data-testid="subscription-request-payment-setup" type="checkbox" checked={requestPaymentSetup} onChange={(event) => setRequestPaymentSetup(event.target.checked)} className="mt-1 h-4 w-4 rounded border-slate-300" />
                  <span>
                    <span className="font-semibold text-slate-950">Request payment setup now</span>
                    <span className="mt-1 block text-slate-600">Alpha will generate a hosted payer link. The operator initiates the step; the payer completes card or bank setup.</span>
                  </span>
                </label>
                <div className="mt-4 max-w-sm">
                  <Field label="Payment method type" hint="Defaults to card.">
                    <input data-testid="subscription-payment-method-type" value={paymentMethodType} onChange={(event) => setPaymentMethodType(event.target.value)} placeholder="card" className={inputClass} />
                  </Field>
                </div>
              </section>

              {mutation.isError ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{mutation.error.message}</p> : null}

              <div className="flex flex-wrap gap-3">
                <button type="submit" data-testid="subscription-submit" disabled={!canSubmit || mutation.isPending} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create subscription
                </button>
                <Link href="/subscriptions" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
              </div>
            </div>
          </form>

          <aside className="grid gap-5 self-start">
            <InfoCard title="Before you start" body="You need at least one customer and one plan. The Wave 1 flow stays intentionally simple." />
            <InfoCard title="Customer-owned payment setup" body="Operators should not collect card details. Use the generated hosted link so the payer completes billing setup directly." />
            <InfoCard title="Current inventory" body={`${customers.length} customers available · ${plans.length} plans available`} />
          </aside>
        </div>
      </main>
    </div>
  );
}

function Field({ label, hint, children }: { label: string; hint: string; children: ReactNode }) {
  return (
    <label className="grid gap-2">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      {children}
      <span className="text-xs text-slate-500">{hint}</span>
    </label>
  );
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

const inputClass = "h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2";
