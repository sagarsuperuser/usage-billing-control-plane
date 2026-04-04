
import { Link } from "@tanstack/react-router";
import { LoaderCircle, LogIn } from "lucide-react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";

import type { UISession, UIAuthProvider } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";
import { isInvitationPendingLoginError } from "@/lib/api";
import { normalizeNextPath } from "@/lib/session-routing";

const loginSchema = z.object({
  email: z.string().email("Enter a valid email address"),
  password: z.string().min(1, "Password is required"),
});

type LoginFields = z.infer<typeof loginSchema>;

export function SessionLoginCard({
  apiBaseURL,
  passwordResetEnabled,
  ssoProviders,
  providerKey,
  authErrorCode,
  onSuccess,
  onInvitationPending,
  nextPath,
}: {
  apiBaseURL: string;
  passwordResetEnabled?: boolean;
  ssoProviders?: UIAuthProvider[];
  providerKey?: string | null;
  authErrorCode?: string | null;
  onSuccess?: (session: UISession) => void;
  onInvitationPending?: (nextPath: string) => void;
  nextPath?: string | null;
}) {
  const { login, loggingIn, loginError, isLoading, configError } = useUISession();

  const {
    register,
    handleSubmit,
    reset,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<LoginFields>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (data: LoginFields) => {
    try {
      const session = await login({ email: data.email, password: data.password, nextPath: nextPath ?? undefined });
      reset();
      onSuccess?.(session);
    } catch (err) {
      if (isInvitationPendingLoginError(err)) {
        reset({ email: data.email });
        onInvitationPending?.(err.nextPath);
        return;
      }
      const message = err instanceof Error ? err.message : "Sign-in failed";
      setError("root", { message });
    }
  };

  const busy = isSubmitting || loggingIn || isLoading;

  return (
    <div className="w-full">
      <h2 className="text-2xl font-semibold tracking-tight text-slate-950">Sign in</h2>
      <p className="mt-1.5 text-sm text-slate-500">Enter your operator account credentials.</p>

      {ssoProviders && ssoProviders.length > 0 ? (
        <div className="mt-6 grid gap-2">
          {ssoProviders.map((provider) => (
            <a
              key={provider.key}
              href={buildSSOStartURL(apiBaseURL, provider.key, nextPath)}
              className="inline-flex h-11 w-full items-center justify-center rounded-xl border border-stone-200 bg-white px-4 text-sm font-medium text-slate-800 shadow-sm transition hover:border-stone-300 hover:bg-stone-50"
            >
              Continue with {provider.display_name}
            </a>
          ))}
          {(providerKey || authErrorCode) && (
            <p className="text-xs text-rose-600">{resolveAuthErrorMessage(providerKey ?? null, authErrorCode ?? null)}</p>
          )}
          <div className="my-2 flex items-center gap-3">
            <div className="h-px flex-1 bg-stone-200" />
            <span className="text-[11px] font-medium uppercase tracking-widest text-slate-400">or</span>
            <div className="h-px flex-1 bg-stone-200" />
          </div>
        </div>
      ) : null}

      <form className="mt-6 grid gap-4" onSubmit={handleSubmit(onSubmit)} noValidate>
        <div className="grid gap-1.5">
          <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">Email</label>
          <input
            type="email"
            data-testid="session-login-email"
            placeholder="you@example.com"
            autoComplete="email"
            className="h-11 rounded-xl border border-stone-200 bg-white px-3.5 text-sm text-slate-900 outline-none ring-slate-300 transition placeholder:text-slate-400 focus:ring-2 aria-invalid:border-rose-300 aria-invalid:ring-rose-200"
            aria-invalid={errors.email ? "true" : undefined}
            {...register("email")}
          />
          {errors.email ? <p className="text-xs text-rose-600">{errors.email.message}</p> : null}
        </div>

        <div className="grid gap-1.5">
          <div className="flex items-center justify-between">
            <label className="text-xs font-semibold uppercase tracking-wider text-slate-500">Password</label>
            {passwordResetEnabled ? (
              <Link to="/forgot-password" className="text-xs text-slate-400 transition hover:text-slate-700">
                Forgot password?
              </Link>
            ) : null}
          </div>
          <input
            type="password"
            data-testid="session-login-password"
            placeholder="••••••••"
            autoComplete="current-password"
            className="h-11 rounded-xl border border-stone-200 bg-white px-3.5 text-sm text-slate-900 outline-none ring-slate-300 transition placeholder:text-slate-400 focus:ring-2 aria-invalid:border-rose-300 aria-invalid:ring-rose-200"
            aria-invalid={errors.password ? "true" : undefined}
            {...register("password")}
          />
          {errors.password ? <p className="text-xs text-rose-600">{errors.password.message}</p> : null}
        </div>

        <button
          type="submit"
          data-testid="session-login-submit"
          disabled={busy || Boolean(configError)}
          className="mt-1 inline-flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-slate-900 px-4 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
          Sign in
        </button>

        {configError ? <p className="text-xs text-rose-600">{configError.message}</p> : null}
        {!configError && (errors.root?.message || loginError?.message) ? (
          <p className="text-xs text-rose-600">{errors.root?.message || loginError?.message}</p>
        ) : null}
      </form>

      <p className="mt-6 text-center text-sm text-slate-500">
        Don&apos;t have an account?{" "}
        <Link to="/register" className="font-medium text-slate-700 transition hover:text-slate-900">
          Create one
        </Link>
      </p>
    </div>
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
      return `No account is provisioned${label}. Ask an admin to grant access first.`;
    case "workspace_invitation_email_mismatch":
      return `This invitation is for a different email address.`;
    case "sso_email_not_verified":
      return `The identity provider did not return a verified email${label}.`;
    case "tenant_selection_required":
      return "This account spans multiple workspaces. Continue to choose one.";
    case "tenant_access_denied":
      return `Authenticated${label}, but this account does not have access to the requested workspace.`;
    case "user_disabled":
      return "This account is disabled.";
    case "sso_denied":
      return `Sign-in was cancelled${label}.`;
    default:
      return `Single sign-on failed${label}. Try again or use email and password.`;
  }
}
