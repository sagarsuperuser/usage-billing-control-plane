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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Password recovery</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">Reset your Alpha password</h1>
          <p className="mt-3 text-sm text-slate-600">
            Enter the email for your Alpha browser account. If that account supports password login, we will send reset instructions.
          </p>

          {!enabled && authProvidersQuery.isSuccess ? (
            <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              Password reset is not configured for this environment.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Email</label>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>
            <button
              type="submit"
              disabled={!enabled || resetMutation.isPending || !email.trim()}
              className="inline-flex h-11 items-center justify-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {resetMutation.isPending ? "Sending..." : "Send reset instructions"}
            </button>
          </form>

          {resetMutation.isSuccess ? (
            <div className="mt-4 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
              If that account exists and supports password login, reset instructions have been sent.
            </div>
          ) : null}
          {resetMutation.isError ? <p className="mt-4 text-xs text-rose-700">{resetMutation.error.message}</p> : null}

          <div className="mt-5">
            <Link href="/login" prefetch={false} className="text-sm text-slate-600 transition hover:text-slate-900">
              Back to login
            </Link>
          </div>
        </section>
      </main>
    </div>
  );
}
