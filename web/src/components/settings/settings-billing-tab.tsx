
import { CheckCircle2, LoaderCircle } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchWorkspaceSettings, updateWorkspaceSettings } from "@/lib/api";
import { showError } from "@/lib/toast";

interface BillingFields {
  billing_entity_code: string;
  net_payment_term_days: string;
  invoice_grace_period_days: string;
  document_number_prefix: string;
  document_numbering: string;
  document_locale: string;
  invoice_memo: string;
  invoice_footer: string;
}

const inputClass =
  "h-10 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2";

export function SettingsBillingTab({
  apiBaseURL,
  csrfToken,
}: {
  apiBaseURL: string;
  csrfToken: string;
}) {
  const queryClient = useQueryClient();
  const queryKey = ["workspace-settings", apiBaseURL];

  const settingsQuery = useQuery({
    queryKey,
    queryFn: () => fetchWorkspaceSettings({ runtimeBaseURL: apiBaseURL }),
    staleTime: 30_000,
  });

  const billing = settingsQuery.data?.billing_settings;

  const {
    register,
    handleSubmit,
    reset,
    formState: { isDirty, isSubmitting },
  } = useForm<BillingFields>({
    defaultValues: {
      billing_entity_code: "",
      net_payment_term_days: "",
      invoice_grace_period_days: "",
      document_number_prefix: "",
      document_numbering: "",
      document_locale: "",
      invoice_memo: "",
      invoice_footer: "",
    },
  });

  useEffect(() => {
    if (billing) {
      reset({
        billing_entity_code: billing.billing_entity_code ?? "",
        net_payment_term_days: billing.net_payment_term_days?.toString() ?? "",
        invoice_grace_period_days: billing.invoice_grace_period_days?.toString() ?? "",
        document_number_prefix: billing.document_number_prefix ?? "",
        document_numbering: billing.document_numbering ?? "",
        document_locale: billing.document_locale ?? "",
        invoice_memo: billing.invoice_memo ?? "",
        invoice_footer: billing.invoice_footer ?? "",
      });
    }
  }, [billing, reset]);

  const saveMutation = useMutation({
    mutationFn: (data: BillingFields) => {
      const body: Record<string, unknown> = {
        billing_entity_code: data.billing_entity_code || undefined,
        document_number_prefix: data.document_number_prefix || undefined,
        document_numbering: data.document_numbering || undefined,
        document_locale: data.document_locale || undefined,
        invoice_memo: data.invoice_memo || undefined,
        invoice_footer: data.invoice_footer || undefined,
      };
      if (data.net_payment_term_days) body.net_payment_term_days = parseInt(data.net_payment_term_days, 10);
      if (data.invoice_grace_period_days) body.invoice_grace_period_days = parseInt(data.invoice_grace_period_days, 10);
      return updateWorkspaceSettings({ runtimeBaseURL: apiBaseURL, csrfToken, body });
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
    onError: (err: Error) => showError(err.message),
  });

  const busy = isSubmitting || saveMutation.isPending;

  if (settingsQuery.isPending) {
    return (
      <div className="flex items-center justify-center py-16">
        <LoaderCircle className="h-5 w-5 animate-spin text-text-muted" />
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))} className="p-6">
      <div className="max-w-2xl space-y-6">
        <div>
          <h3 className="text-sm font-semibold text-text-primary">Invoice & billing</h3>
          <p className="mt-0.5 text-xs text-text-muted">Configure how invoices are generated and numbered.</p>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Billing entity code" hint="Identifier for your legal entity on invoices.">
            <input placeholder="ACME-US" className={inputClass} {...register("billing_entity_code")} />
          </Field>
          <Field label="Document number prefix" hint="Prepended to invoice numbers. e.g. INV-">
            <input placeholder="INV-" className={inputClass} {...register("document_number_prefix")} />
          </Field>
          <Field label="Net payment terms (days)" hint="Days after issue before invoice is due.">
            <input type="number" min="0" placeholder="30" className={inputClass} {...register("net_payment_term_days")} />
          </Field>
          <Field label="Grace period (days)" hint="Days after due date before dunning begins.">
            <input type="number" min="0" placeholder="3" className={inputClass} {...register("invoice_grace_period_days")} />
          </Field>
          <Field label="Document numbering" hint="Numbering scheme: per_customer or sequential.">
            <select className={inputClass} {...register("document_numbering")}>
              <option value="">Default</option>
              <option value="sequential">Sequential</option>
              <option value="per_customer">Per customer</option>
            </select>
          </Field>
          <Field label="Document locale" hint="Language for invoice generation.">
            <select className={inputClass} {...register("document_locale")}>
              <option value="">Default (en)</option>
              <option value="en">English</option>
              <option value="fr">French</option>
              <option value="de">German</option>
              <option value="es">Spanish</option>
            </select>
          </Field>
        </div>

        <div className="grid gap-4">
          <Field label="Invoice memo" hint="Custom text shown on the invoice body.">
            <textarea
              rows={2}
              placeholder="Thank you for your business."
              className="rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              {...register("invoice_memo")}
            />
          </Field>
          <Field label="Invoice footer" hint="Text at the bottom of every invoice.">
            <textarea
              rows={2}
              placeholder="Acme Inc. | 123 Main St | support@acme.com"
              className="rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              {...register("invoice_footer")}
            />
          </Field>
        </div>

        {saveMutation.isSuccess ? (
          <p className="flex items-center gap-1.5 text-xs text-emerald-600">
            <CheckCircle2 className="h-3.5 w-3.5" /> Billing settings saved
          </p>
        ) : null}

        <div className="flex gap-2 border-t border-border pt-4">
          <button
            type="submit"
            disabled={!isDirty || busy}
            className="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
          >
            {busy ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
            Save changes
          </button>
        </div>
      </div>
    </form>
  );
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-1.5">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      {children}
      {hint ? <span className="text-[11px] text-text-faint">{hint}</span> : null}
    </label>
  );
}
