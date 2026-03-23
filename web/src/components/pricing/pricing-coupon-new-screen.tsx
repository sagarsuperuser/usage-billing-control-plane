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
import { createCoupon } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingCouponNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState("draft");
  const [discountType, setDiscountType] = useState<"amount_off" | "percent_off">("percent_off");
  const [currency, setCurrency] = useState("USD");
  const [amountOff, setAmountOff] = useState("10");
  const [percentOff, setPercentOff] = useState("20");
  const [error, setError] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: () =>
      createCoupon({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name,
          code,
          description,
          status,
          discount_type: discountType,
          currency: discountType === "amount_off" ? currency : "",
          amount_off_cents: discountType === "amount_off" ? Math.round(Number(amountOff || 0) * 100) : 0,
          percent_off: discountType === "percent_off" ? Math.round(Number(percentOff || 0)) : 0,
        },
      }),
    onSuccess: (coupon) => router.push(`/pricing/coupons/${encodeURIComponent(coupon.id)}`),
    onError: (err: Error) => setError(err.message),
  });

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Coupons are tenant-scoped. Sign in with a tenant account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing coupon</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create coupon</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Use coupons for structured commercial relief on plans, such as launches, negotiated discounts, or limited promotions.</p>
        </section>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <h2 className="text-lg font-semibold text-slate-950">Commercial basics</h2>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <Field label="Coupon name" value={name} onChange={setName} placeholder="Launch 20" testID="pricing-coupon-name" />
                  <Field label="Coupon code" value={code} onChange={setCode} placeholder="launch_20" testID="pricing-coupon-code" />
                  <SelectField label="Status" value={status} onChange={setStatus} options={["draft", "active", "archived"]} />
                  <SelectField label="Discount type" value={discountType} onChange={(value) => setDiscountType(value as "amount_off" | "percent_off")} options={["percent_off", "amount_off"]} />
                  {discountType === "amount_off" ? (
                    <>
                      <Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-coupon-currency" />
                      <Field label="Amount off" value={amountOff} onChange={setAmountOff} placeholder="10" testID="pricing-coupon-amount-off" />
                    </>
                  ) : (
                    <Field label="Percent off" value={percentOff} onChange={setPercentOff} placeholder="20" testID="pricing-coupon-percent-off" />
                  )}
                  <div className="md:col-span-2">
                    <label className="grid gap-2 text-sm text-slate-700">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Description</span>
                      <textarea data-testid="pricing-coupon-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Applied to early launch customers on annual commit." className="min-h-[120px] rounded-lg border border-slate-200 bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" />
                    </label>
                  </div>
                </div>
              </section>

              {error ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

              <div className="flex flex-wrap gap-3">
                <button data-testid="pricing-coupon-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create coupon
                </button>
                <Link href="/pricing/coupons" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
              </div>
            </div>
          </section>

          <aside className="grid gap-5 self-start">
            <InfoCard title="Good fit" body="Use percent-off for simple promos and amount-off for negotiated fixed relief." />
            <InfoCard title="Current scope" body="Coupons are modeled and attachable to plans. This slice does not yet change invoice math." />
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
  return <label className="grid gap-2 text-sm text-slate-700"><span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span><select value={value} onChange={(event) => onChange(event.target.value)} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2">{options.map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}</select></label>;
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}
