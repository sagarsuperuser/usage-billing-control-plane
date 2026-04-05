import { Link, useNavigate } from "@tanstack/react-router";
import { FormEvent, useMemo, useState } from "react";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";
import { useMutation } from "@tanstack/react-query";

import { resetPassword } from "@/lib/api";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

export function ResetPasswordScreen() {
  const navigate = useNavigate();
  const searchParams = useSearchParamsCompat();
  const { apiBaseURL } = useUISession();
  const token = useMemo(() => searchParams.get("token")?.trim() ?? "", [searchParams]);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const resetMutation = useMutation({
    mutationFn: () => resetPassword({ runtimeBaseURL: apiBaseURL, token, password }),
    onSuccess: () => {
      navigate({ to: "/login?reset=success", replace: true });
    },
    onError: (err: Error) => showError(err.message),
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
    <div className="text-text-primary">
      <main className="mx-auto flex min-h-screen max-w-2xl items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-border bg-surface p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Password reset</p>
          <h1 className="mt-2 text-2xl font-semibold text-text-primary">Choose a new password</h1>
          <p className="mt-3 text-sm text-text-muted">
            Reset tokens are single-use and expire automatically.
          </p>
          <div className="mt-5 grid gap-3 sm:grid-cols-3">
            <OperatorHint title="Token rule" body="Open the reset link directly from the email so Alpha receives the signed token intact." />
            <OperatorHint title="Password rule" body="Set the new password here only after confirming the token is valid and the repeated value matches." />
            <OperatorHint title="Completion rule" body="After success, Alpha returns you to login so the new password is exercised through a fresh browser session." />
          </div>

          {!token ? (
            <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              This reset link is missing its token. Open the link directly from your email.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-text-muted">New password</label>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="At least 12 characters"
                className="h-11 rounded-xl border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
              />
            </div>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-text-muted">Confirm password</label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                placeholder="Repeat the new password"
                className="h-11 rounded-xl border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
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
            <Link to="/login" className="text-sm text-text-muted transition hover:text-text-primary">
              Back to login
            </Link>
          </div>
        </section>
      </main>
    </div>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-border bg-surface-secondary px-4 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-muted">{title}</p>
      <p className="mt-2 text-sm leading-6 text-text-secondary">{body}</p>
    </div>
  );
}
