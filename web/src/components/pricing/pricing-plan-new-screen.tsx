import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { FormField } from "@/components/ui/form-field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { createPlan, fetchAddOns, fetchCoupons, fetchPricingMetrics } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
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
  const queryClient = useQueryClient();
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
    onSuccess: (plan) => {
      showSuccess("Plan created");
      queryClient.invalidateQueries({ queryKey: ["pricing-plans"] });
      navigate({ to: `/pricing/plans/${encodeURIComponent(plan.id)}` });
    },
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



        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create plan</h1>
                <p className="mt-0.5 text-xs text-text-muted">One base price, one cadence, and explicit linked metrics.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/plans" })}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Plan name" error={errors.name?.message}>
                    <Input data-testid="pricing-plan-name" placeholder="Growth" {...register("name")} error={Boolean(errors.name)} />
                  </FormField>
                  <FormField label="Plan code" error={errors.code?.message}>
                    <Input data-testid="pricing-plan-code" placeholder="growth" {...register("code")} error={Boolean(errors.code)} />
                  </FormField>
                  <FormField label="Currency" error={errors.currency?.message}>
                    <Input data-testid="pricing-plan-currency" placeholder="USD" {...register("currency")} error={Boolean(errors.currency)} />
                  </FormField>
                  <FormField label="Base price" error={errors.base_amount?.message}>
                    <Input data-testid="pricing-plan-base-price" placeholder="49" {...register("base_amount")} error={Boolean(errors.base_amount)} />
                  </FormField>
                  <FormField label="Billing interval" error={errors.billing_interval?.message}>
                    <Select {...register("billing_interval")} error={Boolean(errors.billing_interval)}>
                      {["monthly", "yearly"].map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
                    </Select>
                  </FormField>
                  <FormField label="Status" error={errors.status?.message}>
                    <Select {...register("status")} error={Boolean(errors.status)}>
                      {["draft", "active", "archived"].map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
                    </Select>
                  </FormField>
                  <div className="md:col-span-2">
                    <FormField label="Description" error={errors.description?.message}>
                      <Textarea data-testid="pricing-plan-description" placeholder="Best for teams moving from pilot to growth." {...register("description")} error={Boolean(errors.description)} />
                    </FormField>
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

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/plans" })}>Cancel</Button>
                <Button data-testid="pricing-plan-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create plan
                </Button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

