"use client";

import Link from "next/link";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createAddOn } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingAddOnNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [billingInterval, setBillingInterval] = useState("monthly");
  const [status, setStatus] = useState("draft");
  const [amount, setAmount] = useState("15");
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () =>
      createAddOn({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name,
          code,
          description,
          currency,
          billing_interval: billingInterval,
          status,
          amount_cents: Math.round(Number(amount || 0) * 100),
        },
      }),
    onSuccess: (item) => router.push(`/pricing/add-ons/${encodeURIComponent(item.id)}`),
    onError: (err: Error) => setError(err.message),
  });

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/add-ons", label: "Add-ons" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Add-ons are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing add-on</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create add-on</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Use add-ons for fixed recurring extras that can be attached to multiple plans.</p>
        </section>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <h2 className="text-lg font-semibold text-slate-950">Commercial basics</h2>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <Field label="Add-on name" value={name} onChange={setName} placeholder="Priority support" testID="pricing-addon-name" />
                  <Field label="Add-on code" value={code} onChange={setCode} placeholder="priority_support" testID="pricing-addon-code" />
                  <Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-addon-currency" />
                  <Field label="Recurring amount" value={amount} onChange={setAmount} placeholder="15" testID="pricing-addon-amount" />
                  <SelectField label="Billing interval" value={billingInterval} onChange={setBillingInterval} options={["monthly", "yearly"]} />
                  <SelectField label="Status" value={status} onChange={setStatus} options={["draft", "active", "archived"]} />
                  <div className="md:col-span-2">
                    <label className="grid gap-2 text-sm text-slate-700">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Description</span>
                      <textarea data-testid="pricing-addon-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Faster response times and operator escalation support." className="min-h-[120px] rounded-lg border border-slate-200 bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" />
                    </label>
                  </div>
                </div>
              </section>

              {error ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

              <div className="flex flex-wrap gap-3">
                <button data-testid="pricing-addon-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create add-on
                </button>
                <Link href="/pricing/add-ons" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
              </div>
            </div>
          </section>

          <aside className="grid gap-5 self-start">
            <InfoCard title="Good fit" body="Use add-ons for fixed recurring extras, not usage-linked charges." />
            <InfoCard title="Examples" body="Premium support, onboarding assistance, compliance bundle, or managed reporting." />
          </aside>
        </div>
      </main>
    </div>
  );
}

function Field({ label, value, onChange, placeholder, testID }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; testID: string }) {
  return <label className="grid gap-2 text-sm text-slate-700"><span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span><input data-testid={testID} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" /></label>;
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return <label className="grid gap-2 text-sm text-slate-700"><span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span><select value={value} onChange={(event) => onChange(event.target.value)} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2">{options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}</select></label>;
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}
