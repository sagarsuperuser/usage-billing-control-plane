
import { Link } from "@tanstack/react-router";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import { useMemo } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { FormField } from "@/components/ui/form-field";
import { Input, Select } from "@/components/ui/input";
import { createSubscription, fetchCustomers, fetchPlans } from "@/lib/api";
import { formatReadinessStatus } from "@/lib/readiness";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  display_name: z.string(),
  code: z.string(),
  customer_external_id: z.string().min(1, "Select a customer"),
  plan_id: z.string().min(1, "Select a plan"),
  request_payment_setup: z.boolean(),
  payment_method_type: z.string(),
});

type FormFields = z.infer<typeof schema>;

export function SubscriptionNewScreen() {
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
      display_name: "",
      code: "",
      customer_external_id: "",
      plan_id: "",
      request_payment_setup: true,
      payment_method_type: "card",
    },
  });

  // eslint-disable-next-line react-hooks/incompatible-library -- watch() triggers re-render on form value changes
  const _watched = { customer_external_id: watch("customer_external_id"), plan_id: watch("plan_id"), request_payment_setup: watch("request_payment_setup") };

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, "subscriptions-new"],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isTenantSession,
  });
  const plansQuery = useQuery({
    queryKey: ["plans", apiBaseURL, "subscriptions-new"],
    queryFn: () => fetchPlans({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const customers = useMemo(() => customersQuery.data ?? [], [customersQuery.data]);
  const plans = useMemo(() => plansQuery.data ?? [], [plansQuery.data]);

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createSubscription({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          code: data.code,
          display_name: data.display_name,
          customer_external_id: data.customer_external_id,
          plan_id: data.plan_id,
          request_payment_setup: data.request_payment_setup,
          payment_method_type: data.payment_method_type,
        },
      }),
    onSuccess: () => showSuccess("Subscription ready"),
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create subscription", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/subscriptions", label: "Subscriptions" }, { label: "New" }]} />


        {isTenantSession && mutation.isSuccess ? (
          <section className="rounded-lg border border-emerald-200 bg-emerald-50 p-6 shadow-sm">
            <div className="flex items-start gap-3">
              <CheckCircle2 className="mt-0.5 h-5 w-5 text-emerald-700" />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-emerald-800">Subscription created</p>
                <p className="mt-2 text-sm text-emerald-700">{mutation.data.subscription.display_name} is now {formatReadinessStatus(mutation.data.subscription.status)}.</p>
                {mutation.data.checkout_url ? (
                  <a href={mutation.data.checkout_url} target="_blank" rel="noreferrer" className="mt-4 inline-flex h-10 items-center rounded-lg border border-emerald-200 bg-surface px-4 text-sm font-medium text-emerald-700 transition hover:bg-emerald-100">
                    Open payment setup link
                  </a>
                ) : null}
                <div className="mt-4 flex flex-wrap gap-3">
                  <a href={`/subscriptions/${encodeURIComponent(mutation.data.subscription.id)}`} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                    Open subscription
                    <ArrowRight className="h-4 w-4" />
                  </a>
                  <Link to="/subscriptions" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">
                    Back to subscriptions
                  </Link>
                </div>
              </div>
            </div>
          </section>
        ) : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create subscription</h1>
                <p className="mt-0.5 text-xs text-text-muted">Choose the customer and plan, then decide whether to start hosted payment setup.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => window.history.back()}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Subscription name" hint="Optional. Alpha can generate a default.">
                    <Input data-testid="subscription-name" placeholder="Acme Growth" {...register("display_name")} />
                  </FormField>
                  <FormField label="Code" hint="Optional stable internal reference.">
                    <Input data-testid="subscription-code" placeholder="acme_growth" {...register("code")} />
                  </FormField>
                  <FormField label="Customer" hint="The account that is subscribing." error={errors.customer_external_id?.message}>
                    <Select data-testid="subscription-customer" error={Boolean(errors.customer_external_id)} {...register("customer_external_id")}>
                      <option value="">Select customer</option>
                      {customers.map((customer) => (
                        <option key={customer.id} value={customer.external_id}>{customer.display_name} ({customer.external_id})</option>
                      ))}
                    </Select>
                  </FormField>
                  <FormField label="Plan" hint="The commercial package this customer is signing up for." error={errors.plan_id?.message}>
                    <Select data-testid="subscription-plan" error={Boolean(errors.plan_id)} {...register("plan_id")}>
                      <option value="">Select plan</option>
                      {plans.map((plan) => (
                        <option key={plan.id} value={plan.id}>{plan.name} ({plan.code})</option>
                      ))}
                    </Select>
                  </FormField>
                </div>

                <label className="flex items-start gap-3 text-sm text-text-secondary">
                  <input data-testid="subscription-request-payment-setup" type="checkbox" className="mt-1 h-4 w-4 rounded border-slate-300" {...register("request_payment_setup")} />
                  <span>
                    <span className="font-semibold text-text-primary">Request payment setup now</span>
                    <span className="mt-1 block text-text-muted">Alpha generates a secure hosted link. Send it to the customer — they complete card or bank setup on their end.</span>
                  </span>
                </label>
                <div className="max-w-sm">
                  <FormField label="Payment method type" hint="Defaults to card.">
                    <Input data-testid="subscription-payment-method-type" placeholder="card" {...register("payment_method_type")} />
                  </FormField>
                </div>

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Link to="/subscriptions" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
                <Button variant="primary" size="lg" type="submit" data-testid="subscription-submit" loading={busy} disabled={!csrfToken}>
                  Create subscription
                </Button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

