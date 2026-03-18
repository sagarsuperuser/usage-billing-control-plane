"use client";

import Link from "next/link";
import { FormEvent, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { fetchUIAuthProviders, requestPasswordReset } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function ForgotPasswordScreen() {
  const { apiBaseURL } = useUISession();
  const [email, setEmail] = useState("");
  const authProvidersQuery = useQuery({
    queryKey: ["ui-auth-providers", apiBaseURL],
    queryFn: () => fetchUIAuthProviders({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(apiBaseURL),
    staleTime: 60_000,
  });
  const resetMutation = useMutation({
    mutationFn: () => requestPasswordReset({ runtimeBaseURL: apiBaseURL, email }),
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    resetMutation.mutate();
  };

  const enabled = Boolean(authProvidersQuery.data?.password_reset_enabled);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Password recovery</p>
          <h1 className="mt-2 text-2xl font-semibold text-white">Reset your Alpha password</h1>
          <p className="mt-3 text-sm text-slate-300">
            Enter the email for your Alpha browser account. If that account supports password login, we will send reset instructions.
          </p>

          {!enabled && authProvidersQuery.isSuccess ? (
            <div className="mt-5 rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
              Password reset is not configured for this environment.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Email</label>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
            </div>
            <button
              type="submit"
              disabled={!enabled || resetMutation.isPending || !email.trim()}
              className="inline-flex h-11 items-center justify-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {resetMutation.isPending ? "Sending..." : "Send reset instructions"}
            </button>
          </form>

          {resetMutation.isSuccess ? (
            <div className="mt-4 rounded-2xl border border-emerald-400/30 bg-emerald-500/10 p-4 text-sm text-emerald-100">
              If that account exists and supports password login, reset instructions have been sent.
            </div>
          ) : null}
          {resetMutation.isError ? <p className="mt-4 text-xs text-rose-200">{resetMutation.error.message}</p> : null}

          <div className="mt-5">
            <Link href="/login" className="text-sm text-slate-300 transition hover:text-white">
              Back to login
            </Link>
          </div>
        </section>
      </main>
    </div>
  );
}
