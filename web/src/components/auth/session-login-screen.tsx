import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";
import { useQuery } from "@tanstack/react-query";

import { AuthLayout } from "@/components/auth/auth-layout";
import { SessionLoginCard } from "@/components/auth/session-login-card";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchUIAuthProviders } from "@/lib/api";
import { getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
import type { UISession } from "@/lib/types";

function resolveTarget(session: UISession | null, nextPath: string | null): string {
  return normalizeNextPath(nextPath, getDefaultLandingPath(session));
}

export function SessionLoginScreen() {
  const navigate = useNavigate();
  const searchParams = useSearchParamsCompat();
  const { session, isAuthenticated, apiBaseURL } = useUISession();
  const requestedNext = searchParams.get("next");
  const accessSwitch = searchParams.get("switch") === "1";
  const providerKey = searchParams.get("provider");
  const authError = searchParams.get("error");
  const resetState = searchParams.get("reset");
  const authProvidersQuery = useQuery({
    queryKey: ["ui-auth-providers", apiBaseURL],
    queryFn: () => fetchUIAuthProviders({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(apiBaseURL),
    staleTime: 60_000,
  });

  useEffect(() => {
    if (isAuthenticated && !accessSwitch) {
      navigate({ to: resolveTarget(session, requestedNext), replace: true });
    }
  }, [accessSwitch, isAuthenticated, requestedNext, navigate, session]);

  if (isAuthenticated && !accessSwitch) {
    return (
      <div className="min-h-screen bg-background">
        <main className="flex min-h-screen items-center justify-center px-4">
          <div className="rounded-xl border border-border bg-surface px-6 py-4 text-sm text-text-muted shadow-sm">
            Redirecting…
          </div>
        </main>
      </div>
    );
  }

  return (
    <AuthLayout>
      {accessSwitch ? (
        <div className="mb-5 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          Sign in with the account you want to switch to.
        </div>
      ) : null}
      {resetState === "success" ? (
        <div className="mb-5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
          Password updated. Sign in with your new password.
        </div>
      ) : null}

      <SessionLoginCard
        apiBaseURL={apiBaseURL}
        passwordResetEnabled={Boolean(authProvidersQuery.data?.password_reset_enabled)}
        ssoProviders={authProvidersQuery.data?.sso_providers ?? []}
        providerKey={providerKey}
        authErrorCode={authError}
        nextPath={requestedNext}
        onSuccess={(nextSession) => {
          navigate({ to: resolveTarget(nextSession, requestedNext), replace: true });
        }}
        onInvitationPending={(nextPath) => {
          navigate({ to: normalizeNextPath(nextPath, "/"), replace: true });
        }}
      />
    </AuthLayout>
  );
}
