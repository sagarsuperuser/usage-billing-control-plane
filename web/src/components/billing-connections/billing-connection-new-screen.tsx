"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import type { InputHTMLAttributes, SelectHTMLAttributes } from "react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createBillingProviderConnection, refreshBillingProviderConnection } from "@/lib/api";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

const schema = z.object({
  display_name: z.string().min(1, "Required"),
  environment: z.enum(["test", "live"]),
  stripe_secret_key: z.string().min(1, "Required"),
});

type FormFields = z.infer<typeof schema>;

export function BillingConnectionNewScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const canViewPlatformSurface = isAuthenticated && scope === "platform" && isPlatformAdmin;

  const {
    register,
    handleSubmit,
    watch,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: { display_name: "", environment: "test", stripe_secret_key: "" },
  });

  const watched = watch();

  const createMutation = useMutation({
    mutationFn: async (data: FormFields) => {
      const created = await createBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          provider_type: "stripe",
          environment: data.environment,
          display_name: data.display_name.trim(),
          scope: "platform",
          stripe_secret_key: data.stripe_secret_key.trim(),
        },
      });
      return refreshBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        connectionID: created.id,
      });
    },
    onSuccess: async (connection) => {
      await queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] });
      router.push(`/billing-connections/${encodeURIComponent(connection.id)}`);
    },
    onError: (err: Error) => {
      setError("root", { message: err.message });
      showError("Failed to create connection", err.message);
    },
  });

  const onSubmit = handleSubmit((data) => createMutation.mutate(data));
  const busy = isSubmitting || createMutation.isPending;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/billing-connections", label: "Billing Connections" }, { label: "New" }]} />

        {canViewPlatformSurface ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Billing connection</p>
                <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">New billing connection</h1>
                <p className="mt-3 max-w-3xl text-sm text-slate-600">
                  Create a platform-owned Stripe connection. Alpha stores the secret, checks the Stripe credentials, and makes the connection available for workspace use.
                </p>
              </div>
              <Link href="/billing-connections" className="inline-flex h-10 items-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100">
                Back to billing connections
              </Link>
            </div>
          </section>
        ) : null}

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are managed at the platform layer. Sign in with a platform account to create them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {canViewPlatformSurface ? (
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
            <form onSubmit={onSubmit} noValidate>
              <section className="min-w-0 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0">
                    <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Operator setup</p>
                    <h2 className="mt-2 text-xl font-semibold text-slate-950">Connect Stripe</h2>
                    <p className="mt-2 max-w-2xl text-sm text-slate-600">
                      Add the Stripe secret and let Alpha check the connection before any workspace uses it.
                    </p>
                  </div>
                  <span className="inline-flex rounded-lg border border-slate-200 bg-slate-50 p-3 text-slate-700">
                    <ShieldCheck className="h-5 w-5" />
                  </span>
                </div>

                <div className="mt-5 grid gap-3 lg:grid-cols-3">
                  <OperatorLine title="Connection record" body="Use a durable display name your operators will recognize later." />
                  <OperatorLine title="Secret handling" body="Alpha stores the Stripe secret and keeps it outside normal workspace records." />
                  <OperatorLine title="Verification path" body="Create the record first, then let Alpha run an explicit provider check before workspace assignment." />
                </div>

                <div className="mt-5 grid gap-5">
                  <section className="rounded-xl border border-slate-200 bg-slate-50 p-5">
                    <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Connection record</p>
                    <h3 className="mt-2 text-lg font-semibold text-slate-950">Connection details</h3>
                    <div className="mt-4 grid gap-4 md:grid-cols-2">
                      <Field label="Connection name" placeholder="Stripe Sandbox" error={errors.display_name?.message} {...register("display_name")} />
                      <SelectField label="Environment" options={["test", "live"]} error={errors.environment?.message} {...register("environment")} />
                      <Field label="Stripe secret key" placeholder="sk_test_..." type="password" error={errors.stripe_secret_key?.message} {...register("stripe_secret_key")} />
                    </div>
                  </section>
                </div>

                <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
                  <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
                  <div className="mt-3 grid gap-2 md:grid-cols-2">
                    <ChecklistLine done={(watched.display_name ?? "").trim().length > 0} text="Connection name is set" />
                    <ChecklistLine done={(watched.stripe_secret_key ?? "").trim().length > 0} text="Stripe secret key is set" />
                    <ChecklistLine done text="Provider type is Stripe" />
                    <ChecklistLine done text="Alpha will check the Stripe connection after create" />
                  </div>
                </div>

                {errors.root?.message ? (
                  <p className="mt-5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p>
                ) : null}

                <div className="mt-6 flex flex-wrap gap-3">
                  <button
                    type="submit"
                    disabled={busy || !isPlatformAdmin || !csrfToken}
                    className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                    Create and check connection
                  </button>
                </div>
              </section>
            </form>

            <aside className="min-w-0 grid gap-5 self-start">
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Alpha posture</p>
                <div className="mt-4 grid gap-3">
                  <OperatorSideCard title="Credential scope" body="Secret stays out of workspace rows and remains platform-owned." />
                  <OperatorSideCard title="Verification scope" body="Provider check is explicit and observable before workspace use." />
                  <OperatorSideCard title="Routing scope" body="Workspace assignment uses the stable connection record while internal billing routing stays hidden." />
                </div>
              </section>
              <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm text-sm text-slate-600">
                <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">After connect</p>
                <p className="mt-3">Rotate the secret, refresh the connection, or disable the connection later from the detail page.</p>
              </section>
            </aside>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function OperatorLine({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}

function OperatorSideCard({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
      <p className="text-sm font-semibold text-slate-950">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-600">{body}</p>
    </div>
  );
}

function Field({ label, error, ...inputProps }: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        {...inputProps}
        aria-invalid={Boolean(error)}
        className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-slate-200"}`}
      />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <select
        {...selectProps}
        aria-invalid={Boolean(error)}
        className={`h-10 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-slate-200"}`}
      >
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
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
