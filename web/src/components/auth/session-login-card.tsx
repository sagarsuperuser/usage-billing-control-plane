"use client";

import { FormEvent, useState } from "react";
import { LoaderCircle, LogIn } from "lucide-react";

import type { UISession } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function SessionLoginCard({
  onSuccess,
}: {
  onSuccess?: (session: UISession) => void;
}) {
  const { login, loggingIn, loginError, isLoading, configError } = useUISession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [tenantID, setTenantID] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMessage("");
    try {
      const session = await login({
        email,
        password,
        tenantID,
      });
      setPassword("");
      onSuccess?.(session);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Sign-in failed";
      setErrorMessage(message);
    }
  };

  return (
    <section className="w-full rounded-3xl border border-cyan-400/20 bg-[linear-gradient(135deg,rgba(6,182,212,0.1),rgba(15,23,42,0.82))] p-5 text-sm text-slate-100 backdrop-blur-xl">
      <div className="flex flex-col gap-4">
        <div className="max-w-2xl">
          <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/90">Browser sign-in</p>
          <h2 className="mt-2 text-xl font-semibold text-white">Start with your account credentials</h2>
          <p className="mt-2 text-sm text-slate-300">
            Human browser sessions now use email and password. Platform accounts open cross-workspace administration. Tenant accounts open assigned workspace surfaces.
          </p>
        </div>
        <div className="grid gap-3 text-xs text-slate-200/90 sm:grid-cols-2">
          <div className="rounded-2xl border border-white/10 bg-slate-950/40 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-cyan-100">Platform account</p>
            <p className="mt-1">Billing connections, workspaces, and cross-workspace readiness.</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-slate-950/40 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-cyan-100">Tenant account</p>
            <p className="mt-1">Customers, payments, recovery, and explainability inside assigned workspaces.</p>
          </div>
        </div>
      </div>

      <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
        <div className="grid gap-3 md:grid-cols-2">
          <div className="grid gap-2">
            <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Email</label>
            <input
              type="email"
              data-testid="session-login-email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="operator@alpha.test"
              className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
            />
          </div>
          <div className="grid gap-2">
            <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Password</label>
            <input
              type="password"
              data-testid="session-login-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Your account password"
              className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
            />
          </div>
        </div>

        <div className="grid gap-2">
          <label className="text-xs font-medium uppercase tracking-wider text-slate-400">Workspace ID, if your account spans more than one workspace</label>
          <input
            type="text"
            data-testid="session-login-tenant-id"
            value={tenantID}
            onChange={(event) => setTenantID(event.target.value)}
            placeholder="Optional tenant/workspace ID"
            className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
          />
        </div>

        <div className="flex items-end">
          <button
            type="submit"
            data-testid="session-login-submit"
            disabled={!email.trim() || !password.trim() || loggingIn || isLoading || Boolean(configError)}
            className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm font-medium text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingIn ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
            Start session
          </button>
        </div>
      </form>

      <p className="mt-3 text-[11px] uppercase tracking-[0.14em] text-cyan-200/80">
        Runtime API endpoint is resolved automatically for this deployment.
      </p>
      {configError ? <p className="mt-3 text-xs text-rose-200">{configError.message}</p> : null}
      {!configError && (errorMessage || loginError?.message) ? (
        <p className="mt-3 text-xs text-rose-200">{errorMessage || loginError?.message}</p>
      ) : null}
    </section>
  );
}
