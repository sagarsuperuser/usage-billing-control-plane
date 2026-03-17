"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { CreditCard, LoaderCircle, ShieldCheck } from "lucide-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { createBillingProviderConnection, syncBillingProviderConnection } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function BillingConnectionNewScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, isAuthenticated, isPlatformAdmin, scope } = useUISession();
  const [displayName, setDisplayName] = useState("");
  const [environment, setEnvironment] = useState<"test" | "live">("test");
  const [lagoOrganizationID, setLagoOrganizationID] = useState("");
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
          lago_organization_id: lagoOrganizationID.trim() || undefined,
        },
      });
      return syncBillingProviderConnection({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        connectionID: created.id,
      });
    },
    onSuccess: async (connection) => {
      setFlash(`Billing connection ${connection.display_name} is connected.`);
      await queryClient.invalidateQueries({ queryKey: ["billing-provider-connections"] });
      router.push(`/billing-connections/${encodeURIComponent(connection.id)}`);
    },
  });

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#1d4ed8_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-0 h-72 w-72 rounded-full bg-fuchsia-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1120px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Platform Setup</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">New Billing Connection</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Create a platform-owned Stripe connection. Alpha stores the secret, syncs the provider, and exposes a stable connection record for workspace assignment.
              </p>
            </div>
            <Link
              href="/billing-connections"
              className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
            >
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

        {flash ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100">
            {flash}
          </section>
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_340px]">
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Guided setup</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Connect Stripe</h2>
                <p className="mt-2 max-w-2xl text-sm text-slate-300">
                  This is a Stripe-first platform flow. Alpha keeps the secret in its secret store and pushes only the provider configuration needed for sync.
                </p>
              </div>
              <span className="inline-flex rounded-xl border border-fuchsia-400/40 bg-fuchsia-500/10 p-3 text-fuchsia-100">
                <CreditCard className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-6 grid gap-3 lg:grid-cols-3">
              <StepCard index="1" title="Name the connection" body="Use a durable display name your operators can recognize later." />
              <StepCard index="2" title="Store the Stripe secret" body="Alpha stores the secret outside the database using the billing secret store." />
              <StepCard index="3" title="Sync the provider" body="Alpha provisions the matching provider and keeps the resulting mapping on the connection record." />
            </div>

            <div className="mt-6 rounded-2xl border border-white/10 bg-slate-950/55 p-5">
              <div className="grid gap-4 md:grid-cols-2">
                <InputField label="Connection name" value={displayName} onChange={setDisplayName} placeholder="Stripe Sandbox" />
                <label className="grid gap-2 text-sm text-slate-200">
                  <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Environment</span>
                  <select
                    aria-label="Environment"
                    value={environment}
                    onChange={(event) => setEnvironment(event.target.value as "test" | "live")}
                    className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  >
                    <option value="test">Test</option>
                    <option value="live">Live</option>
                  </select>
                </label>
                <InputField
                  label="Stripe secret key"
                  value={stripeSecretKey}
                  onChange={setStripeSecretKey}
                  placeholder="sk_test_..."
                  type="password"
                />
              </div>

              <details className="mt-4 rounded-2xl border border-white/10 bg-slate-950/35 p-4">
                <summary className="cursor-pointer list-none text-sm font-semibold text-white">Advanced provider mapping</summary>
                <div className="mt-4 grid gap-4 md:grid-cols-2">
                  <InputField
                    label="Billing organization reference (optional)"
                    value={lagoOrganizationID}
                    onChange={setLagoOrganizationID}
                    placeholder="org_acme"
                  />
                </div>
              </details>
            </div>

            <div className="mt-4 rounded-2xl border border-cyan-400/20 bg-cyan-500/5 p-4">
              <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Before you run</p>
              <div className="mt-3 grid gap-2 md:grid-cols-2">
                <ChecklistLine done={displayName.trim().length > 0} text="Connection name is set" />
                <ChecklistLine done={stripeSecretKey.trim().length > 0} text="Stripe secret key is set" />
                <ChecklistLine done text="Provider type is Stripe" />
                <ChecklistLine done text={lagoOrganizationID.trim().length > 0 ? "Billing organization reference added" : "Billing organization reference can be added later"} />
              </div>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  createMutation.mutate();
                }}
                disabled={!isPlatformAdmin || !csrfToken || createMutation.isPending || !displayName.trim() || !stripeSecretKey.trim()}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-fuchsia-400/40 bg-fuchsia-500/10 px-4 text-sm font-medium text-fuchsia-100 transition hover:bg-fuchsia-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {createMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
                Create and sync connection
              </button>
            </div>
          </section>

          <aside className="flex flex-col gap-4">
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">What Alpha owns</p>
              <div className="mt-4 grid gap-3">
                <ChecklistLine done text="Secret stays out of tenant rows" />
                <ChecklistLine done text="Provider sync is explicit and observable" />
                <ChecklistLine done text="Workspaces link a stable connection record" />
              </div>
            </section>
            <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl text-sm text-slate-300">
              <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Current limitation</p>
              <p className="mt-3">
                Secret rotation is not exposed here yet because the current provider update contract cannot cleanly rotate Stripe secrets through the same public update path.
              </p>
            </section>
          </aside>
        </div>
      </main>
    </div>
  );
}

function StepCard({ index, title, body }: { index: string; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/45 p-4">
      <p className="text-xs uppercase tracking-[0.16em] text-cyan-300/80">Step {index}</p>
      <p className="mt-2 text-sm font-semibold text-white">{title}</p>
      <p className="mt-2 text-sm text-slate-300">{body}</p>
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
    <label className="grid gap-2 text-sm text-slate-200">
      <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">{label}</span>
      <input
        aria-label={label}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </label>
  );
}

function ChecklistLine({ done, text }: { done: boolean; text: string }) {
  return (
    <div className="flex items-start gap-3 rounded-xl border border-white/10 bg-white/5 px-3 py-3">
      <span
        className={`mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold ${done ? "bg-emerald-500/20 text-emerald-100" : "bg-amber-500/20 text-amber-100"}`}
      >
        {done ? "OK" : "!"}
      </span>
      <p className="text-sm text-slate-200">{text}</p>
    </div>
  );
}
