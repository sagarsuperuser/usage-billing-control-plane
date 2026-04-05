import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";
import { useQuery } from "@tanstack/react-query";

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
      <div className="min-h-screen bg-[#f5f7fb]">
        <main className="flex min-h-screen items-center justify-center px-4">
          <div className="rounded-xl border border-stone-200 bg-white px-6 py-4 text-sm text-slate-500 shadow-sm">
            Redirecting…
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f5f7fb]">
      <main className="grid min-h-screen xl:grid-cols-[minmax(0,1fr)_480px]">

        {/* Left — brand panel */}
        <div className="hidden xl:flex flex-col justify-between overflow-hidden relative bg-[#0a0a0f]  px-14 py-12">
          {/* Background gradient mesh */}
          <div className="absolute inset-0 opacity-30">
            <div className="absolute -left-20 -top-20 h-[500px] w-[500px] rounded-full bg-blue-600/30 blur-[120px]" />
            <div className="absolute -bottom-20 -right-20 h-[400px] w-[400px] rounded-full bg-indigo-600/20 blur-[100px]" />
            <div className="absolute left-1/2 top-1/2 h-[300px] w-[300px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-violet-600/15 blur-[80px]" />
          </div>

          <div className="relative flex items-center gap-3">
            <BrandLogo size={32} />
            <span className="text-sm font-semibold text-white tracking-wide">Alpha</span>
          </div>

          <div className="relative">
            <h1 className="text-5xl font-bold leading-[1.1] tracking-tight text-white">
              Billing<br />infrastructure<br />
              <span className="bg-gradient-to-r from-blue-400 to-violet-400 bg-clip-text text-transparent">
                that scales.
              </span>
            </h1>
            <p className="mt-6 max-w-sm text-[15px] leading-7 text-slate-400">
              Usage-based pricing, automated invoicing, and payment collection — built for operators who need full control.
            </p>
          </div>

          <p className="relative text-[11px] text-slate-600 tracking-wider uppercase">Staging environment</p>
        </div>

        {/* Right — login form */}
        <div className="flex flex-col items-center justify-center px-6 py-12 sm:px-10">
          <div className="w-full max-w-[400px]">
            <div className="mb-8 xl:hidden flex items-center gap-2.5">
              <BrandLogo size={28} />
              <span className="text-sm font-semibold text-slate-900">Alpha</span>
            </div>

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
          </div>
        </div>
      </main>
    </div>
  );
}

function BrandLogo({ size = 32 }: { size?: number }) {
  return (
    <div
      className="flex items-center justify-center rounded-xl bg-gradient-to-br from-blue-600 to-violet-600"
      style={{ width: size, height: size }}
    >
      <svg width={size * 0.5} height={size * 0.5} viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.5" />
        <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.75" />
        <rect x="12" y="2" width="3" height="14" rx="1" fill="white" />
      </svg>
    </div>
  );
}
