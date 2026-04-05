
import { Link } from "@tanstack/react-router";
import { ArrowRight, CreditCard, LoaderCircle } from "lucide-react";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { onboardCustomer } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { showError } from "@/lib/toast";
import { type CustomerOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  external_id: z.string().min(1, "Required"),
  display_name: z.string().min(1, "Required"),
  email: z.string().email("Enter a valid email").or(z.literal("")),
  legal_name: z.string(),
  address_line1: z.string(),
  city: z.string(),
  postal_code: z.string(),
  country: z.string(),
  currency: z.string().min(1, "Required"),
  provider_code: z.string(),
  payment_method_type: z.string(),
  start_payment_setup: z.boolean(),
});

type FormFields = z.infer<typeof schema>;

export function CustomerOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, role, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [result, setResult] = useState<CustomerOnboardingResult | null>(null);

  const {
    register,
    handleSubmit,
    watch,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: {
      external_id: "",
      display_name: "",
      email: "",
      legal_name: "",
      address_line1: "",
      city: "",
      postal_code: "",
      country: "",
      currency: "USD",
      provider_code: "",
      payment_method_type: "card",
      start_payment_setup: true,
    },
  });

  const watched = watch();

  const onboardingMutation = useMutation({
    mutationFn: (data: FormFields) =>
      onboardCustomer({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          external_id: data.external_id.trim(),
          display_name: data.display_name.trim(),
          email: data.email.trim(),
          start_payment_setup: data.start_payment_setup,
          payment_method_type: data.payment_method_type.trim() || undefined,
          billing_profile: {
            legal_name: data.legal_name.trim(),
            email: data.email.trim(),
            billing_address_line1: data.address_line1.trim(),
            billing_city: data.city.trim(),
            billing_postal_code: data.postal_code.trim(),
            billing_country: data.country.trim(),
            currency: data.currency.trim(),
            provider_code: data.provider_code.trim(),
          },
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["overview-customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.customer.external_id] }),
      ]);
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Customer setup failed", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => onboardingMutation.mutate(data));
  const busy = isSubmitting || onboardingMutation.isPending;

  const handleReset = () => {
    reset();
    setResult(null);
    onboardingMutation.reset();
  };

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/customers", label: "Customers" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isTenantSession && !canWrite ? (
          <ScopeNotice
            title="Read-only session"
            body={`Current session role ${role ?? "reader"} can inspect customer detail pages, but a writer or admin account is required to run setup.`}
            actionHref="/customers"
            actionLabel="Open customer directory"
          />
        ) : null}

        {isTenantSession && onboardingMutation.isSuccess ? (
          <section className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {watched.start_payment_setup
              ? `Customer ${result?.customer.external_id} created and payment setup is ready to continue.`
              : `Customer ${result?.customer.external_id} created and readiness has been refreshed.`}
          </section>
        ) : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-slate-900">Create customer</h1>
                <p className="mt-0.5 text-xs text-slate-500">Create the customer record, apply the billing profile, and optionally start payment setup.</p>
              </div>
              <Link to="/customers" className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <InputField label="Customer external ID" placeholder="cust_acme_primary" error={errors.external_id?.message} {...register("external_id")} />
                  <InputField label="Display name" placeholder="Acme Primary Customer" error={errors.display_name?.message} {...register("display_name")} />
                  <InputField label="Billing email" placeholder="billing@acme.test" error={errors.email?.message} {...register("email")} />
                </div>

                <section className="rounded-lg border border-stone-200 bg-slate-50 p-5">
                  <p className="text-xs font-medium text-slate-500">Billing profile</p>
                  <div className="mt-4 grid gap-4 md:grid-cols-2">
                    <InputField label="Legal name" placeholder="Acme Primary Customer LLC" error={errors.legal_name?.message} {...register("legal_name")} />
                    <InputField label="Billing address line 1" placeholder="1 Billing Street" error={errors.address_line1?.message} {...register("address_line1")} />
                    <InputField label="Billing city" placeholder="Bengaluru" error={errors.city?.message} {...register("city")} />
                    <InputField label="Billing postal code" placeholder="560001" error={errors.postal_code?.message} {...register("postal_code")} />
                    <InputField label="Billing country" placeholder="IN" error={errors.country?.message} {...register("country")} />
                    <InputField label="Currency" placeholder="USD" error={errors.currency?.message} {...register("currency")} />
                  </div>
                </section>

                <section className="rounded-lg border border-stone-200 bg-slate-50 p-5">
                  <p className="text-xs font-medium text-slate-500">Payment setup</p>
                  <div className="mt-4 grid gap-4 md:grid-cols-[1.05fr_0.95fr]">
                    <div className="grid gap-4">
                      <InputField label="Billing connection code" placeholder="stripe_default" error={errors.provider_code?.message} {...register("provider_code")} />
                      <InputField label="Payment method type" placeholder="card" error={errors.payment_method_type?.message} {...register("payment_method_type")} />
                    </div>
                    <div className="rounded-lg border border-stone-200 bg-white p-4">
                      <p className="text-xs font-medium text-slate-500">Submission mode</p>
                      <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                        <input type="checkbox" className="h-4 w-4 rounded border-slate-300" {...register("start_payment_setup")} />
                        Start payment setup now
                      </label>
                      <p className="mt-3 text-xs leading-relaxed text-slate-500">Leave this enabled when the payer should receive a hosted setup link immediately after the customer record is created.</p>
                    </div>
                  </div>
                </section>

                {errors.root?.message ? (
                  <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p>
                ) : null}

                {result?.checkout_url ? (
                  <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
                    <p className="font-semibold text-emerald-800">Payment link</p>
                    <a href={result.checkout_url} target="_blank" rel="noreferrer" className="mt-2 block break-all rounded-lg border border-emerald-200 bg-white px-3 py-3 font-mono text-xs text-emerald-800 hover:bg-emerald-100">
                      {result.checkout_url}
                    </a>
                  </div>
                ) : null}

                {result?.customer ? (
                  <section className="rounded-lg border border-stone-200 bg-slate-50 p-5">
                    <p className="text-xs font-medium text-slate-500">Customer created</p>
                    <h2 className="mt-2 break-words text-base font-semibold text-slate-950">{result.customer.display_name}</h2>
                    <p className="mt-1 break-all font-mono text-xs text-slate-500">{result.customer.external_id}</p>
                    <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-slate-600">
                      <span>Customer: <span className="font-medium text-slate-900">{result.readiness.customer_active ? "Ready" : "Pending"}</span> {result.readiness.customer_active ? "(Active)" : "(Needs attention)"}</span>
                      <span>Overall: <span className="font-medium text-slate-900">{formatReadinessStatus(result.readiness.status)}</span> ({normalizeMissingSteps(result.readiness.missing_steps).length} checklist items remain)</span>
                    </div>
                    <div className="mt-5 flex flex-wrap gap-3">
                      <Link to={`/customers/${encodeURIComponent(result.customer.external_id)}`} className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                        View customer detail
                        <ArrowRight className="h-4 w-4" />
                      </Link>
                      <Link to="/customers" className="inline-flex h-10 items-center justify-center rounded-lg border border-stone-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open customer directory</Link>
                    </div>
                  </section>
                ) : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-stone-200 px-6 py-4">
                <button
                  type="button"
                  onClick={handleReset}
                  className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                >
                  Reset form
                </button>
                <button
                  type="submit"
                  disabled={!canWrite || !csrfToken || busy}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                  Run customer setup
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function InputField({ label, error, ...inputProps }: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-xs font-medium text-slate-500">{label}</span>
      <input
        {...inputProps}
        aria-invalid={Boolean(error)}
        className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-stone-200"}`}
      />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
