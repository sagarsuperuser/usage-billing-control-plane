"use client";

import { FormEvent, useState } from "react";
import { LoaderCircle, LogIn } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";

export function SessionLoginCard() {
  const { login, loggingIn, loginError, isLoading, configError } = useUISession();
  const [apiKey, setAPIKey] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMessage("");
    try {
      await login(apiKey);
      setAPIKey("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Login failed";
      setErrorMessage(message);
    }
  };

  return (
    <section className="rounded-3xl border border-amber-400/30 bg-[linear-gradient(135deg,rgba(245,158,11,0.14),rgba(15,23,42,0.72))] p-5 text-sm text-amber-100 backdrop-blur-xl">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="max-w-2xl">
          <p className="text-xs uppercase tracking-[0.2em] text-amber-200/90">Sign in</p>
          <h2 className="mt-2 text-xl font-semibold text-amber-50">Open the right control surface</h2>
          <p className="mt-2 text-sm text-amber-100/90">
            Use a platform key for workspace setup and a tenant key for customer, payment, recovery, and explainability workflows.
          </p>
        </div>
        <div className="grid gap-3 text-xs text-amber-100/90 sm:grid-cols-2">
          <div className="rounded-2xl border border-white/10 bg-slate-950/40 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-amber-50">Platform key</p>
            <p className="mt-1">Workspace setup, onboarding status, and cross-workspace visibility.</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-slate-950/40 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-amber-50">Tenant key</p>
            <p className="mt-1">Customer onboarding, payments, recovery, and invoice explainability.</p>
          </div>
        </div>
      </div>
      <form className="grid gap-3 md:grid-cols-[1fr_auto]" onSubmit={onSubmit}>
        <div className="grid gap-2">
          <label className="text-xs font-medium uppercase tracking-wider text-amber-200">API Key</label>
          <input
            type="password"
            data-testid="session-login-api-key"
            value={apiKey}
            onChange={(event) => setAPIKey(event.target.value)}
            placeholder="tenant or platform key"
            className="h-10 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
          />
        </div>
        <div className="flex items-end">
          <button
            type="submit"
            data-testid="session-login-submit"
            disabled={!apiKey.trim() || loggingIn || isLoading || Boolean(configError)}
            className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingIn ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
            Sign In
          </button>
        </div>
      </form>
      <p className="mt-3 text-[11px] uppercase tracking-[0.14em] text-amber-200/80">
        Runtime API endpoint is resolved automatically for this deployment.
      </p>
      {configError ? <p className="mt-3 text-xs text-rose-200">{configError.message}</p> : null}
      {!configError && (errorMessage || loginError?.message) ? (
        <p className="mt-3 text-xs text-rose-200">{errorMessage || loginError?.message}</p>
      ) : null}
    </section>
  );
}
