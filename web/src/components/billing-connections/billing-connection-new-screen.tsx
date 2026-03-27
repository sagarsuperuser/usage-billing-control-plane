"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CreditCard, LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createBillingProviderConnection, refreshBillingProviderConnection } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function BillingConnectionNewScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [displayName, setDisplayName] = useState("");
  const [environment, setEnvironment] = useState<"test" | "live">("test");
  const [stripeSecretKey, setStripeSecretKey] = useState("");
  const [flash, setFlash] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: async () => {
      const created = await createBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          provider_type: "stripe",
          environment,
          display_name: displayName.trim(),
          scope: "platform",
          stripe_secret_key: stripeSecretKey.trim(),
        },
      });
      return refreshBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        connectionID: created.id,
      });
    },
    onSuccess: async (connection) => {
      setFlash(`Billing connection ${connection.display_name} is ready.`);
      await queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] });
      router.push(`/billing-connections/${encodeURIComponent(connection.id)}`);
    },
  });

  const canSubmit = Boolean(isPlatformAdmin && displayName.trim() && stripeSecretKey.trim());

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/billing-connections", label: "Platform" }, { href: "/billing-connections", label: "Billing Connections" }, { label: "New" }]} />

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

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "platform" ? (
          <ScopeNotice
            title="Platform session required"
            body="Billing connections are managed at the platform layer. Sign in with a platform account to create them."
            actionHref="/customers"
            actionLabel="Open tenant home"
          />
        ) : null}

        {flash ? <section className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{flash}</section> : null}

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
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
                <CreditCard className="h-5 w-5" />
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
                  <InputField label="Connection name" value={displayName} onChange={setDisplayName} placeholder="Stripe Sandbox" />
                  <label className="grid gap-2 text-sm text-slate-700">
                    <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Environment</span>
                    <select
                      aria-label="Environment"
                      value={environment}
                      onChange={(event) => setEnvironment(event.target.value as "test" | "live")}
                      className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                    >
                      <option value="test">Test</option>
                      <option value="live">Live</option>
                    </select>
                  </label>
                  <InputField label="Stripe secret key" value={stripeSecretKey} onChange={setStripeSecretKey} placeholder="sk_test_..." type="password" />
                </div>
              </section>
            </div>

            <div className="mt-5 rounded-xl border border-slate-200 bg-slate-50 p-4">
              <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Preflight</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={displayName.trim().length > 0} text="Connection name is set" />
                <ChecklistLine done={stripeSecretKey.trim().length > 0} text="Stripe secret key is set" />
                <ChecklistLine done text="Provider type is Stripe" />
                <ChecklistLine done text="Alpha will check the Stripe connection after create" />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  createMutation.mutate();
                }}
                disabled={!canSubmit || createMutation.isPending}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {createMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                Create and check connection
              </button>
            </div>
          </section>

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

function InputField({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  type?: string;
}) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        aria-label={label}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
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
