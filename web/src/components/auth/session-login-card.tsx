"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { LoaderCircle, LogIn } from "lucide-react";

import type { UISession, UIAuthProvider } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";
import { isInvitationPendingLoginError, isWorkspaceSelectionRequiredError } from "@/lib/api";
import { normalizeNextPath } from "@/lib/session-routing";

export function SessionLoginCard({
  apiBaseURL,
  passwordResetEnabled,
  ssoProviders,
  providerKey,
  authErrorCode,
  onSuccess,
  onSelectionRequired,
  onInvitationPending,
  nextPath,
}: {
  apiBaseURL: string;
  passwordResetEnabled?: boolean;
  ssoProviders?: UIAuthProvider[];
  providerKey?: string | null;
  authErrorCode?: string | null;
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
      <div className="flex flex-col gap-4 border-b border-stone-200 pb-5">
        <div className="max-w-2xl">
          <div className="flex items-center justify-between gap-4">
            <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Browser sign-in</p>
            <span className="rounded-full border border-stone-200 bg-stone-50 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-600">
              Password + SSO
            </span>
          </div>
          <h2 className="mt-2 text-xl font-semibold text-slate-950">Start with your account credentials</h2>
          <p className="mt-2 text-sm leading-7 text-slate-600">
            Human browser sessions support email and password as well as single sign-on. Platform accounts open cross-workspace administration. Tenant accounts open assigned workspace surfaces.
          </p>
        </div>
        <div className="grid gap-3 text-xs text-slate-600 sm:grid-cols-2">
          <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-slate-900">Platform account</p>
            <p className="mt-1 leading-6">Billing connections, workspaces, and cross-workspace readiness.</p>
          </div>
          <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
            <p className="font-semibold uppercase tracking-[0.14em] text-slate-900">Workspace account</p>
            <p className="mt-1 leading-6">Customers, payments, recovery, and explainability inside assigned workspaces.</p>
          </div>
        </div>
      </div>

      {ssoProviders && ssoProviders.length > 0 ? (
        <div className="mt-5 rounded-2xl border border-stone-200 bg-stone-50 p-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Single sign-on</p>
              <h3 className="mt-1 text-base font-semibold text-slate-950">Continue with your identity provider</h3>
            </div>
            <span className="rounded-full border border-stone-200 bg-white px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-600">
              Recommended
            </span>
          </div>
          <p className="mt-2 text-sm text-slate-600">Use SSO for browser sessions. API keys stay on API and integration traffic only.</p>
          <div className="mt-4 grid gap-3">
            {ssoProviders.map((provider) => (
              <a
                key={provider.key}
                href={buildSSOStartURL(apiBaseURL, provider.key, nextPath)}
                className="inline-flex h-11 items-center justify-center rounded-xl border border-stone-200 bg-white px-4 text-sm font-medium text-slate-800 transition hover:border-stone-300 hover:bg-stone-50"
              >
                Continue with {provider.display_name}
              </a>
            ))}
          </div>
          {(providerKey || authErrorCode) && (
            <p className="mt-3 text-xs text-amber-700">{resolveAuthErrorMessage(providerKey ?? null, authErrorCode ?? null)}</p>
          )}
        </div>
      ) : null}

      <div className="mt-5 flex items-center gap-3">
        <div className="h-px flex-1 bg-stone-200" />
        <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">Or use password</span>
        <div className="h-px flex-1 bg-stone-200" />
      </div>

      <form className="mt-5 grid gap-4" onSubmit={onSubmit}>
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

function buildSSOStartURL(apiBaseURL: string, providerKey: string, nextPath: string | null | undefined): string {
  const baseURL = apiBaseURL.replace(/\/+$/, "");
  const url = new URL(`${baseURL}/v1/ui/auth/sso/${encodeURIComponent(providerKey)}/start`);
  if (nextPath) {
    url.searchParams.set("next", normalizeNextPath(nextPath, "/"));
  }
  return url.toString();
}

function resolveAuthErrorMessage(providerKey: string | null, errorCode: string | null): string {
  const label = providerKey ? ` for ${providerKey}` : "";
  switch (errorCode) {
    case "sso_user_not_provisioned":
      return `No browser account is provisioned${label}. Ask an admin to grant platform or workspace access first.`;
    case "workspace_invitation_email_mismatch":
      return `This invitation is for a different email${label}. Sign in with the invited Google account or open the invite again with the correct account.`;
    case "sso_email_not_verified":
      return `The identity provider did not return a verified email${label}.`;
    case "tenant_selection_required":
      return "This account spans more than one workspace. Continue to choose the workspace you want to open.";
    case "tenant_access_denied":
      return `This account is authenticated${label}, but it does not have access to the requested workspace.`;
    case "user_disabled":
      return "This browser account is disabled.";
    case "sso_denied":
      return `The sign-in request was cancelled${label}.`;
    default:
      return `Single sign-on failed${label}. Try again or use email and password.`;
  }
}
