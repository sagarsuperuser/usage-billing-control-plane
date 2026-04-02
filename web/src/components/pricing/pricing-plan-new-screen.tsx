"use client";

import Link from "next/link";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createPlan, fetchAddOns, fetchCoupons, fetchPricingMetrics } from "@/lib/api";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  name: z.string().min(1, "Required"),
  code: z.string().min(1, "Required"),
  description: z.string(),
  currency: z.string().min(1, "Required"),
  billing_interval: z.enum(["monthly", "yearly"]),
  status: z.enum(["draft", "active", "archived"]),
  base_amount: z.string().min(1, "Required").refine((v) => !isNaN(Number(v)) && Number(v) >= 0, "Must be a valid number"),
});

type FormFields = z.infer<typeof schema>;

export function PricingPlanNewScreen() {
  const router = useRouter();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [selectedMetricIDs, setSelectedMetricIDs] = useState<string[]>([]);
  const [selectedAddOnIDs, setSelectedAddOnIDs] = useState<string[]>([]);
  const [selectedCouponIDs, setSelectedCouponIDs] = useState<string[]>([]);

  const {
    register,
    handleSubmit,
    watch,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", code: "", description: "", currency: "USD", billing_interval: "monthly", status: "draft", base_amount: "49" },
  });

  const watched = watch();

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createPlan({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name: data.name,
          code: data.code,
          description: data.description,
          currency: data.currency,
          billing_interval: data.billing_interval,
          status: data.status,
          base_amount_cents: Math.round(Number(data.base_amount) * 100),
          meter_ids: selectedMetricIDs,
          add_on_ids: selectedAddOnIDs,
          coupon_ids: selectedCouponIDs,
        },
      }),
    onSuccess: (plan) => router.push(`/pricing/plans/${encodeURIComponent(plan.id)}`),
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create plan", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => {
    if (selectedMetricIDs.length === 0) {
      setError("root", { message: "At least one metric must be selected." });
      return;
    }
    mutation.mutate(data);
  });
  const busy = isSubmitting || mutation.isPending;

  const toggleMetric = (id: string) => setSelectedMetricIDs((c) => c.includes(id) ? c.filter((x) => x !== id) : [...c, id]);
  const toggleAddOn = (id: string) => setSelectedAddOnIDs((c) => c.includes(id) ? c.filter((x) => x !== id) : [...c, id]);
  const toggleCoupon = (id: string) => setSelectedCouponIDs((c) => c.includes(id) ? c.filter((x) => x !== id) : [...c, id]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Plans are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {isTenantSession ? (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create plan</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">Keep the first version opinionated: one base price, one cadence, and explicit linked metrics.</p>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <form onSubmit={onSubmit} noValidate>
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="grid gap-5">
                    <div className="grid gap-3 lg:grid-cols-3">
                      <OperatorCard title="Commercial scope" body="Keep the first plan opinionated: one base price, one cadence, and explicit linked metrics." />
                      <OperatorCard title="Dependencies" body="Plans require at least one metric. Add-ons and coupons are optional attachments." />
                      <OperatorCard title="After create" body="Use plan detail for tax assignment, activation review, and commercial attachment changes." />
                    </div>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Commercial record</p>
                      <h2 className="text-lg font-semibold text-slate-950">Commercial basics</h2>
                      <div className="mt-4 grid gap-4 md:grid-cols-2">
                        <Field label="Plan name" placeholder="Growth" testID="pricing-plan-name" error={errors.name?.message} {...register("name")} />
                        <Field label="Plan code" placeholder="growth" testID="pricing-plan-code" error={errors.code?.message} {...register("code")} />
                        <Field label="Currency" placeholder="USD" testID="pricing-plan-currency" error={errors.currency?.message} {...register("currency")} />
                        <Field label="Base price" placeholder="49" testID="pricing-plan-base-price" error={errors.base_amount?.message} {...register("base_amount")} />
                        <SelectField label="Billing interval" options={["monthly", "yearly"]} error={errors.billing_interval?.message} {...register("billing_interval")} />
                        <SelectField label="Status" options={["draft", "active", "archived"]} error={errors.status?.message} {...register("status")} />
                        <div className="md:col-span-2">
                          <TextareaField label="Description" placeholder="Best for teams moving from pilot to growth." testID="pricing-plan-description" error={errors.description?.message} {...register("description")} />
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

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                      <div className="mt-3 grid gap-2 md:grid-cols-2">
                        <ChecklistLine done={(watched.name ?? "").trim().length > 0} text="Plan name is set" />
                        <ChecklistLine done={(watched.code ?? "").trim().length > 0} text="Plan code is set" />
                        <ChecklistLine done={selectedMetricIDs.length > 0} text="At least one metric is attached" />
                        <ChecklistLine done={Boolean(csrfToken)} text="Writable workspace session present" />
                      </div>
                    </section>

                    {errors.root?.message ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}

                    <div className="flex flex-wrap gap-3">
                      <button data-testid="pricing-plan-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                        {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                        Create plan
                      </button>
                      <Link href="/pricing/plans" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
                    </div>
                  </div>
                </section>
              </form>

              <aside className="grid gap-5 self-start">
                <InfoCard title="Before you start" body="You need at least one metric before a plan can be created." />
                <InfoCard title="Operator guidance" body="This screen creates the commercial package. Use plan detail afterward for activation checks and attachment changes." />
                <InfoCard title="Current selection" body={`${selectedMetricIDs.length} metric(s), ${selectedAddOnIDs.length} add-on(s), ${selectedCouponIDs.length} coupon(s)`} />
              </aside>
            </div>
          </>
        ) : null}
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p>
    </section>
  );
}

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-slate-200"}`}>
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function TextareaField({ label, error, testID, ...textareaProps }: { label: string; error?: string; testID?: string } & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <textarea data-testid={testID} {...textareaProps} aria-invalid={Boolean(error)} className={`min-h-[120px] rounded-lg border bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
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
