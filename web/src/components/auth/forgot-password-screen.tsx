import { Link } from "@tanstack/react-router";
import { FormEvent, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";

import { AuthLayout } from "@/components/auth/auth-layout";
import { fetchUIAuthProviders, requestPasswordReset } from "@/lib/api";
import { showError } from "@/lib/toast";
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
    onError: (err: Error) => showError(err.message),
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    resetMutation.mutate();
  };

  const enabled = Boolean(authProvidersQuery.data?.password_reset_enabled);

  return (
    <AuthLayout>
      <h2 className="text-2xl font-semibold tracking-tight text-text-primary">Reset password</h2>
      <p className="mt-1.5 text-sm text-text-muted">
        Enter your email and we'll send reset instructions.
      </p>

      {!enabled && authProvidersQuery.isSuccess ? (
        <div className="mt-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          Password reset is not available for this environment.
        </div>
      ) : null}

      {resetMutation.isSuccess ? (
        <div className="mt-5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
          If that email is registered, reset instructions have been sent.
        </div>
      ) : (
        <form className="mt-6 grid gap-4" onSubmit={onSubmit}>
          <div className="grid gap-1.5">
            <label className="text-xs font-semibold uppercase tracking-wider text-text-muted">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              autoFocus
              className="h-11 rounded-xl border border-border bg-surface px-3.5 text-sm text-text-primary outline-none ring-slate-300 transition placeholder:text-text-faint focus:ring-2"
            />
          </div>
          <button
            type="submit"
            disabled={!enabled || resetMutation.isPending || !email.trim()}
            className="mt-1 inline-flex h-11 w-full items-center justify-center rounded-xl bg-slate-900 px-4 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:opacity-50"
          >
            {resetMutation.isPending ? "Sending..." : "Send reset link"}
          </button>
          {resetMutation.isError ? <p className="text-xs text-rose-600">{resetMutation.error.message}</p> : null}
        </form>
      )}

      <p className="mt-6 text-center text-sm text-text-muted">
        <Link to="/login" className="font-medium text-text-secondary transition hover:text-text-primary">
          Back to sign in
        </Link>
      </p>
    </AuthLayout>
  );
}
