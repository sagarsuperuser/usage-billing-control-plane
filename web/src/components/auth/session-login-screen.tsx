"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchUIAuthProviders } from "@/lib/api";
import { buildWorkspaceSelectionPath, getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
import type { UISession } from "@/lib/types";

function resolveTarget(session: UISession | null, nextPath: string | null): string {
  return normalizeNextPath(nextPath, getDefaultLandingPath(session));
}

export function SessionLoginScreen() {
  const router = useRouter();
  const searchParams = useSearchParams();
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
      router.replace(resolveTarget(session, requestedNext));
    }
  }, [accessSwitch, isAuthenticated, requestedNext, router, session]);

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
        <div className="hidden xl:flex flex-col justify-between bg-slate-950 px-14 py-12">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-white/10">
              <svg width="16" height="16" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.4"/>
                <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.65"/>
                <rect x="12" y="2" width="3" height="14" rx="1" fill="white"/>
              </svg>
            </div>
            <span className="text-sm font-semibold text-white">Alpha</span>
          </div>

          <div>
            <h1 className="text-4xl font-semibold leading-tight tracking-tight text-white">
              Usage billing<br />control plane
            </h1>
            <p className="mt-4 max-w-sm text-base leading-7 text-slate-400">
              Pricing, customers, subscriptions, payments, and invoices — all in one place.
            </p>
            <div className="mt-10 grid gap-3">
              <Capability label="Billing connections" detail="Own provider records and assign them to workspaces." />
              <Capability label="Pricing catalog" detail="Metrics, rating rules, plans, coupons, and taxes." />
              <Capability label="Customer operations" detail="Onboarding, payment setup, and status checks." />
              <Capability label="Invoices & recovery" detail="Usage billing, replay, dunning, and explainability." />
            </div>
          </div>

          <p className="text-xs text-slate-600">Staging environment</p>
        </div>

        {/* Right — login form */}
        <div className="flex flex-col items-center justify-center px-6 py-12 sm:px-10">
          <div className="w-full max-w-[400px]">
            <div className="mb-8 xl:hidden flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-900">
                <svg width="16" height="16" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.4"/>
                  <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.65"/>
                  <rect x="12" y="2" width="3" height="14" rx="1" fill="white"/>
                </svg>
              </div>
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
                router.replace(resolveTarget(nextSession, requestedNext));
              }}
              onSelectionRequired={() => {
                router.replace(buildWorkspaceSelectionPath(requestedNext));
              }}
              onInvitationPending={(nextPath) => {
                router.replace(normalizeNextPath(nextPath, "/"));
              }}
            />
          </div>
        </div>
      </main>
    </div>
  );
}

function Capability({ label, detail }: { label: string; detail: string }) {
  return (
    <div className="flex items-start gap-3">
      <div className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-slate-500" />
      <div>
        <span className="text-sm font-medium text-slate-200">{label}</span>
        <span className="ml-2 text-sm text-slate-500">{detail}</span>
      </div>
    </div>
  );
}
