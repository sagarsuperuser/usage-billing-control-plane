import { Link, useNavigate } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
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
  const navigate = useNavigate();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const [selectedMetricIDs, setSelectedMetricIDs] = useState<string[]>([]);
  const [selectedAddOnIDs, setSelectedAddOnIDs] = useState<string[]>([]);
  const [selectedCouponIDs, setSelectedCouponIDs] = useState<string[]>([]);

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", code: "", description: "", currency: "USD", billing_interval: "monthly", status: "draft", base_amount: "49" },
  });

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
    onSuccess: (plan) => navigate({ to: `/pricing/plans/${encodeURIComponent(plan.id)}` }),
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
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}


        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create plan</h1>
                <p className="mt-0.5 text-xs text-text-muted">One base price, one cadence, and explicit linked metrics.</p>
              </div>
              <Link to="/pricing/plans" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
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

                <section className="rounded-lg border border-border bg-surface-secondary p-5">
                  <p className="text-xs font-medium text-text-muted">Linked metrics</p>
                  <div className="mt-3 grid gap-3">
                    {metricsQuery.isLoading ? (
                      <div className="animate-pulse space-y-2"><div className="h-4 w-40 rounded bg-surface-secondary" /><div className="h-4 w-28 rounded bg-surface-secondary" /></div>
                    ) : (metricsQuery.data ?? []).length === 0 ? (
                      <p className="text-sm text-text-muted">Create at least one metric before creating a plan.</p>
                    ) : (
                      metricsQuery.data?.map((metric) => (
                        <label key={metric.id} className="flex items-center gap-3 rounded-lg border border-border bg-surface px-4 py-3 text-sm text-text-secondary">
                          <input data-testid={`pricing-plan-metric-${metric.id}`} type="checkbox" checked={selectedMetricIDs.includes(metric.id)} onChange={() => toggleMetric(metric.id)} className="h-4 w-4 rounded border-slate-300" />
                          <span className="font-semibold text-text-primary">{metric.name}</span>
                          <span className="font-mono text-xs text-text-muted">{metric.key}</span>
                          <span className="text-xs font-medium text-text-muted">{metric.aggregation}</span>
                        </label>
                      ))
                    )}
                  </div>
                </section>

                <section className="rounded-lg border border-border bg-surface-secondary p-5">
                  <p className="text-xs font-medium text-text-muted">Attached add-ons</p>
                  <div className="mt-3 grid gap-3">
                    {(addOnsQuery.data ?? []).length === 0 ? (
                      <p className="text-sm text-text-muted">No add-ons created yet. This plan can still be created without them.</p>
                    ) : (
                      addOnsQuery.data?.map((addOn) => (
                        <label key={addOn.id} className="flex items-center gap-3 rounded-lg border border-border bg-surface px-4 py-3 text-sm text-text-secondary">
                          <input data-testid={`pricing-plan-addon-${addOn.id}`} type="checkbox" checked={selectedAddOnIDs.includes(addOn.id)} onChange={() => toggleAddOn(addOn.id)} className="h-4 w-4 rounded border-slate-300" />
                          <span className="font-semibold text-text-primary">{addOn.name}</span>
                          <span className="font-mono text-xs text-text-muted">{addOn.code}</span>
                          <span className="text-xs font-medium text-text-muted">{(addOn.amount_cents / 100).toFixed(2)} {addOn.currency}</span>
                        </label>
                      ))
                    )}
                  </div>
                </section>

                <section className="rounded-lg border border-border bg-surface-secondary p-5">
                  <p className="text-xs font-medium text-text-muted">Attached coupons</p>
                  <div className="mt-3 grid gap-3">
                    {(couponsQuery.data ?? []).length === 0 ? (
                      <p className="text-sm text-text-muted">No coupons created yet. This plan can still be created without them.</p>
                    ) : (
                      couponsQuery.data?.map((coupon) => (
                        <label key={coupon.id} className="flex items-center gap-3 rounded-lg border border-border bg-surface px-4 py-3 text-sm text-text-secondary">
                          <input data-testid={`pricing-plan-coupon-${coupon.id}`} type="checkbox" checked={selectedCouponIDs.includes(coupon.id)} onChange={() => toggleCoupon(coupon.id)} className="h-4 w-4 rounded border-slate-300" />
                          <span className="font-semibold text-text-primary">{coupon.name}</span>
                          <span className="font-mono text-xs text-text-muted">{coupon.code}</span>
                          <span className="text-xs font-medium text-text-muted">{coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}</span>
                        </label>
                      ))
                    )}
                  </div>
                </section>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Link to="/pricing/plans" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
                <button data-testid="pricing-plan-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create plan
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-border"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <select {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-border"}`}>
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function TextareaField({ label, error, testID, ...textareaProps }: { label: string; error?: string; testID?: string } & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <textarea data-testid={testID} {...textareaProps} aria-invalid={Boolean(error)} className={`min-h-[120px] rounded-lg border bg-surface px-3 py-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-border"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
