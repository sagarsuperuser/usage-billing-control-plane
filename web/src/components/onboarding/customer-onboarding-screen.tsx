"use client";

import Link from "next/link";
import { ArrowRight, CreditCard, LoaderCircle, UserRoundPlus } from "lucide-react";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Workspace" }, { href: "/customers", label: "Customers" }, { label: "New" }]} />

        {isTenantSession ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer setup</p>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create customer</h1>
                <p className="mt-3 max-w-3xl text-sm text-slate-600">Create the customer, apply the billing profile, and optionally start payment setup.</p>
              </div>
              <Link href="/customers" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open customers</Link>
            </div>
          </section>
        ) : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="This screen drives workspace-scoped customer and payment APIs. Sign in with a workspace account."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}
        {isTenantSession && !canWrite ? (
          <ScopeNotice
            title="Read-only session"
            body={`Current session role ${role ?? "reader"} can inspect customer detail pages, but a writer or admin account is required to run setup.`}
            actionHref="/customers"
            actionLabel="Open customer directory"
          />
        ) : null}

        {isTenantSession && onboardingMutation.isSuccess ? (
          <section className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
            {watched.start_payment_setup
              ? `Customer ${result?.customer.external_id} created and payment setup is ready to continue.`
              : `Customer ${result?.customer.external_id} created and readiness has been refreshed.`}
          </section>
        ) : null}

        {isTenantSession ? (
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
            <form onSubmit={onSubmit} noValidate>
              <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
                    <h2 className="mt-2 text-xl font-semibold text-slate-950">Customer onboarding</h2>
                    <p className="mt-2 max-w-2xl text-sm text-slate-600">Create the customer record, apply the billing profile, and decide whether to start payment setup now.</p>
                  </div>
                  <span className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                    <UserRoundPlus className="h-5 w-5" />
                  </span>
                </div>

                <div className="mt-5 grid gap-3 lg:grid-cols-3">
                  <OperatorCard title="Required now" body="Customer ID, display name, and the billing record the workspace will operate on." />
                  <OperatorCard title="Billing context" body="Legal entity, address, currency, and the connection code used for collection." />
                  <OperatorCard title="After submit" body="Open the customer record for readiness, payment recovery, and hosted link follow-up." />
                </div>

                <div className="mt-5 grid gap-5">
                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Customer record</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Identity and billing contact</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-2">
                      <InputField label="Customer external ID" placeholder="cust_acme_primary" error={errors.external_id?.message} {...register("external_id")} />
                      <InputField label="Display name" placeholder="Acme Primary Customer" error={errors.display_name?.message} {...register("display_name")} />
                      <InputField label="Billing email" placeholder="billing@acme.test" error={errors.email?.message} {...register("email")} />
                    </div>
                  </section>

                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Billing profile</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Commercial and address detail</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-2">
                      <InputField label="Legal name" placeholder="Acme Primary Customer LLC" error={errors.legal_name?.message} {...register("legal_name")} />
                      <InputField label="Billing address line 1" placeholder="1 Billing Street" error={errors.address_line1?.message} {...register("address_line1")} />
                      <InputField label="Billing city" placeholder="Bengaluru" error={errors.city?.message} {...register("city")} />
                      <InputField label="Billing postal code" placeholder="560001" error={errors.postal_code?.message} {...register("postal_code")} />
                      <InputField label="Billing country" placeholder="IN" error={errors.country?.message} {...register("country")} />
                      <InputField label="Currency" placeholder="USD" error={errors.currency?.message} {...register("currency")} />
                    </div>
                  </section>

                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Payment setup</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Payment setup</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-[1.05fr_0.95fr]">
                      <div className="grid gap-4">
                        <InputField label="Billing connection code" placeholder="stripe_default" error={errors.provider_code?.message} {...register("provider_code")} />
                        <InputField label="Payment method type" placeholder="card" error={errors.payment_method_type?.message} {...register("payment_method_type")} />
                      </div>
                      <div className="rounded-xl border border-slate-200 bg-white p-4">
                        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Submission mode</p>
                        <label className="mt-3 flex items-center gap-2 text-sm text-slate-700">
                          <input type="checkbox" className="h-4 w-4 rounded border-slate-300" {...register("start_payment_setup")} />
                          Start payment setup now
                        </label>
                        <p className="mt-3 text-xs leading-relaxed text-slate-500">Leave this enabled when the payer should receive a hosted setup link immediately after the customer record is created.</p>
                      </div>
                    </div>
                  </section>
                </div>

                <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <ChecklistLine done={(watched.external_id ?? "").trim().length > 0} text="Customer ID is set" />
                    <ChecklistLine done={(watched.display_name ?? "").trim().length > 0} text="Display name is set" />
                    <ChecklistLine done={(watched.email ?? "").trim().length > 0} text="Billing email is set" />
                    <ChecklistLine done={(watched.provider_code ?? "").trim().length > 0} text="Connection code is set" />
                    <ChecklistLine done={(watched.currency ?? "").trim().length > 0} text="Currency is set" />
                    <ChecklistLine done={Boolean(watched.start_payment_setup)} text="Payment setup will start" />
                  </div>
                </div>

                {errors.root?.message ? (
                  <p className="mt-5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p>
                ) : null}

                <div className="mt-6 flex flex-wrap gap-3">
                  <button
                    type="submit"
                    disabled={!canWrite || !csrfToken || busy}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                    Run customer setup
                  </button>
                  <button
                    type="button"
                    onClick={handleReset}
                    className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
                  >
                    Reset form
                  </button>
                </div>

                {result?.checkout_url ? (
                  <div className="mt-6 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
                    <p className="font-semibold text-emerald-800">Payment link</p>
                    <a href={result.checkout_url} target="_blank" rel="noreferrer" className="mt-2 block break-all rounded-lg border border-emerald-200 bg-white px-3 py-3 font-mono text-xs text-emerald-800 hover:bg-emerald-100">
                      {result.checkout_url}
                    </a>
                  </div>
                ) : null}
              </section>
            </form>

            <aside className="min-w-0 grid gap-5 self-start">
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Next steps</p>
                <h2 className="mt-2 text-xl font-semibold text-slate-950">After creating</h2>
                <div className="mt-4 grid gap-2">
                  <ChecklistLine done text="Complete the billing profile on the customer detail page" />
                  <ChecklistLine done text="Send a payment setup request once the profile is ready" />
                  <ChecklistLine done text="Create a subscription to start billing" />
                </div>
              </section>

              {result?.customer ? (
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Customer created</p>
                  <h2 className="mt-2 break-words text-xl font-semibold text-slate-950">{result.customer.display_name}</h2>
                  <p className="mt-1 break-all font-mono text-xs text-slate-500">{result.customer.external_id}</p>
                  <div className="mt-4 grid gap-3 sm:grid-cols-2">
                    <SummaryStat label="Customer" value={result.readiness.customer_active ? "ready" : "pending"} helper={result.readiness.customer_active ? "Active" : "Needs attention"} />
                    <SummaryStat label="Overall" value={result.readiness.status} helper={`${normalizeMissingSteps(result.readiness.missing_steps).length} checklist items remain`} />
                  </div>
                  <div className="mt-5 flex flex-col gap-3">
                    <Link href={`/customers/${encodeURIComponent(result.customer.external_id)}`} className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                      View customer detail
                      <ArrowRight className="h-4 w-4" />
                    </Link>
                    <Link href="/customers" className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Open customer directory</Link>
                  </div>
                </section>
              ) : (
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Expected output</p>
                  <div className="mt-4 grid gap-3 text-sm text-slate-600">
                    <p>Submit once the operator has enough billing detail to open a usable customer record.</p>
                    <p>The created record becomes the place for payment setup follow-up, recovery actions, and collection diagnosis.</p>
                  </div>
                </section>
              )}
            </aside>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm text-slate-600">{body}</p>
    </div>
  );
}

function SummaryStat({ label, value, helper }: { label: string; value: string; helper: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className="mt-2 text-base font-semibold text-slate-950">{formatReadinessStatus(value)}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-600">{helper}</p>
    </div>
  );
}

function InputField({ label, error, ...inputProps }: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        {...inputProps}
        aria-invalid={Boolean(error)}
        className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`}
      />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
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
