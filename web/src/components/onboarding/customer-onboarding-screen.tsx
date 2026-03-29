"use client";

import Link from "next/link";
import { ArrowRight, CreditCard, LoaderCircle, UserRoundPlus } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { onboardCustomer } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { type CustomerOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function CustomerOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, role, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

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
          : `Customer ${payload.customer.external_id} created and readiness has been refreshed.`,
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.customer.external_id] }),
      ]);
    },
  });

  const canSubmit = Boolean(canWrite && csrfToken && externalID.trim() && displayName.trim());

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { href: "/customers", label: "Customers" }, { label: "New" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer setup</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create customer</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">Create the customer, apply the billing profile, and optionally start payment setup.</p>
            </div>
            <Link href="/customers" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open customers</Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="This screen drives workspace-scoped customer and payment APIs. Sign in with a workspace account."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}
        {isTenantSession && !canWrite ? (
          <ScopeNotice
            title="Read-only session"
            body={`Current session role ${role ?? "reader"} can inspect customer detail pages, but a writer or admin account is required to run setup.`}
            actionHref="/customers"
            actionLabel="Open customer directory"
          />
        ) : null}

        {isTenantSession && flash ? <section className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{flash}</section> : null}

        {isTenantSession ? <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
          <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
                <h2 className="mt-2 text-xl font-semibold text-slate-950">Customer onboarding</h2>
                <p className="mt-2 max-w-2xl text-sm text-slate-600">Create the customer record, apply the billing profile, and decide whether to start payment setup now.</p>
              </div>
              <span className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                <UserRoundPlus className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-5 grid gap-3 lg:grid-cols-3">
              <OperatorCard title="Required now" body="Customer ID, display name, and the billing record the workspace will operate on." />
              <OperatorCard title="Billing context" body="Legal entity, address, currency, and the connection code used for collection." />
              <OperatorCard title="After submit" body="Open the customer record for readiness, payment recovery, and hosted link follow-up." />
            </div>

            <div className="mt-5 grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Customer record</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Identity and billing contact</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <InputField label="Customer external ID" value={externalID} onChange={setExternalID} placeholder="cust_acme_primary" />
                  <InputField label="Display name" value={displayName} onChange={setDisplayName} placeholder="Acme Primary Customer" />
                  <InputField label="Billing email" value={email} onChange={setEmail} placeholder="billing@acme.test" />
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Billing profile</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Commercial and address detail</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <InputField label="Legal name" value={legalName} onChange={setLegalName} placeholder="Acme Primary Customer LLC" />
                  <InputField label="Billing address line 1" value={addressLine1} onChange={setAddressLine1} placeholder="1 Billing Street" />
                  <InputField label="Billing city" value={city} onChange={setCity} placeholder="Bengaluru" />
                  <InputField label="Billing postal code" value={postalCode} onChange={setPostalCode} placeholder="560001" />
                  <InputField label="Billing country" value={country} onChange={setCountry} placeholder="IN" />
                  <InputField label="Currency" value={currency} onChange={setCurrency} placeholder="USD" />
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Payment setup</p>
                <h3 className="mt-2 text-lg font-semibold text-slate-950">Payment setup</h3>
                <div className="mt-4 grid gap-4 md:grid-cols-[1.05fr_0.95fr]">
                  <div className="grid gap-4">
                    <InputField label="Billing connection code" value={providerCode} onChange={setProviderCode} placeholder="stripe_default" />
                    <InputField label="Payment method type" value={paymentMethodType} onChange={setPaymentMethodType} placeholder="card" />
                  </div>
                  <div className="rounded-xl border border-slate-200 bg-white p-4">
                    <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Submission mode</p>
                    <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                      <input type="checkbox" checked={startPaymentSetup} onChange={(event) => setStartPaymentSetup(event.target.checked)} className="h-4 w-4 rounded border-slate-300" />
                      Start payment setup now
                    </label>
                    <p className="mt-3 text-xs leading-relaxed text-slate-500">Leave this enabled when the payer should receive a hosted setup link immediately after the customer record is created.</p>
                  </div>
                </div>
              </section>
            </div>

            <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
              <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={externalID.trim().length > 0} text="Customer ID is set" />
                <ChecklistLine done={displayName.trim().length > 0} text="Display name is set" />
                <ChecklistLine done={email.trim().length > 0} text="Billing email is set" />
                <ChecklistLine done={providerCode.trim().length > 0} text="Connection code is set" />
                <ChecklistLine done={currency.trim().length > 0} text="Currency is set" />
                <ChecklistLine done={startPaymentSetup} text="Payment setup will start" />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardingMutation.mutate();
                }}
                disabled={!canSubmit || onboardingMutation.isPending}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
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
                className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
              >
                Reset form
              </button>
            </div>

            {result?.checkout_url ? (
              <div className="mt-6 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
                <p className="font-semibold text-emerald-800">Payment link</p>
                <a href={result.checkout_url} target="_blank" rel="noreferrer" className="mt-2 block break-all rounded-lg border border-emerald-200 bg-white px-3 py-3 font-mono text-xs text-emerald-800 hover:bg-emerald-100">
                  {result.checkout_url}
                </a>
              </div>
            ) : null}
          </section>

          <aside className="min-w-0 grid gap-5 self-start">
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Operator posture</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">After submit</h2>
              <div className="mt-4 grid gap-2">
                <ChecklistLine done text="Use this screen for customer creation only" />
                <ChecklistLine done text="Open customer detail for readiness and recovery" />
                <ChecklistLine done text="Use the customer directory for inventory review" />
              </div>
            </section>

            {result?.customer ? (
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer created</p>
                <h2 className="mt-2 break-words text-xl font-semibold text-slate-950">{result.customer.display_name}</h2>
                <p className="mt-1 break-all font-mono text-xs text-slate-500">{result.customer.external_id}</p>
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <SummaryStat label="Customer" value={result.readiness.customer_active ? "ready" : "pending"} helper={result.readiness.customer_active ? "Active" : "Needs attention"} />
                  <SummaryStat label="Overall" value={result.readiness.status} helper={`${normalizeMissingSteps(result.readiness.missing_steps).length} checklist items remain`} />
                </div>
                <div className="mt-5 flex flex-col gap-3">
                  <Link href={`/customers/${encodeURIComponent(result.customer.external_id)}`} className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                    View customer detail
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link href="/customers" className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open customer directory</Link>
                </div>
              </section>
            ) : (
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Expected output</p>
                <div className="mt-4 grid gap-3 text-sm text-slate-600">
                  <p>Submit once the operator has enough billing detail to open a usable customer record.</p>
                  <p>The created record becomes the place for payment setup follow-up, recovery actions, and collection diagnosis.</p>
                </div>
              </section>
            )}
          </aside>
        </div> : null}
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm text-slate-600">{body}</p>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-white px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}
