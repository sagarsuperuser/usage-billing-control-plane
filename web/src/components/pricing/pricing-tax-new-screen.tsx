"use client";

import Link from "next/link";
import { LoaderCircle } from "lucide-react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createTax } from "@/lib/api";
import { showError } from "@/lib/toast";
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
  const router = useRouter();
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
    defaultValues: { name: "", code: "", description: "", status: "active", rate: "18" },
  });

  const watched = watch();

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
    onSuccess: (tax) => router.push("/pricing/taxes/" + encodeURIComponent(tax.id)),
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create tax", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => mutation.mutate(data));
  const busy = isSubmitting || mutation.isPending;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1200px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/taxes", label: "Taxes" }, { label: "New" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Taxes are workspace-scoped. Sign in with a workspace account to create one." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {isTenantSession ? (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace operator flow</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Create tax</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">Define a reusable tax code and rate that Alpha can assign to customer billing profiles and workspace billing settings.</p>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <form onSubmit={onSubmit} noValidate>
                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <div className="grid gap-5">
                    <div className="grid gap-3 lg:grid-cols-3">
                      <OperatorCard title="Assignment" body="Active taxes become available to customer billing profiles and workspace billing settings." />
                      <OperatorCard title="Stable codes" body="Treat the code as reusable configuration. Change rates deliberately so invoice behavior stays explainable." />
                      <OperatorCard title="After create" body="Use tax detail and customer billing settings to confirm where the rule is applied." />
                    </div>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Tax rule</p>
                      <h2 className="text-lg font-semibold text-slate-950">Tax basics</h2>
                      <div className="mt-4 grid gap-4 md:grid-cols-2">
                        <Field label="Tax name" placeholder="India GST 18" testID="pricing-tax-name" error={errors.name?.message} {...register("name")} />
                        <Field label="Tax code" placeholder="gst_in_18" testID="pricing-tax-code" error={errors.code?.message} {...register("code")} />
                        <SelectField label="Status" options={["active", "draft", "archived"]} error={errors.status?.message} {...register("status")} />
                        <Field label="Rate (%)" placeholder="18" testID="pricing-tax-rate" error={errors.rate?.message} {...register("rate")} />
                        <div className="md:col-span-2">
                          <TextareaField label="Description" placeholder="Applied to domestic B2C sales." testID="pricing-tax-description" error={errors.description?.message} {...register("description")} />
                        </div>
                      </div>
                    </section>

                    <section className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                      <div className="mt-3 grid gap-2 md:grid-cols-2">
                        <ChecklistLine done={(watched.name ?? "").trim().length > 0} text="Tax name is set" />
                        <ChecklistLine done={(watched.code ?? "").trim().length > 0} text="Tax code is set" />
                        <ChecklistLine done={(watched.rate ?? "").trim().length > 0} text="Tax rate is set" />
                        <ChecklistLine done={Boolean(csrfToken)} text="Writable workspace session present" />
                      </div>
                    </section>

                    {errors.root?.message ? <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p> : null}

                    <div className="flex flex-wrap gap-3">
                      <button data-testid="pricing-tax-submit" type="submit" disabled={busy || !csrfToken} className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50">
                        {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                        Create tax
                      </button>
                      <Link href="/pricing/taxes" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">Cancel</Link>
                    </div>
                  </div>
                </section>
              </form>

              <aside className="grid gap-5 self-start">
                <InfoCard title="Operator guidance" body="This screen creates the reusable tax rule only. Apply it later through customer and workspace billing settings." />
                <InfoCard title="Use stable codes" body="Treat tax codes like reusable configuration. Change rates or descriptions deliberately so invoice behavior stays explainable." />
              </aside>
            </div>
          </>
        ) : null}
      </main>
    </div>
  );
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-xl border border-slate-200 bg-slate-50 p-4"><p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}

function Field({ label, error, testID, ...inputProps }: { label: string; error?: string; testID?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input data-testid={testID} {...inputProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select {...selectProps} aria-invalid={Boolean(error)} className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-slate-200"}`}>
        {options.map((option) => <option key={option} value={option}>{option.replace(/_/g, " ")}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function TextareaField({ label, error, testID, ...textareaProps }: { label: string; error?: string; testID?: string } & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <textarea data-testid={testID} {...textareaProps} aria-invalid={Boolean(error)} className={`min-h-[120px] rounded-lg border bg-white px-3 py-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`} />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function InfoCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-lg border border-slate-200 bg-white px-3 py-3">
      <span className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-100 text-emerald-700" : "bg-amber-100 text-amber-700"}`}>
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-800">{text}</p>
    </div>
  );
}
