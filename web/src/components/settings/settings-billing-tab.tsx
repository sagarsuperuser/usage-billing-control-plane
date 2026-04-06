
import { Eye, EyeOff, LoaderCircle, RefreshCw, Zap } from "lucide-react";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import {
  fetchWorkspaceSettings, updateWorkspaceSettings,
  fetchWorkspaceBillingConnection, createWorkspaceBillingConnection, verifyWorkspaceBillingConnection,
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
      <PaymentProviderCard apiBaseURL={apiBaseURL} csrfToken={csrfToken} />

      <form onSubmit={handleSubmit((data) => saveMutation.mutate(data))}>
        <div className="divide-y divide-border">
          {/* Section: Entity & numbering */}
          <div className="px-6 py-5">
            <SectionHeader title="Entity & numbering" />
            <div className="mt-4 grid gap-4 max-w-2xl md:grid-cols-2">
              <Field label="Billing entity code" hint="e.g. ACME-US">
                <input placeholder="ACME-US" className={inputClass} {...register("billing_entity_code")} />
              </Field>
              <Field label="Document number prefix" hint="e.g. INV-">
                <input placeholder="INV-" className={inputClass} {...register("document_number_prefix")} />
              </Field>
              <Field label="Document numbering">
                <select className={inputClass} {...register("document_numbering")}>
                  <option value="">Default</option>
                  <option value="sequential">Sequential</option>
                  <option value="per_customer">Per customer</option>
                </select>
              </Field>
              <Field label="Document locale">
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
          <div className="px-6 py-5">
            <SectionHeader title="Payment terms" />
            <div className="mt-4 grid gap-4 max-w-2xl md:grid-cols-2">
              <Field label="Net payment terms" hint="Days until invoice is due.">
                <input type="number" min="0" placeholder="30" className={inputClass} {...register("net_payment_term_days")} />
              </Field>
              <Field label="Grace period" hint="Days after due date before dunning.">
                <input type="number" min="0" placeholder="3" className={inputClass} {...register("invoice_grace_period_days")} />
              </Field>
            </div>
          </div>

          {/* Section: Invoice content */}
          <div className="px-6 py-5">
            <SectionHeader title="Invoice content" />
            <div className="mt-4 grid gap-4 max-w-2xl">
              <Field label="Invoice memo">
                <textarea
                  rows={2}
                  placeholder="Thank you for your business."
                  className="rounded-lg border border-border bg-surface px-3 py-2.5 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                  {...register("invoice_memo")}
                />
              </Field>
              <Field label="Invoice footer">
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

        {/* Sticky save bar */}
        <div className="sticky bottom-0 flex items-center gap-3 border-t border-border bg-surface px-6 py-4">
          {isDirty ? <p className="flex-1 text-xs text-amber-600 dark:text-amber-400">Unsaved changes</p> : <span className="flex-1" />}
          <Button variant="primary" size="lg" type="submit" loading={busy} disabled={!isDirty}>
            Save changes
          </Button>
        </div>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Payment provider card — reads provider name from connection data
// ---------------------------------------------------------------------------

function PaymentProviderCard({ apiBaseURL, csrfToken }: { apiBaseURL: string; csrfToken: string }) {
  const queryClient = useQueryClient();
  const connQueryKey = ["workspace-billing-connection", apiBaseURL];
  const [secretKey, setSecretKey] = useState("");
  const [showKey, setShowKey] = useState(false);

  const connQuery = useQuery({
    queryKey: connQueryKey,
    queryFn: () => fetchWorkspaceBillingConnection({ runtimeBaseURL: apiBaseURL }),
    staleTime: 30_000,
  });

  const connectMutation = useMutation({
    mutationFn: () => createWorkspaceBillingConnection({ runtimeBaseURL: apiBaseURL, csrfToken, stripeSecretKey: secretKey.trim() }),
    onSuccess: () => {
      showSuccess("Payment provider connected");
      setSecretKey("");
      setShowKey(false);
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
  const providerLabel = conn?.display_name || "Stripe";

  return (
    <div className="border-b border-border px-6 py-5">
      <SectionHeader title="Payment provider" />

      {connQuery.isPending ? (
        <div className="mt-4 flex items-center gap-2 text-sm text-text-muted">
          <LoaderCircle className="h-4 w-4 animate-spin" /> Checking connection...
        </div>
      ) : isConnected && conn ? (
        <div className="mt-4 flex items-center gap-4 max-w-2xl">
          <div className="flex items-center gap-2.5 flex-1 min-w-0">
            <span className="inline-block h-2 w-2 shrink-0 rounded-full bg-emerald-500" />
            <span className="text-sm font-medium text-text-primary">{providerLabel}</span>
            <span className="rounded-full border border-border px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-text-muted">{conn.environment}</span>
            {conn.connected_at ? (
              <span className="text-xs text-text-faint">
                Connected {new Date(conn.connected_at).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
              </span>
            ) : null}
          </div>
          <Button variant="secondary" size="sm" type="button" onClick={() => verifyMutation.mutate()} loading={verifyMutation.isPending}>
            {!verifyMutation.isPending ? <RefreshCw className="h-3 w-3" /> : null}
            Verify
          </Button>
        </div>
      ) : (
        <div className="mt-4 max-w-2xl">
          {isFailed && conn ? (
            <div className="mb-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300">
              {conn.last_sync_error || "API key could not be verified."}
            </div>
          ) : null}
          <div className="flex items-end gap-2">
            <label className="grid flex-1 gap-1.5">
              <span className="text-xs font-medium text-text-muted">Stripe secret key</span>
              <div className="relative">
                <input
                  type={showKey ? "text" : "password"}
                  value={secretKey}
                  onChange={(e) => setSecretKey(e.target.value)}
                  placeholder="sk_test_... or sk_live_..."
                  className="h-10 w-full rounded-lg border border-border bg-surface pl-3 pr-10 font-mono text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                />
                <button
                  type="button"
                  onClick={() => setShowKey(!showKey)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-text-faint transition hover:text-text-muted"
                  tabIndex={-1}
                >
                  {showKey ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                </button>
              </div>
            </label>
            <Button
              variant="primary"
              size="lg"
              type="button"
              onClick={() => connectMutation.mutate()}
              disabled={!secretKey.trim().startsWith("sk_")}
              loading={connectMutation.isPending}
            >
              {!connectMutation.isPending ? <Zap className="h-3.5 w-3.5" /> : null}
              Connect
            </Button>
          </div>
          <p className="mt-2 text-[11px] text-text-faint">
            Find your key in the{" "}
            <a href="https://dashboard.stripe.com/apikeys" target="_blank" rel="noreferrer" className="underline hover:text-text-muted">Stripe Dashboard</a>.
            Encrypted and stored securely.
          </p>
        </div>
      )}
    </div>
  );
}

function SectionHeader({ title }: { title: string }) {
  return <h3 className="text-sm font-semibold text-text-primary">{title}</h3>;
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-1.5">
      <span className="text-xs font-medium text-text-muted">
        {label}
        {hint ? <span className="ml-1.5 font-normal text-text-faint">{hint}</span> : null}
      </span>
      {children}
    </label>
  );
}
