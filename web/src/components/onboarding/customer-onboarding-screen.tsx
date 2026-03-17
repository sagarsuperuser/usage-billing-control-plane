"use client";

import Link from "next/link";
import { CreditCard, ArrowRight, LoaderCircle, UserRoundPlus } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { onboardCustomer } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { type CustomerOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function CustomerOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, role, scope } = useUISession();

  const [externalID, setExternalID] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [legalName, setLegalName] = useState("");
  const [addressLine1, setAddressLine1] = useState("");
  const [city, setCity] = useState("");
  const [postalCode, setPostalCode] = useState("");
  const [country, setCountry] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [providerCode, setProviderCode] = useState("");
  const [startPaymentSetup, setStartPaymentSetup] = useState(true);
  const [paymentMethodType, setPaymentMethodType] = useState("card");
  const [flash, setFlash] = useState<string | null>(null);
  const [result, setResult] = useState<CustomerOnboardingResult | null>(null);

  const onboardingMutation = useMutation({
    mutationFn: () =>
      onboardCustomer({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          external_id: externalID.trim(),
          display_name: displayName.trim(),
          email: email.trim(),
          start_payment_setup: startPaymentSetup,
          payment_method_type: paymentMethodType.trim() || undefined,
          billing_profile: {
            legal_name: legalName.trim(),
            email: email.trim(),
            billing_address_line1: addressLine1.trim(),
            billing_city: city.trim(),
            billing_postal_code: postalCode.trim(),
            billing_country: country.trim(),
            currency: currency.trim(),
            provider_code: providerCode.trim(),
          },
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      setExternalID(payload.customer.external_id);
      setDisplayName(payload.customer.display_name);
      setEmail(payload.customer.email ?? "");
      setFlash(
        payload.payment_setup_started
          ? `Customer ${payload.customer.external_id} created and payment setup is ready to continue.`
          : `Customer ${payload.customer.external_id} created and readiness has been refreshed.`
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.customer.external_id] }),
      ]);
    },
  });

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#164e63_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1200px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Customer Setup</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Customer Setup</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Create a billable customer, apply the billing profile, and optionally start payment setup. Ongoing review and recovery now live in dedicated customer pages.
              </p>
            </div>
            <div className="flex flex-wrap gap-3">
              <Link
                href="/customers"
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Open customers
              </Link>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="This screen drives tenant-scoped customer and payment APIs. Sign in with a tenant reader, writer, or admin API key."
          />
        ) : null}
        {isAuthenticated && scope === "tenant" && !canWrite ? (
          <ScopeNotice
            title="Read-only session"
            body={`Current session role ${role ?? "reader"} can inspect customer detail pages, but a writer or admin key is required to run setup.`}
          />
        ) : null}

        {flash ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100">
            {flash}
          </section>
        ) : null}

        <div className="grid gap-6 2xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="min-w-0 rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Guided setup</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Create customer</h2>
                <p className="mt-2 max-w-2xl text-sm text-slate-300">
                  This page is only for creating or reconciling a customer. Browse customer health and run recovery actions from the dedicated directory and detail pages.
                </p>
              </div>
              <span className="inline-flex rounded-xl border border-cyan-400/40 bg-cyan-500/10 p-3 text-cyan-100">
                <UserRoundPlus className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-6 grid gap-3 lg:grid-cols-3">
              <StepCard index="1" title="Create the customer" body="Capture the customer identity you want teams to search for and manage later." />
              <StepCard index="2" title="Complete billing profile" body="Add the legal and billing details Alpha needs before it can sync billing state." />
              <StepCard index="3" title="Start payment setup" body="Launch payment setup now, or leave it for a controlled follow-up after the customer record is ready." />
            </div>

            <div className="mt-6 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 1</p>
              <h3 className="mt-2 text-lg font-semibold text-white">Customer identity</h3>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <InputField label="Customer external ID" value={externalID} onChange={setExternalID} placeholder="cust_acme_primary" />
                <InputField label="Display name" value={displayName} onChange={setDisplayName} placeholder="Acme Primary Customer" />
                <InputField label="Billing email" value={email} onChange={setEmail} placeholder="billing@acme.test" />
              </div>
            </div>

            <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 2</p>
              <h3 className="mt-2 text-lg font-semibold text-white">Billing profile</h3>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                <InputField label="Legal name" value={legalName} onChange={setLegalName} placeholder="Acme Primary Customer LLC" />
                <InputField label="Billing address line 1" value={addressLine1} onChange={setAddressLine1} placeholder="1 Billing Street" />
                <InputField label="Billing city" value={city} onChange={setCity} placeholder="Bengaluru" />
                <InputField label="Billing postal code" value={postalCode} onChange={setPostalCode} placeholder="560001" />
                <InputField label="Billing country" value={country} onChange={setCountry} placeholder="IN" />
                <InputField label="Currency" value={currency} onChange={setCurrency} placeholder="USD" />
              </div>
            </div>

            <details className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <summary className="cursor-pointer list-none">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Step 3</p>
                    <h3 className="mt-2 text-lg font-semibold text-white">Payment setup options</h3>
                  </div>
                  <span className="text-xs uppercase tracking-[0.14em] text-slate-400">Expand</span>
                </div>
              </summary>
              <div className="mt-4 grid gap-4 md:grid-cols-[1.05fr_0.95fr]">
                <div className="grid gap-3">
                  <InputField label="Billing connection code" value={providerCode} onChange={setProviderCode} placeholder="stripe_default" />
                  <InputField label="Payment method type" value={paymentMethodType} onChange={setPaymentMethodType} placeholder="card" />
                </div>
                <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
                  <p className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Advanced controls</p>
                  <label className="mt-3 flex items-center gap-2 text-sm text-slate-200">
                    <input
                      type="checkbox"
                      checked={startPaymentSetup}
                      onChange={(event) => setStartPaymentSetup(event.target.checked)}
                      className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                    />
                    Start payment setup now
                  </label>
                </div>
              </div>
            </details>

            <div className="mt-4 rounded-2xl border border-cyan-400/20 bg-cyan-500/5 p-4">
              <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Before you run</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={externalID.trim().length > 0} text="Customer external ID is set" />
                <ChecklistLine done={displayName.trim().length > 0} text="Display name is set" />
                <ChecklistLine done={email.trim().length > 0} text="Billing email is set" />
                <ChecklistLine done={providerCode.trim().length > 0} text="Billing connection code is set" />
                <ChecklistLine done={currency.trim().length > 0} text="Currency is set" />
                <ChecklistLine done={startPaymentSetup} text="Payment setup will start after customer sync" />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardingMutation.mutate();
                }}
                disabled={!canWrite || !csrfToken || onboardingMutation.isPending || !externalID.trim() || !displayName.trim()}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {onboardingMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                Run customer setup
              </button>
              <button
                type="button"
                onClick={() => {
                  setExternalID("");
                  setDisplayName("");
                  setEmail("");
                  setLegalName("");
                  setAddressLine1("");
                  setCity("");
                  setPostalCode("");
                  setCountry("");
                  setCurrency("USD");
                  setProviderCode("");
                  setPaymentMethodType("card");
                  setStartPaymentSetup(true);
                  setResult(null);
                  setFlash(null);
                }}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Reset form
              </button>
            </div>

            {result?.checkout_url ? (
              <div className="mt-6 rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-4 text-sm text-emerald-100">
                <p className="font-semibold text-emerald-50">Payment setup link</p>
                <a
                  href={result.checkout_url}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-2 block break-all rounded-xl border border-white/10 bg-slate-950/60 px-3 py-3 font-mono text-xs text-emerald-50 hover:bg-slate-950/80"
                >
                  {result.checkout_url}
                </a>
              </div>
            ) : null}
          </section>

          <aside className="flex flex-col gap-4">
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">After setup</p>
              <h2 className="mt-2 text-xl font-semibold text-white">Use dedicated customer pages</h2>
              <div className="mt-4 grid gap-3">
                <ChecklistLine done text="Create or reconcile the customer here" />
                <ChecklistLine done text="Open the customer detail page to review readiness" />
                <ChecklistLine done text="Use the customer directory to browse the tenant customer base" />
              </div>
            </section>

            {result?.customer ? (
              <section className="rounded-3xl border border-cyan-400/20 bg-cyan-500/5 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Customer created</p>
                <h2 className="mt-2 break-words text-xl font-semibold text-white">{result.customer.display_name}</h2>
                <p className="mt-1 break-all font-mono text-xs text-slate-400">{result.customer.external_id}</p>
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <SummaryStat label="Customer" value={result.readiness.customer_active ? "ready" : "pending"} helper={result.readiness.customer_active ? "Active" : "Needs attention"} />
                  <SummaryStat label="Overall" value={result.readiness.status} helper={`${result.readiness.missing_steps.length} checklist items remain`} />
                </div>
                <div className="mt-5 flex flex-col gap-3">
                  <Link
                    href={`/customers/${encodeURIComponent(result.customer.external_id)}`}
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
                  >
                    View customer detail
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link
                    href="/customers"
                    className="inline-flex h-11 items-center justify-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                  >
                    Open customer directory
                  </Link>
                </div>
              </section>
            ) : (
              <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">What changes now</p>
                <div className="mt-4 space-y-3 text-sm text-slate-300">
                  <p>Customer setup is now a focused form instead of a combined setup and diagnostics screen.</p>
                  <p>Use <span className="font-semibold text-white">Customers</span> to browse and inspect readiness across the tenant.</p>
                  <p>Use customer detail pages for billing sync status, payment readiness, and recovery actions.</p>
                </div>
              </section>
            )}
          </aside>
        </div>
      </main>
    </div>
  );
}

function StepCard({ index, title, body }: { index: string; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/45 p-4">
      <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Step {index}</p>
      <p className="mt-2 text-sm font-semibold text-white">{title}</p>
      <p className="mt-2 text-sm text-slate-300">{body}</p>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="min-w-0 rounded-2xl border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-[11px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className="mt-2 break-words text-base font-semibold leading-tight text-white">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{helper}</p>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-300">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </label>
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
