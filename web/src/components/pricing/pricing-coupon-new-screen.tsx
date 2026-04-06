import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { FormField } from "@/components/ui/form-field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { createCoupon } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  name: z.string().min(1, "Required"),
  code: z.string().min(1, "Required"),
  description: z.string(),
  status: z.enum(["draft", "active", "archived"]),
  discount_type: z.enum(["percent_off", "amount_off"]),
  currency: z.string(),
  amount_off: z.string(),
  percent_off: z.string(),
  frequency: z.enum(["forever", "once", "recurring"]),
  frequency_duration: z.string(),
  expiration_at: z.string(),
});

type FormFields = z.infer<typeof schema>;

export function PricingCouponNewScreen() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const {
    register,
    handleSubmit,
    watch,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      code: "",
      description: "",
      status: "draft",
      discount_type: "percent_off",
      currency: "USD",
      amount_off: "10",
      percent_off: "20",
      frequency: "forever",
      frequency_duration: "3",
      expiration_at: "",
    },
  });

  const watched = watch();
  const discountType = watched.discount_type;
  const frequency = watched.frequency;

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createCoupon({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name: data.name,
          code: data.code,
          description: data.description,
          status: data.status,
          discount_type: data.discount_type,
          currency: data.discount_type === "amount_off" ? data.currency : "",
          amount_off_cents: data.discount_type === "amount_off" ? Math.round(Number(data.amount_off || 0) * 100) : 0,
          percent_off: data.discount_type === "percent_off" ? Math.round(Number(data.percent_off || 0)) : 0,
          frequency: data.frequency,
          frequency_duration: data.frequency === "recurring" ? Math.max(1, Math.round(Number(data.frequency_duration || 0))) : 0,
          expiration_at: data.expiration_at ? new Date(data.expiration_at).toISOString() : null,
        },
      }),
    onSuccess: (coupon) => {
      showSuccess("Coupon created");
      queryClient.invalidateQueries({ queryKey: ["pricing-coupons"] });
      navigate({ to: `/pricing/coupons/${encodeURIComponent(coupon.id)}` });
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create coupon", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: "New" }]} />



        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create coupon</h1>
                <p className="mt-0.5 text-xs text-text-muted">Structured commercial relief for plans, launches, or negotiated discounts.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/coupons" })}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Coupon name" error={errors.name?.message}>
                    <Input data-testid="pricing-coupon-name" placeholder="Launch 20" {...register("name")} error={Boolean(errors.name)} />
                  </FormField>
                  <FormField label="Coupon code" error={errors.code?.message}>
                    <Input data-testid="pricing-coupon-code" placeholder="launch_20" {...register("code")} error={Boolean(errors.code)} />
                  </FormField>
                  <FormField label="Status" error={errors.status?.message}>
                    <Select {...register("status")} error={Boolean(errors.status)}>
                      {["draft", "active", "archived"].map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
                    </Select>
                  </FormField>
                  <FormField label="Discount type" error={errors.discount_type?.message}>
                    <Select {...register("discount_type")} error={Boolean(errors.discount_type)}>
                      {["percent_off", "amount_off"].map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
                    </Select>
                  </FormField>
                  {discountType === "amount_off" ? (
                    <>
                      <FormField label="Currency" error={errors.currency?.message}>
                        <Input data-testid="pricing-coupon-currency" placeholder="USD" {...register("currency")} error={Boolean(errors.currency)} />
                      </FormField>
                      <FormField label="Amount off" error={errors.amount_off?.message}>
                        <Input data-testid="pricing-coupon-amount-off" placeholder="10" {...register("amount_off")} error={Boolean(errors.amount_off)} />
                      </FormField>
                    </>
                  ) : (
                    <FormField label="Percent off" error={errors.percent_off?.message}>
                      <Input data-testid="pricing-coupon-percent-off" placeholder="20" {...register("percent_off")} error={Boolean(errors.percent_off)} />
                    </FormField>
                  )}
                  <div className="md:col-span-2">
                    <FormField label="Description" error={errors.description?.message}>
                      <Textarea data-testid="pricing-coupon-description" placeholder="Applied to early launch customers on annual commit." {...register("description")} error={Boolean(errors.description)} />
                    </FormField>
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Frequency" error={errors.frequency?.message}>
                    <Select data-testid="pricing-coupon-frequency" {...register("frequency")} error={Boolean(errors.frequency)}>
                      {["forever", "once", "recurring"].map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
                    </Select>
                  </FormField>
                  {frequency === "recurring" ? (
                    <FormField label="Recurring billing periods" error={errors.frequency_duration?.message}>
                      <Input data-testid="pricing-coupon-frequency-duration" placeholder="3" {...register("frequency_duration")} error={Boolean(errors.frequency_duration)} />
                    </FormField>
                  ) : (
                    <div className="rounded-lg border border-dashed border-border bg-surface px-4 py-3 text-sm text-text-muted">Recurring duration is only needed when frequency is recurring.</div>
                  )}
                  <FormField label="Expires at">
                    <Input data-testid="pricing-coupon-expiration-at" type="datetime-local" inputSize="md" {...register("expiration_at")} />
                  </FormField>
                  <div className="rounded-lg border border-dashed border-border bg-surface px-4 py-3 text-sm text-text-muted">
                    Leave expiration empty for ongoing coupons. Use once or recurring when the discount should stop after a defined number of billing periods.
                  </div>
                </div>

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/coupons" })}>Cancel</Button>
                <Button data-testid="pricing-coupon-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create coupon
                </Button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

