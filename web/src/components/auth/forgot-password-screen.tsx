
import { Link } from "@tanstack/react-router";
import { FormEvent, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";

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
    <div className="text-text-primary">
      <main className="mx-auto flex min-h-screen max-w-2xl items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-border bg-surface p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Password recovery</p>
          <h1 className="mt-2 text-2xl font-semibold text-text-primary">Reset your Alpha password</h1>
          <p className="mt-3 text-sm text-text-muted">
            Enter the email for your Alpha browser account. If that account supports password login, we will send reset instructions.
          </p>
          <div className="mt-5 grid gap-3 sm:grid-cols-3">
            <OperatorHint title="Recovery rule" body="Use this only for browser accounts that authenticate with password. SSO-only accounts recover through the identity provider." />
            <OperatorHint title="Delivery rule" body="Alpha sends instructions only when password login is enabled for the environment and the account supports it." />
            <OperatorHint title="Security rule" body="The response stays neutral so this flow does not disclose whether an account exists." />
          </div>

          {!enabled && authProvidersQuery.isSuccess ? (
            <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              Password reset is not configured for this environment.
            </div>
          ) : null}

          <form className="mt-5 grid gap-3" onSubmit={onSubmit}>
            <div className="grid gap-2">
              <label className="text-xs font-medium uppercase tracking-wider text-text-muted">Email</label>
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                className="h-11 rounded-xl border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
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
