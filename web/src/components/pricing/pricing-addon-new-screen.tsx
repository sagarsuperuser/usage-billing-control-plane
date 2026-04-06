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
import { createAddOn } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  name: z.string().min(1, "Required"),
  code: z.string().min(1, "Required"),
  description: z.string(),
  currency: z.string().min(1, "Required"),
  billing_interval: z.enum(["monthly", "yearly"]),
  status: z.enum(["draft", "active", "archived"]),
  amount: z.string().min(1, "Required").refine((v) => !isNaN(Number(v)) && Number(v) >= 0, "Must be a valid number"),
});

type FormFields = z.infer<typeof schema>;

export function PricingAddOnNewScreen() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", code: "", description: "", currency: "USD", billing_interval: "monthly", status: "draft", amount: "15" },
  });

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createAddOn({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name: data.name,
          code: data.code,
          description: data.description,
          currency: data.currency,
          billing_interval: data.billing_interval,
          status: data.status,
          amount_cents: Math.round(Number(data.amount) * 100),
        },
      }),
    onSuccess: (item) => {
      showSuccess("Add-on created");
      queryClient.invalidateQueries({ queryKey: ["pricing-add-ons"] });
      navigate({ to: `/pricing/add-ons/${encodeURIComponent(item.id)}` });
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create add-on", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/add-ons", label: "Add-ons" }, { label: "New" }]} />



        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create add-on</h1>
                <p className="mt-0.5 text-xs text-text-muted">Fixed recurring extras that can be attached to multiple plans.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/add-ons" })}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Add-on name" error={errors.name?.message}>
                    <Input data-testid="pricing-addon-name" placeholder="Priority support" {...register("name")} error={Boolean(errors.name)} />
                  </FormField>
                  <FormField label="Add-on code" error={errors.code?.message}>
                    <Input data-testid="pricing-addon-code" placeholder="priority_support" {...register("code")} error={Boolean(errors.code)} />
                  </FormField>
                  <FormField label="Currency" error={errors.currency?.message}>
                    <Input data-testid="pricing-addon-currency" placeholder="USD" {...register("currency")} error={Boolean(errors.currency)} />
                  </FormField>
                  <FormField label="Recurring amount" error={errors.amount?.message}>
                    <Input data-testid="pricing-addon-amount" placeholder="15" {...register("amount")} error={Boolean(errors.amount)} />
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
                      <Textarea data-testid="pricing-addon-description" placeholder="Faster response times and operator escalation support." {...register("description")} error={Boolean(errors.description)} />
                    </FormField>
                  </div>
                </div>

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/add-ons" })}>Cancel</Button>
                <Button data-testid="pricing-addon-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create add-on
                </Button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

