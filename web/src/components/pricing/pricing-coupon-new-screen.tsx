import { Link, useNavigate } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
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
              <Link to="/pricing/coupons" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
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
                    <input data-testid="pricing-coupon-expiration-at" type="datetime-local" className="h-9 rounded-lg border border-border bg-surface px-3 text-sm text-text-secondary outline-none ring-slate-400 transition focus:ring-1" {...register("expiration_at")} />
                  </label>
                  <div className="rounded-lg border border-dashed border-border bg-surface px-4 py-3 text-sm text-text-muted">
                    Leave expiration empty for ongoing coupons. Use once or recurring when the discount should stop after a defined number of billing periods.
                  </div>
                </div>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Link to="/pricing/coupons" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
                <button data-testid="pricing-coupon-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create coupon
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

function SelectField({ label, error, options, testID, ...selectProps }: { label: string; error?: string; options: string[]; testID?: string } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <select data-testid={testID} {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-border"}`}>
        {options.map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
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
