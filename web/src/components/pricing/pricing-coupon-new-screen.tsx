"use client";

import Link from "next/link";
import { LoaderCircle } from "lucide-react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createCoupon } from "@/lib/api";
import { showError } from "@/lib/toast";
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
  const router = useRouter();
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
    onSuccess: (coupon) => router.push(`/pricing/coupons/${encodeURIComponent(coupon.id)}`),
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create coupon", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  const discountValueSet = discountType === "percent_off"
    ? (watched.percent_off ?? "").trim().length > 0
    : (watched.amount_off ?? "").trim().length > 0;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Coupons are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {isTenantSession ? (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create coupon</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">Use coupons for structured commercial relief on plans, such as launches, negotiated discounts, or limited promotions.</p>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <form onSubmit={onSubmit} noValidate>
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="grid gap-5">
                    <div className="grid gap-3 lg:grid-cols-3">
                      <OperatorCard title="Discount shape" body="Use percent-off for simple promotions and amount-off when commercial terms require fixed relief." />
                      <OperatorCard title="Runtime scope" body="Set the frequency and expiration deliberately so operators can explain exactly when the relief stops." />
                      <OperatorCard title="After create" body="Apply the coupon through plan configuration or subscription-level commercial follow-up." />
                    </div>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Commercial record</p>
                      <h2 className="text-lg font-semibold text-slate-950">Coupon basics</h2>
                      <div className="mt-4 grid gap-4 md:grid-cols-2">
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
                    </section>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Runtime behavior</p>
                      <h2 className="text-lg font-semibold text-slate-950">Frequency and expiration</h2>
                      <div className="mt-4 grid gap-4 md:grid-cols-2">
                        <SelectField label="Frequency" options={["forever", "once", "recurring"]} testID="pricing-coupon-frequency" error={errors.frequency?.message} {...register("frequency")} />
                        {frequency === "recurring" ? (
                          <Field label="Recurring billing periods" placeholder="3" testID="pricing-coupon-frequency-duration" error={errors.frequency_duration?.message} {...register("frequency_duration")} />
                        ) : (
                          <div className="rounded-lg border border-dashed border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">Recurring duration is only needed when frequency is recurring.</div>
                        )}
                        <label className="grid gap-2 text-sm text-slate-700">
                          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Expires at</span>
                          <input data-testid="pricing-coupon-expiration-at" type="datetime-local" className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2" {...register("expiration_at")} />
                        </label>
                        <div className="rounded-lg border border-dashed border-slate-200 bg-white px-4 py-3 text-sm text-slate-500">
                          Leave expiration empty for ongoing coupons. Use once or recurring when the discount should stop after a defined number of billing periods.
                        </div>
                      </div>
                    </section>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                      <div className="mt-3 grid gap-2 md:grid-cols-2">
                        <ChecklistLine done={(watched.name ?? "").trim().length > 0} text="Coupon name is set" />
                        <ChecklistLine done={(watched.code ?? "").trim().length > 0} text="Coupon code is set" />
                        <ChecklistLine done={discountValueSet} text="Discount value is set" />
                        <ChecklistLine done={Boolean(csrfToken)} text="Writable workspace session present" />
                      </div>
                    </section>

                    {errors.root?.message ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}

                    <div className="flex flex-wrap gap-3">
                      <button data-testid="pricing-coupon-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                        {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                        Create coupon
                      </button>
                      <Link href="/pricing/coupons" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
                    </div>
                  </div>
                </section>
              </form>

              <aside className="grid gap-5 self-start">
                <InfoCard title="Operator guidance" body="Create the commercial rule here, then apply it through plans and active customer subscriptions." />
                <InfoCard title="Current scope" body="Coupons follow plan scoping, billing-period frequency, and optional expiration, then apply to customers through active subscription plans." />
              </aside>
            </div>
          </>
        ) : null}
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-xl border border-slate-200 bg-slate-50 p-4"><p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
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

function SelectField({ label, error, options, testID, ...selectProps }: { label: string; error?: string; options: string[]; testID?: string } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select data-testid={testID} {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-slate-200"}`}>
        {options.map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
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
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
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
