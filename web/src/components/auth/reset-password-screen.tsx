import { Link, useNavigate } from "@tanstack/react-router";
import { FormEvent, useMemo, useState } from "react";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";
import { useMutation } from "@tanstack/react-query";

import { AuthLayout } from "@/components/auth/auth-layout";
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
    if (!token || mismatch) return;
    resetMutation.mutate();
  };

  return (
    <AuthLayout>
      <h2 className="text-2xl font-semibold tracking-tight text-text-primary">Choose a new password</h2>
      <p className="mt-1.5 text-sm text-text-muted">
        Enter your new password below.
      </p>

      {!token ? (
        <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          This reset link is missing its token. Open the link directly from your email.
        </div>
      ) : null}

      <form className="mt-6 grid gap-4" onSubmit={onSubmit}>
        <div className="grid gap-1.5">
          <label className="text-xs font-semibold uppercase tracking-wider text-text-muted">New password</label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="At least 12 characters"
            autoFocus
            className="h-11 rounded-xl border border-border bg-surface px-3.5 text-sm text-text-primary outline-none ring-slate-300 transition placeholder:text-text-faint focus:ring-2"
          />
        </div>
        <div className="grid gap-1.5">
          <label className="text-xs font-semibold uppercase tracking-wider text-text-muted">Confirm password</label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="Repeat the new password"
            className="h-11 rounded-xl border border-border bg-surface px-3.5 text-sm text-text-primary outline-none ring-slate-300 transition placeholder:text-text-faint focus:ring-2"
          />
        </div>
        {mismatch ? <p className="text-xs text-rose-600">Passwords do not match.</p> : null}
        <button
          type="submit"
          disabled={!token || !password.trim() || mismatch || resetMutation.isPending}
          className="mt-1 inline-flex h-11 w-full items-center justify-center rounded-xl bg-slate-900 px-4 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:opacity-50"
        >
          {resetMutation.isPending ? "Updating..." : "Update password"}
        </button>
        {resetMutation.isError ? <p className="text-xs text-rose-600">{resetMutation.error.message}</p> : null}
      </form>

      <p className="mt-6 text-center text-sm text-text-muted">
        <Link to="/login" className="font-medium text-text-secondary transition hover:text-text-primary">
          Back to sign in
        </Link>
      </p>
    </AuthLayout>
  );
}
