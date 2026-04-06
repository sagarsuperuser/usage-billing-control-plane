import { Link, useNavigate } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
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
              <Link to="/pricing/add-ons" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
            </div>
            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <div className="grid gap-4 md:grid-cols-2">
                  <Field label="Add-on name" placeholder="Priority support" testID="pricing-addon-name" error={errors.name?.message} {...register("name")} />
                  <Field label="Add-on code" placeholder="priority_support" testID="pricing-addon-code" error={errors.code?.message} {...register("code")} />
                  <Field label="Currency" placeholder="USD" testID="pricing-addon-currency" error={errors.currency?.message} {...register("currency")} />
                  <Field label="Recurring amount" placeholder="15" testID="pricing-addon-amount" error={errors.amount?.message} {...register("amount")} />
                  <SelectField label="Billing interval" options={["monthly", "yearly"]} error={errors.billing_interval?.message} {...register("billing_interval")} />
                  <SelectField label="Status" options={["draft", "active", "archived"]} error={errors.status?.message} {...register("status")} />
                  <div className="md:col-span-2">
                    <TextareaField label="Description" placeholder="Faster response times and operator escalation support." testID="pricing-addon-description" error={errors.description?.message} {...register("description")} />
                  </div>
                </div>

                {errors.root?.message ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}
              </div>
              <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
                <Link to="/pricing/add-ons" className="inline-flex h-10 items-center rounded-lg border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:bg-surface-tertiary">Cancel</Link>
                <button data-testid="pricing-addon-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                  Create add-on
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-border"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <select {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-border"}`}>
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function TextareaField({ label, error, testID, ...textareaProps }: { label: string; error?: string; testID?: string } & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <label className="grid gap-2 text-sm text-text-secondary">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <textarea data-testid={testID} {...textareaProps} aria-invalid={Boolean(error)} className={`min-h-[120px] rounded-lg border bg-surface px-3 py-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-border"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
