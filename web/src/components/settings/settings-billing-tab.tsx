
import { CheckCircle2, LoaderCircle, RefreshCw, Zap } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  fetchWorkspaceSettings, updateWorkspaceSettings,
  fetchWorkspaceBillingConnection, createWorkspaceBillingConnection, verifyWorkspaceBillingConnection,
  type BillingConnectionResponse,
} from "@/lib/api";
import { showError, showSuccess } from "@/lib/toast";

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
    onSuccess: () => {
      showSuccess("Billing settings saved");
      queryClient.invalidateQueries({ queryKey });
    },
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
    <div>
      <StripeConnectionCard apiBaseURL={apiBaseURL} csrfToken={csrfToken} />

      <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))}>
        <div className="divide-y divide-border">
          {/* Section: Entity & numbering */}
        <div className="p-6">
          <SectionHeader title="Entity & numbering" description="Identify your legal entity and control how invoices are numbered." />
          <div className="mt-4 grid gap-4 max-w-2xl md:grid-cols-2">
            <Field label="Billing entity code" hint="Identifier for your legal entity on invoices.">
              <input placeholder="ACME-US" className={inputClass} {...register("billing_entity_code")} />
            </Field>
            <Field label="Document number prefix" hint="Prepended to invoice numbers, e.g. INV-">
              <input placeholder="INV-" className={inputClass} {...register("document_number_prefix")} />
            </Field>
            <Field label="Document numbering" hint="Numbering scheme for invoices.">
              <select className={inputClass} {...register("document_numbering")}>
                <option value="">Default</option>
                <option value="sequential">Sequential</option>
                <option value="per_customer">Per customer</option>
              </select>
            </Field>
            <Field label="Document locale" hint="Language for invoice text.">
              <select className={inputClass} {...register("document_locale")}>
                <option value="">Default (en)</option>
                <option value="en">English</option>
                <option value="fr">French</option>
                <option value="de">German</option>
                <option value="es">Spanish</option>
              </select>
            </Field>
          </div>
        </div>

        {/* Section: Payment terms */}
        <div className="p-6">
          <SectionHeader title="Payment terms" description="Control when invoices are due and when dunning begins." />
          <div className="mt-4 grid gap-4 max-w-2xl md:grid-cols-2">
            <Field label="Net payment terms (days)" hint="Days after issue before the invoice is due.">
              <input type="number" min="0" placeholder="30" className={inputClass} {...register("net_payment_term_days")} />
            </Field>
            <Field label="Grace period (days)" hint="Days after due date before dunning starts.">
              <input type="number" min="0" placeholder="3" className={inputClass} {...register("invoice_grace_period_days")} />
            </Field>
          </div>
        </div>

        {/* Section: Invoice content */}
        <div className="p-6">
          <SectionHeader title="Invoice content" description="Custom text that appears on every invoice." />
          <div className="mt-4 grid gap-4 max-w-2xl">
            <Field label="Invoice memo" hint="Shown in the invoice body, above line items.">
              <textarea
                rows={2}
                placeholder="Thank you for your business."
                className="rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                {...register("invoice_memo")}
              />
            </Field>
            <Field label="Invoice footer" hint="Shown at the bottom of every invoice.">
              <textarea
                rows={2}
                placeholder="Acme Inc. | 123 Main St | support@acme.com"
                className="rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                {...register("invoice_footer")}
              />
            </Field>
          </div>
        </div>
      </div>

      {/* Save bar — always visible, disabled when clean */}
      <div className="flex items-center gap-3 border-t border-border px-6 py-4">
        {isDirty ? <p className="flex-1 text-xs text-amber-600 dark:text-amber-400">Unsaved changes</p> : <span className="flex-1" />}
        <button
          type="submit"
          disabled={!isDirty || busy}
          className="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
        >
          {busy ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
          Save changes
        </button>
      </div>
    </form>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Stripe connection card
// ---------------------------------------------------------------------------

function StripeConnectionCard({ apiBaseURL, csrfToken }: { apiBaseURL: string; csrfToken: string }) {
  const queryClient = useQueryClient();
  const connQueryKey = ["workspace-billing-connection", apiBaseURL];
  const [secretKey, setSecretKey] = useState("");

  const connQuery = useQuery({
    queryKey: connQueryKey,
    queryFn: () => fetchWorkspaceBillingConnection({ runtimeBaseURL: apiBaseURL }),
    staleTime: 30_000,
  });

  const connectMutation = useMutation({
    mutationFn: () => createWorkspaceBillingConnection({ runtimeBaseURL: apiBaseURL, csrfToken, stripeSecretKey: secretKey.trim() }),
    onSuccess: () => {
      showSuccess("Stripe connected");
      setSecretKey("");
      queryClient.invalidateQueries({ queryKey: connQueryKey });
    },
    onError: (err: Error) => showError("Connection failed", err.message),
  });

  const verifyMutation = useMutation({
    mutationFn: () => verifyWorkspaceBillingConnection({ runtimeBaseURL: apiBaseURL, csrfToken }),
    onSuccess: () => {
      showSuccess("Connection verified");
      queryClient.invalidateQueries({ queryKey: connQueryKey });
    },
    onError: (err: Error) => showError("Verification failed", err.message),
  });

  const conn = connQuery.data;
  const isConnected = conn?.status === "connected";
  const isFailed = conn?.status === "sync_error";

  return (
    <div className="border-b border-border p-6">
      <SectionHeader title="Stripe connection" description="Connect your Stripe account to process payments and manage subscriptions." />

      {connQuery.isPending ? (
        <div className="mt-4 flex items-center gap-2 text-sm text-text-muted">
          <LoaderCircle className="h-4 w-4 animate-spin" /> Checking connection...
        </div>
      ) : isConnected && conn ? (
        <div className="mt-4 max-w-2xl">
          <div className="flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 dark:border-emerald-900 dark:bg-emerald-950">
            <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-emerald-800 dark:text-emerald-200">Stripe connected</p>
              <p className="mt-0.5 text-xs text-emerald-700 dark:text-emerald-300">
                Environment: <span className="font-medium">{conn.environment}</span>
                {conn.connected_at ? ` · Connected ${new Date(conn.connected_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}` : ""}
              </p>
            </div>
            <button
              type="button"
              onClick={() => verifyMutation.mutate()}
              disabled={verifyMutation.isPending}
              className="inline-flex h-8 items-center gap-1.5 rounded-md border border-emerald-200 bg-white px-3 text-xs font-medium text-emerald-700 transition hover:bg-emerald-50 disabled:opacity-50 dark:border-emerald-800 dark:bg-emerald-900 dark:text-emerald-200"
            >
              {verifyMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3" />}
              Verify
            </button>
          </div>
        </div>
      ) : (
        <div className="mt-4 max-w-2xl">
          {isFailed && conn ? (
            <div className="mb-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300">
              Connection failed: {conn.last_sync_error || "Stripe API key could not be verified."}
            </div>
          ) : null}
          <div className="flex items-end gap-2">
            <label className="grid flex-1 gap-1.5">
              <span className="text-xs font-medium text-text-muted">Stripe secret key</span>
              <input
                type="password"
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                placeholder="sk_test_... or sk_live_..."
                className="h-10 rounded-lg border border-border bg-surface px-3 font-mono text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              />
            </label>
            <button
              type="button"
              onClick={() => connectMutation.mutate()}
              disabled={connectMutation.isPending || !secretKey.trim().startsWith("sk_")}
              className="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
            >
              {connectMutation.isPending ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Zap className="h-3.5 w-3.5" />}
              Connect
            </button>
          </div>
          <p className="mt-2 text-[11px] text-text-faint">
            Find your API key in the <a href="https://dashboard.stripe.com/apikeys" target="_blank" rel="noreferrer" className="underline hover:text-text-muted">Stripe Dashboard</a>. Your key is encrypted and stored securely.
          </p>
        </div>
      )}
    </div>
  );
}

function SectionHeader({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-text-primary">{title}</h3>
      <p className="mt-0.5 text-xs text-text-muted">{description}</p>
    </div>
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
