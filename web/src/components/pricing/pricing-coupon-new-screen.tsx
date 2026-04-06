import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
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
                  <Field label="Coupon name" placeholder="Launch 20" testID="pricing-coupon-name" error={errors.name?.message} {...register("name")} />
                  <Field label="Coupon code" placeholder="launch_20" testID="pricing-coupon-code" error={errors.code?.message} {...register("code")} />
                  <SelectField label="Status" options={["draft", "active", "archived"]} error={errors.status?.message} {...register("status")} />
                  <SelectField label="Discount type" options={["percent_off", "amount_off"]} error={errors.discount_type?.message} {...register("discount_type")} />
                  {discountType === "amount_off" ? (
                    <>
                      <Field label="Currency" placeholder="USD" testID="pricing-coupon-currency" error={errors.currency?.message} {...register("currency")} />
                      <Field label="Amount off" placeholder="10" testID="pricing-coupon-amount-off" error={errors.amount_off?.message} {...register("amount_off")} />
                    </>
                  ) : (
                    <Field label="Percent off" placeholder="20" testID="pricing-coupon-percent-off" error={errors.percent_off?.message} {...register("percent_off")} />
                  )}
                  <div className="md:col-span-2">
                    <TextareaField label="Description" placeholder="Applied to early launch customers on annual commit." testID="pricing-coupon-description" error={errors.description?.message} {...register("description")} />
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <SelectField label="Frequency" options={["forever", "once", "recurring"]} testID="pricing-coupon-frequency" error={errors.frequency?.message} {...register("frequency")} />
                  {frequency === "recurring" ? (
                    <Field label="Recurring billing periods" placeholder="3" testID="pricing-coupon-frequency-duration" error={errors.frequency_duration?.message} {...register("frequency_duration")} />
                  ) : (
                    <div className="rounded-lg border border-dashed border-border bg-surface px-4 py-3 text-sm text-text-muted">Recurring duration is only needed when frequency is recurring.</div>
                  )}
                  <label className="grid gap-2 text-sm text-text-secondary">
                    <span className="text-xs font-medium text-text-muted">Expires at</span>
                    <Input data-testid="pricing-coupon-expiration-at" type="datetime-local" inputSize="md" {...register("expiration_at")} />
                  </label>
                  <div className="rounded-lg border border-dashed border-border bg-surface px-4 py-3 text-sm text-text-muted">
                    Leave expiration empty for ongoing coupons. Use once or recurring when the discount should stop after a defined number of billing periods.
                  </div>
                </div>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
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

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <Input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} error={Boolean(error)} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, testID, ...selectProps }: { label: string; error?: string; options: string[]; testID?: string } & React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <Select data-testid={testID} {...selectProps} aria-invalid={Boolean(error)} error={Boolean(error)}>
        {options.map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
      </Select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function TextareaField({ label, error, testID, ...textareaProps }: { label: string; error?: string; testID?: string } & React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <Textarea data-testid={testID} {...textareaProps} aria-invalid={Boolean(error)} error={Boolean(error)} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
