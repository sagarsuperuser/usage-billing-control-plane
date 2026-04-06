import { useNavigate } from "@tanstack/react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { FormField } from "@/components/ui/form-field";
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
    <PageContainer>
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/taxes", label: "Taxes" }, { label: "New" }]} />



        {isTenantSession ? (
          <Card>
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
                  <FormField label="Tax name" error={errors.name?.message}>
                    <Input data-testid="pricing-tax-name" placeholder="India GST 18" {...register("name")} error={Boolean(errors.name)} />
                  </FormField>
                  <FormField label="Tax code" error={errors.code?.message}>
                    <Input data-testid="pricing-tax-code" placeholder="gst_in_18" {...register("code")} error={Boolean(errors.code)} />
                  </FormField>
                  <FormField label="Status" error={errors.status?.message}>
                    <Select {...register("status")} error={Boolean(errors.status)}>
                      {["active", "draft", "archived"].map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
                    </Select>
                  </FormField>
                  <FormField label="Rate (%)" error={errors.rate?.message}>
                    <Input data-testid="pricing-tax-rate" placeholder="18" {...register("rate")} error={Boolean(errors.rate)} />
                  </FormField>
                  <div className="md:col-span-2">
                    <FormField label="Description" error={errors.description?.message}>
                      <Textarea data-testid="pricing-tax-description" placeholder="Applied to domestic B2C sales." {...register("description")} error={Boolean(errors.description)} />
                    </FormField>
                  </div>
                </div>

                {errors.root?.message ? <Alert tone="danger">{errors.root.message}</Alert> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Button variant="secondary" size="lg" onClick={() => navigate({ to: "/pricing/taxes" })}>Cancel</Button>
                <Button data-testid="pricing-tax-submit" variant="primary" size="lg" type="submit" loading={busy} disabled={!csrfToken}>
                  Create tax
                </Button>
              </div>
            </form>
          </Card>
        ) : null}
    </PageContainer>
  );
}

