"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { LoaderCircle, LogIn } from "lucide-react";

import type { UISession } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";
import { isInvitationPendingLoginError, isWorkspaceSelectionRequiredError } from "@/lib/api";

export function SessionLoginCard({
  passwordResetEnabled,
  onSuccess,
  onSelectionRequired,
  onInvitationPending,
  nextPath,
}: {
  passwordResetEnabled?: boolean;
  onSuccess?: (session: UISession) => void;
  onSelectionRequired?: () => void;
  onInvitationPending?: (nextPath: string) => void;
  nextPath?: string | null;
}) {
  const { login, loggingIn, loginError, isLoading, configError } = useUISession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMessage("");
    try {
      const session = await login({ email, password, nextPath: nextPath ?? undefined });
      setPassword("");
      onSuccess?.(session);
    } catch (err) {
      if (isWorkspaceSelectionRequiredError(err)) {
        setPassword("");
        onSelectionRequired?.();
        return;
      }
      if (isInvitationPendingLoginError(err)) {
        setPassword("");
        onInvitationPending?.(err.nextPath);
        return;
      }
      const message = err instanceof Error ? err.message : "Sign-in failed";
      setErrorMessage(message);
    }
  };

  return (
    <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 text-sm text-slate-800 shadow-sm">
      <div className="flex flex-col gap-4">
        <div className="max-w-2xl">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Browser sign-in</p>
          <h2 className="mt-2 text-xl font-semibold text-slate-950">Start with your account credentials</h2>
          <p className="mt-2 text-sm text-slate-600">
            Human browser sessions now use email and password. Platform accounts open cross-workspace administration. Tenant accounts open assigned workspace surfaces.
          </p>
        </div>
        <div className="grid gap-3 text-xs text-slate-600 sm:grid-cols-2">
          <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-slate-900">Platform account</p>
            <p className="mt-1">Billing connections, workspaces, and cross-workspace readiness.</p>
          </div>
          <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-slate-900">Workspace account</p>
            <p className="mt-1">Customers, payments, recovery, and explainability inside assigned workspaces.</p>
          </div>
        </div>
      </div>

      <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
        <div className="grid gap-3 md:grid-cols-2">
          <div className="grid gap-2">
            <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Email</label>
            <input
              type="email"
              data-testid="session-login-email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="operator@alpha.test"
              className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
          <div className="grid gap-2">
            <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Password</label>
            <input
              type="password"
              data-testid="session-login-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Your account password"
              className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
        </div>

        <div className="flex items-end">
          <button
            type="submit"
            data-testid="session-login-submit"
            disabled={!email.trim() || !password.trim() || loggingIn || isLoading || Boolean(configError)}
            className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingIn ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
            Start session
          </button>
        </div>
        {passwordResetEnabled ? (
          <div className="flex justify-end">
            <Link href="/forgot-password" className="text-xs text-slate-600 transition hover:text-slate-900">
              Forgot password?
            </Link>
          </div>
        ) : null}
      </form>

      <p className="mt-3 text-[11px] uppercase tracking-[0.14em] text-slate-500">
        Runtime API endpoint is resolved automatically for this deployment.
      </p>
      {configError ? <p className="mt-3 text-xs text-rose-700">{configError.message}</p> : null}
      {!configError && (errorMessage || loginError?.message) ? (
        <p className="mt-3 text-xs text-rose-700">{errorMessage || loginError?.message}</p>
      ) : null}
    </section>
  );
}
