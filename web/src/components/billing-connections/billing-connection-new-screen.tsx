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
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormFields>({
    resolver: zodResolver(schema),
    defaultValues: { display_name: "", environment: "test", stripe_secret_key: "" },
  });

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
      <main className="mx-auto flex max-w-[760px] flex-col gap-5 px-4 py-6 md:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Billing connections" }, { label: "New" }]} />

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
          <div className="overflow-hidden rounded-xl border border-stone-200 bg-white shadow-sm">
            <div className="flex items-center justify-between border-b border-stone-200 px-6 py-4">
              <div>
                <h1 className="text-base font-semibold text-slate-900">New billing connection</h1>
                <p className="mt-0.5 text-xs text-slate-500">Stripe · Platform-owned · Alpha checks credentials after create</p>
              </div>
              <Link
                href="/billing-connections"
                className="inline-flex h-8 items-center rounded-lg border border-stone-200 px-3 text-sm text-slate-600 transition hover:bg-stone-100"
              >
                Cancel
              </Link>
            </div>

            <form onSubmit={onSubmit} noValidate>
              <div className="grid gap-4 p-6">
                <Field label="Connection name" placeholder="e.g. Stripe Sandbox" error={errors.display_name?.message} {...register("display_name")} />
                <div className="grid gap-4 sm:grid-cols-2">
                  <SelectField label="Environment" options={["test", "live"]} error={errors.environment?.message} {...register("environment")} />
                  <Field label="Stripe secret key" placeholder="sk_test_..." type="password" error={errors.stripe_secret_key?.message} {...register("stripe_secret_key")} />
                </div>
                {errors.root?.message ? (
                  <p className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{errors.root.message}</p>
                ) : null}
              </div>

              <div className="flex justify-end gap-2 border-t border-stone-200 px-6 py-4">
                <Link
                  href="/billing-connections"
                  className="inline-flex h-9 items-center rounded-lg border border-stone-200 px-4 text-sm text-slate-700 transition hover:bg-stone-100"
                >
                  Cancel
                </Link>
                <button
                  type="submit"
                  disabled={busy || !isPlatformAdmin || !csrfToken}
                  className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                  Create and check connection
                </button>
              </div>
            </form>
          </div>
        ) : null}
      </main>
    </div>
  );
}

function Field({ label, error, ...inputProps }: { label: string; error?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="grid gap-1.5 text-sm text-slate-700">
      <span className="text-xs font-medium text-slate-700">{label}</span>
      <input
        {...inputProps}
        aria-invalid={Boolean(error)}
        className={`h-9 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2 ${error ? "border-rose-300 focus:ring-rose-200" : "border-stone-200"}`}
      />
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

function SelectField({ label, error, options, ...selectProps }: { label: string; error?: string; options: string[] } & SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <label className="grid gap-1.5 text-sm text-slate-700">
      <span className="text-xs font-medium text-slate-700">{label}</span>
      <select
        {...selectProps}
        aria-invalid={Boolean(error)}
        className={`h-9 rounded-lg border bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2 ${error ? "border-rose-300" : "border-stone-200"}`}
      >
        {options.map((option) => <option key={option} value={option}>{option[0].toUpperCase() + option.slice(1)}</option>)}
      </select>
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}
