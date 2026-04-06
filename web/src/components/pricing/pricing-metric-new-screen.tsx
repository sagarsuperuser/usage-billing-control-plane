import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
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
                  <Field label="Metric name" placeholder="API Calls" testID="pricing-metric-name" error={errors.name?.message} {...register("name")} />
                  <Field label="Metric code" placeholder="api_calls" testID="pricing-metric-code" error={errors.key?.message} {...register("key")} />
                  <Field label="Unit" placeholder="request" testID="pricing-metric-unit" error={errors.unit?.message} {...register("unit")} />
                  <SelectField label="Aggregation" options={["sum", "count", "max"]} error={errors.aggregation?.message} {...register("aggregation")} />
                  <Field label="Currency" placeholder="USD" testID="pricing-metric-currency" error={errors.currency?.message} {...register("currency")} />
                </div>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
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

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <Input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} error={Boolean(error)} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <Select {...selectProps} aria-invalid={Boolean(error)} error={Boolean(error)}>
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </Select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
