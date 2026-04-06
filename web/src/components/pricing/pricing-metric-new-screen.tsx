import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { FormField } from "@/components/ui/form-field";
import { Input, Select } from "@/components/ui/input";
import { createPricingMetric } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  name: z.string().min(1, "Required"),
  key: z.string().min(1, "Required"),
  unit: z.string().min(1, "Required"),
  aggregation: z.enum(["sum", "count", "max"]),
  currency: z.string().min(1, "Required"),
});

type FormFields = z.infer<typeof schema>;

export function PricingMetricNewScreen() {
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
    defaultValues: { name: "", key: "", unit: "request", aggregation: "sum", currency: "USD" },
  });

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createPricingMetric({ runtimeBaseURL: apiBaseURL, csrfToken, body: data }),
    onSuccess: (metric) => {
      showSuccess("Metric created");
      queryClient.invalidateQueries({ queryKey: ["pricing-metrics"] });
      navigate({ to: `/pricing/metrics/${encodeURIComponent(metric.id)}` });
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create metric", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-[1120px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: "New" }]} />



        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create metric</h1>
                <p className="mt-0.5 text-xs text-text-muted">Define the usage record plans will price against.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/metrics" })}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField label="Metric name" error={errors.name?.message}>
                    <Input data-testid="pricing-metric-name" placeholder="API Calls" {...register("name")} error={Boolean(errors.name)} />
                  </FormField>
                  <FormField label="Metric code" error={errors.key?.message}>
                    <Input data-testid="pricing-metric-code" placeholder="api_calls" {...register("key")} error={Boolean(errors.key)} />
                  </FormField>
                  <FormField label="Unit" error={errors.unit?.message}>
                    <Input data-testid="pricing-metric-unit" placeholder="request" {...register("unit")} error={Boolean(errors.unit)} />
                  </FormField>
                  <FormField label="Aggregation" error={errors.aggregation?.message}>
                    <Select {...register("aggregation")} error={Boolean(errors.aggregation)}>
                      {["sum", "count", "max"].map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
                    </Select>
                  </FormField>
                  <FormField label="Currency" error={errors.currency?.message}>
                    <Input data-testid="pricing-metric-currency" placeholder="USD" {...register("currency")} error={Boolean(errors.currency)} />
                  </FormField>
                </div>

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/metrics" })}>Cancel</Button>
                <Button data-testid="pricing-metric-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create metric
                </Button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

