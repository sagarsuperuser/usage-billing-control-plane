"use client";

import Link from "next/link";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { LoaderCircle } from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createPlan, fetchAddOns, fetchCoupons, fetchPricingMetrics } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [billingInterval, setBillingInterval] = useState("monthly");
  const [status, setStatus] = useState("draft");
  const [baseAmount, setBaseAmount] = useState("49");
  const [selectedMetricIDs, setSelectedMetricIDs] = useState<string[]>([]);
  const [selectedAddOnIDs, setSelectedAddOnIDs] = useState<string[]>([]);
  const [selectedCouponIDs, setSelectedCouponIDs] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });
  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });
  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const mutation = useMutation({
    mutationFn: () =>
      createPlan({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name,
          code,
          description,
          currency,
          billing_interval: billingInterval,
          status,
          base_amount_cents: Math.round(Number(baseAmount || 0) * 100),
          meter_ids: selectedMetricIDs,
          add_on_ids: selectedAddOnIDs,
          coupon_ids: selectedCouponIDs,
        },
      }),
    onSuccess: (plan) => router.push(`/pricing/plans/${encodeURIComponent(plan.id)}`),
    onError: (err: Error) => setError(err.message),
  });

  const toggleMetric = (metricID: string) => {
    setSelectedMetricIDs((current) => (current.includes(metricID) ? current.filter((id) => id !== metricID) : [...current, metricID]));
  };
  const toggleAddOn = (addOnID: string) => {
    setSelectedAddOnIDs((current) => (current.includes(addOnID) ? current.filter((id) => id !== addOnID) : [...current, addOnID]));
  };
  const toggleCoupon = (couponID: string) => {
    setSelectedCouponIDs((current) => (current.includes(couponID) ? current.filter((id) => id !== couponID) : [...current, couponID]));
  };

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Plans are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing plan</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create plan</h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-600">Keep the first version opinionated: one base price, one cadence, and explicit linked metrics.</p>
        </section>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="grid gap-5">
              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <h2 className="text-lg font-semibold text-slate-950">Commercial basics</h2>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <Field label="Plan name" value={name} onChange={setName} placeholder="Growth" testID="pricing-plan-name" />
                  <Field label="Plan code" value={code} onChange={setCode} placeholder="growth" testID="pricing-plan-code" />
                  <Field label="Currency" value={currency} onChange={setCurrency} placeholder="USD" testID="pricing-plan-currency" />
                  <Field label="Base price" value={baseAmount} onChange={setBaseAmount} placeholder="49" testID="pricing-plan-base-price" />
                  <SelectField label="Billing interval" value={billingInterval} onChange={setBillingInterval} options={["monthly", "yearly"]} />
                  <SelectField label="Status" value={status} onChange={setStatus} options={["draft", "active", "archived"]} />
                  <div className="md:col-span-2">
                    <label className="grid gap-2 text-sm text-slate-700">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Description</span>
                      <textarea data-testid="pricing-plan-description" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Best for teams moving from pilot to growth." className="min-h-[120px] rounded-lg border border-slate-200 bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" />
                    </label>
                  </div>
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Linked metrics</p>
                <div className="mt-3 grid gap-3">
                  {metricsQuery.isLoading ? (
                    <div className="flex items-center gap-2 text-sm text-slate-600"><LoaderCircle className="h-4 w-4 animate-spin" />Loading metrics</div>
                  ) : (metricsQuery.data ?? []).length === 0 ? (
                    <p className="text-sm text-slate-600">Create at least one metric before creating a plan.</p>
                  ) : (
                    metricsQuery.data?.map((metric) => (
                      <label key={metric.id} className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700">
                        <input data-testid={`pricing-plan-metric-${metric.id}`} type="checkbox" checked={selectedMetricIDs.includes(metric.id)} onChange={() => toggleMetric(metric.id)} className="h-4 w-4 rounded border-slate-300" />
                        <span className="font-semibold text-slate-950">{metric.name}</span>
                        <span className="font-mono text-xs text-slate-500">{metric.key}</span>
                        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{metric.aggregation}</span>
                      </label>
                    ))
                  )}
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Attached add-ons</p>
                <div className="mt-3 grid gap-3">
                  {(addOnsQuery.data ?? []).length === 0 ? (
                    <p className="text-sm text-slate-600">No add-ons created yet. This plan can still be created without them.</p>
                  ) : (
                    addOnsQuery.data?.map((addOn) => (
                      <label key={addOn.id} className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700">
                        <input data-testid={`pricing-plan-addon-${addOn.id}`} type="checkbox" checked={selectedAddOnIDs.includes(addOn.id)} onChange={() => toggleAddOn(addOn.id)} className="h-4 w-4 rounded border-slate-300" />
                        <span className="font-semibold text-slate-950">{addOn.name}</span>
                        <span className="font-mono text-xs text-slate-500">{addOn.code}</span>
                        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{(addOn.amount_cents / 100).toFixed(2)} {addOn.currency}</span>
                      </label>
                    ))
                  )}
                </div>
              </section>

              <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Attached coupons</p>
                <div className="mt-3 grid gap-3">
                  {(couponsQuery.data ?? []).length === 0 ? (
                    <p className="text-sm text-slate-600">No coupons created yet. This plan can still be created without them.</p>
                  ) : (
                    couponsQuery.data?.map((coupon) => (
                      <label key={coupon.id} className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700">
                        <input data-testid={`pricing-plan-coupon-${coupon.id}`} type="checkbox" checked={selectedCouponIDs.includes(coupon.id)} onChange={() => toggleCoupon(coupon.id)} className="h-4 w-4 rounded border-slate-300" />
                        <span className="font-semibold text-slate-950">{coupon.name}</span>
                        <span className="font-mono text-xs text-slate-500">{coupon.code}</span>
                        <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}</span>
                      </label>
                    ))
                  )}
                </div>
              </section>

              {error ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

              <div className="flex flex-wrap gap-3">
                <button data-testid="pricing-plan-submit" type="button" onClick={() => mutation.mutate()} disabled={!csrfToken || mutation.isPending || selectedMetricIDs.length === 0} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create plan
                </button>
                <Link href="/pricing/plans" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
              </div>
            </div>
          </section>

          <aside className="grid gap-5 self-start">
            <InfoCard title="Before you start" body="You need at least one metric before a plan can be created." />
            <InfoCard title="Design rule" body="Keep the first version simple: stable naming, one base price, and explicit linked metrics." />
            <InfoCard title="Current selection" body={`${selectedMetricIDs.length} metric(s), ${selectedAddOnIDs.length} add-on(s), ${selectedCouponIDs.length} coupon(s)`} />
          </aside>
        </div>
      </main>
    </div>
  );
}

function Field({ label, value, onChange, placeholder, testID }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; testID: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input data-testid={testID} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2" />
    </label>
  );
}

function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2">
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
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
