import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Input, Select, Textarea } from "@/components/ui/input";
import { createTax } from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  name: z.string().min(1, "Required"),
  code: z.string().min(1, "Required"),
  description: z.string(),
  status: z.enum(["active", "draft", "archived"]),
  rate: z.string().min(1, "Required").refine((v) => !isNaN(Number(v)) && Number(v) >= 0, "Must be a valid number"),
});

type FormFields = z.infer<typeof schema>;

export function PricingTaxNewScreen() {
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
    defaultValues: { name: "", code: "", description: "", status: "active", rate: "18" },
  });

  const mutation = useMutation({
    mutationFn: (data: FormFields) =>
      createTax({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          name: data.name,
          code: data.code,
          description: data.description,
          status: data.status,
          rate: Number(data.rate),
        },
      }),
    onSuccess: (tax) => {
      showSuccess("Tax created");
      queryClient.invalidateQueries({ queryKey: ["pricing-taxes"] });
      navigate({ to: "/pricing/taxes/" + encodeURIComponent(tax.id) });
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create tax", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/taxes", label: "Taxes" }, { label: "New" }]} />



        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex items-center justify-between border-b border-border px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-text-primary">Create tax</h1>
                <p className="mt-0.5 text-xs text-text-muted">Reusable tax code and rate for customer billing profiles.</p>
              </div>
              <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/taxes" })}>Cancel</Button>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <Field label="Tax name" placeholder="India GST 18" testID="pricing-tax-name" error={errors.name?.message} {...register("name")} />
                  <Field label="Tax code" placeholder="gst_in_18" testID="pricing-tax-code" error={errors.code?.message} {...register("code")} />
                  <SelectField label="Status" options={["active", "draft", "archived"]} error={errors.status?.message} {...register("status")} />
                  <Field label="Rate (%)" placeholder="18" testID="pricing-tax-rate" error={errors.rate?.message} {...register("rate")} />
                  <div className="md:col-span-2">
                    <TextareaField label="Description" placeholder="Applied to domestic B2C sales." testID="pricing-tax-description" error={errors.description?.message} {...register("description")} />
                  </div>
                </div>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/taxes" })}>Cancel</Button>
                <Button data-testid="pricing-tax-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create tax
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
