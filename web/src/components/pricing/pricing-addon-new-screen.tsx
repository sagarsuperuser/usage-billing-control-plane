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
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create add-on</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Use add-ons for fixed recurring extras that can be attached to multiple plans.</p>
        </section>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="grid gap-5">
              <div className="grid gap-3 lg:grid-cols-3">
                <OperatorCard title="Good fit" body="Use add-ons for fixed recurring extras, not usage-linked charges." />
                <OperatorCard title="Operator input" body="Keep the description concise and the amount obvious so account teams can attach it confidently." />
                <OperatorCard title="After create" body="Attach the add-on from plan detail or plan create once the commercial record is live." />
              </div>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Commercial record</p>
                <h2 className="text-lg font-semibold text-slate-950">Recurring add-on terms</h2>
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

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                <div className="mt-3 grid gap-2 md:grid-cols-2">
                  <ChecklistLine done={name.trim().length > 0} text="Add-on name is set" />
                  <ChecklistLine done={code.trim().length > 0} text="Add-on code is set" />
                  <ChecklistLine done={amount.trim().length > 0} text="Recurring amount is set" />
                  <ChecklistLine done={Boolean(csrfToken)} text="Writable workspace session present" />
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
            <InfoCard title="Examples" body="Premium support, onboarding assistance, compliance bundle, or managed reporting." />
            <InfoCard title="Operator guidance" body="This screen creates the reusable commercial record. Attach it to plans after submit." />
          </aside>
        </div>
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-xl border border-slate-200 bg-slate-50 p-4"><p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
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

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-white px-3 py-3"><span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>{done ? "OK" : "!"}</span><p className="text-sm text-slate-800">{text}</p></div>;
}
