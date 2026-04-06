
import { Link } from "@tanstack/react-router";
import { ArrowRight, CreditCard } from "lucide-react";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { FormField, SectionHeader } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { onboardCustomer } from "@/lib/api";
import { formatReadinessStatus, normalizeMissingSteps } from "@/lib/readiness";
import { showError, showSuccess } from "@/lib/toast";
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
          billing_profile: {
            legal_name: data.legal_name.trim(),
            email: data.email.trim(),
            billing_address_line1: data.address_line1.trim(),
            billing_city: data.city.trim(),
            billing_postal_code: data.postal_code.trim(),
            billing_country: data.country.trim(),
            currency: data.currency.trim(),
          },
        },
      }),
    onSuccess: async (payload) => {
      showSuccess("Customer created");
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
    <PageContainer>
        <AppBreadcrumbs items={[{ href: "/customers", label: "Customers" }, { label: "New" }]} />

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
          <Card>
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create customer</h1>
                <p className="mt-0.5 text-xs text-text-muted">Create the customer record, apply the billing profile, and optionally start payment setup.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => window.history.back()}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Customer external ID" error={errors.external_id?.message}>
                    <Input placeholder="cust_acme_primary" {...register("external_id")} error={Boolean(errors.external_id)} />
                  </FormField>
                  <FormField label="Display name" error={errors.display_name?.message}>
                    <Input placeholder="Acme Primary Customer" {...register("display_name")} error={Boolean(errors.display_name)} />
                  </FormField>
                  <FormField label="Billing email" error={errors.email?.message}>
                    <Input placeholder="billing@acme.test" {...register("email")} error={Boolean(errors.email)} />
                  </FormField>
                </div>

                <section className="rounded-lg border border-border bg-surface-secondary/50 p-5">
                  <SectionHeader title="Billing profile" />
                  <div className="mt-4 grid gap-4 md:grid-cols-2">
                    <FormField label="Legal name" error={errors.legal_name?.message}>
                      <Input placeholder="Acme Primary Customer LLC" {...register("legal_name")} error={Boolean(errors.legal_name)} />
                    </FormField>
                    <FormField label="Billing address line 1" error={errors.address_line1?.message}>
                      <Input placeholder="1 Billing Street" {...register("address_line1")} error={Boolean(errors.address_line1)} />
                    </FormField>
                    <FormField label="Billing city" error={errors.city?.message}>
                      <Input placeholder="Bengaluru" {...register("city")} error={Boolean(errors.city)} />
                    </FormField>
                    <FormField label="Billing postal code" error={errors.postal_code?.message}>
                      <Input placeholder="560001" {...register("postal_code")} error={Boolean(errors.postal_code)} />
                    </FormField>
                    <FormField label="Billing country" error={errors.country?.message}>
                      <Input placeholder="IN" {...register("country")} error={Boolean(errors.country)} />
                    </FormField>
                    <FormField label="Currency" error={errors.currency?.message}>
                      <Input placeholder="USD" {...register("currency")} error={Boolean(errors.currency)} />
                    </FormField>
                  </div>
                </section>

                <section className="rounded-lg border border-border bg-surface-secondary/50 p-5">
                  <SectionHeader title="Payment setup" />
                  <label className="mt-4 flex items-start gap-2.5 text-sm text-text-secondary">
                    <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-border accent-blue-600" {...register("start_payment_setup")} />
                    <span>
                      <span className="font-medium text-text-primary">Start payment setup now</span>
                      <span className="mt-1 block text-xs text-text-muted">The customer receives a secure hosted link to add their payment method. Uses your workspace's connected Stripe account.</span>
                    </span>
                  </label>
                </section>

                {errors.root?.message ? (
                  <Alert tone="danger">{errors.root.message}</Alert>
                ) : null}

                {result?.checkout_url ? (
                  <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-700">
                    <p className="font-semibold text-emerald-800">Payment link</p>
                    <a href={result.checkout_url} target="_blank" rel="noreferrer" className="mt-2 block break-all rounded-lg border border-emerald-200 bg-surface px-3 py-3 font-mono text-xs text-emerald-800 hover:bg-emerald-100">
                      {result.checkout_url}
                    </a>
                  </div>
                ) : null}

                {result?.customer ? (
                  <section className="rounded-lg border border-border bg-surface-secondary p-5">
                    <p className="text-xs font-medium text-text-muted">Customer created</p>
                    <h2 className="mt-2 break-words text-base font-semibold text-text-primary">{result.customer.display_name}</h2>
                    <p className="mt-1 break-all font-mono text-xs text-text-muted">{result.customer.external_id}</p>
                    <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-text-muted">
                      <span>Customer: <span className="font-medium text-text-primary">{result.readiness.customer_active ? "Ready" : "Pending"}</span> {result.readiness.customer_active ? "(Active)" : "(Needs attention)"}</span>
                      <span>Overall: <span className="font-medium text-text-primary">{formatReadinessStatus(result.readiness.status)}</span> ({normalizeMissingSteps(result.readiness.missing_steps).length} checklist items remain)</span>
                    </div>
                    <div className="mt-5 flex flex-wrap gap-3">
                      <Link to={`/customers/${encodeURIComponent(result.customer.external_id)}`} className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-blue-600 bg-blue-600 px-4 text-sm font-medium text-white transition hover:bg-blue-700">
                        View customer detail
                        <ArrowRight className="h-4 w-4" />
                      </Link>
                      <Link to="/customers" className="inline-flex h-10 items-center justify-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Open customer directory</Link>
                    </div>
                  </section>
                ) : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" type="button" onClick={handleReset}>
                  Reset form
                </Button>
                <Button variant="primary" size="lg" type="submit" loading={busy} disabled={!canWrite || !csrfToken}>
                  <CreditCard className="h-4 w-4" />
                  Run customer setup
                </Button>
              </div>
            </form>
          </Card>
        ) : null}
    </PageContainer>
  );
}

