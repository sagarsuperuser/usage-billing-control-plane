"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useMutation } from "@tanstack/react-query";

import { resetPassword } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function ResetPasswordScreen() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { apiBaseURL } = useUISession();
  const token = useMemo(() => searchParams.get("token")?.trim() ?? "", [searchParams]);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const resetMutation = useMutation({
    mutationFn: () => resetPassword({ runtimeBaseURL: apiBaseURL, token, password }),
    onSuccess: () => {
      router.replace("/login?reset=success");
    },
  });

  const mismatch = confirmPassword.length > 0 && password !== confirmPassword;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!token || mismatch) {
      return;
    }
    resetMutation.mutate();
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Password reset</p>
          <h1 className="mt-2 text-2xl font-semibold text-white">Choose a new password</h1>
          <p className="mt-3 text-sm text-slate-300">
            Reset tokens are single-use and expire automatically.
          </p>

          {!token ? (
            <div className="mt-5 rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
              This reset link is missing its token. Open the link directly from your email.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">New password</label>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="At least 12 characters"
                className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
            </div>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Confirm password</label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                placeholder="Repeat the new password"
                className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
              />
            </div>
            {mismatch ? <p className="text-xs text-rose-200">Passwords do not match.</p> : null}
            <button
              type="submit"
              disabled={!token || !password.trim() || mismatch || resetMutation.isPending}
              className="inline-flex h-11 items-center justify-center rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm font-medium text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {resetMutation.isPending ? "Updating..." : "Update password"}
            </button>
          </form>

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
