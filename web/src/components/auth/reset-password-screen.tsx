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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Password reset</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">Choose a new password</h1>
          <p className="mt-3 text-sm text-slate-600">
            Reset tokens are single-use and expire automatically.
          </p>

          {!token ? (
            <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              This reset link is missing its token. Open the link directly from your email.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-500">New password</label>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="At least 12 characters"
                className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Confirm password</label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                placeholder="Repeat the new password"
                className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
              />
            </div>
            {mismatch ? <p className="text-xs text-rose-700">Passwords do not match.</p> : null}
            <button
              type="submit"
              disabled={!token || !password.trim() || mismatch || resetMutation.isPending}
              className="inline-flex h-11 items-center justify-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {resetMutation.isPending ? "Updating..." : "Update password"}
            </button>
          </form>

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
