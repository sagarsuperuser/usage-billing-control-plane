"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Building2, CreditCard, UserRoundPlus } from "lucide-react";

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
  const { session, isAuthenticated, isLoading, apiBaseURL } = useUISession();
  const requestedNext = searchParams.get("next");
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
    if (!isLoading && isAuthenticated) {
      router.replace(resolveTarget(session, requestedNext));
    }
  }, [isAuthenticated, isLoading, requestedNext, router, session]);

  if (isLoading || isAuthenticated) {
    return (
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
        <main className="relative mx-auto flex min-h-screen max-w-[1200px] items-center justify-center px-4 py-10 md:px-8 lg:px-10">
          <div className="rounded-3xl border border-white/10 bg-slate-900/70 px-6 py-5 text-sm text-slate-300 backdrop-blur-xl">
            Preparing your session
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute -left-20 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-orange-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto grid min-h-screen max-w-[1280px] gap-8 px-4 py-10 md:px-8 lg:grid-cols-[0.95fr_1.05fr] lg:px-10">
        <section className="flex flex-col justify-center">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Alpha Control Plane</p>
          <h1 className="mt-3 text-4xl font-semibold tracking-tight text-white md:text-5xl">
            Role-aware billing operations without exposing the engine behind it
          </h1>
          <p className="mt-4 max-w-2xl text-base text-slate-300">
            Sign in with your account credentials to open the correct control surface. Platform accounts cover billing connections and workspaces. Tenant accounts cover customers, payments, recovery, and explainability inside assigned workspaces.
          </p>

          <div className="mt-8 grid gap-4 md:grid-cols-3">
            <FeatureCard icon={<CreditCard className="h-5 w-5" />} title="Platform billing" body="Own Stripe connection records in Alpha, then assign those connections to workspaces." />
            <FeatureCard icon={<Building2 className="h-5 w-5" />} title="Workspace setup" body="Create workspace boundaries, attach connected billing, and hand off the first admin credential." />
            <FeatureCard icon={<UserRoundPlus className="h-5 w-5" />} title="Tenant operations" body="Run customer onboarding, payment setup, diagnostics, and recovery inside one workspace surface." />
          </div>

          <p className="mt-6 text-xs uppercase tracking-[0.14em] text-slate-400">
            API keys remain for API and integration traffic only. Browser sessions are now derived from human account credentials.
          </p>
          <Link
            href="/control-plane"
            className="mt-6 inline-flex h-10 w-fit items-center rounded-xl border border-white/10 bg-white/5 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-100 transition hover:bg-white/10"
          >
            Open product overview
          </Link>
        </section>

        <section className="flex items-center">
          {resetState === "success" ? (
            <div className="mr-6 w-full max-w-md rounded-3xl border border-emerald-400/30 bg-emerald-500/10 p-5 text-sm text-emerald-100 backdrop-blur-xl">
              Password updated. Sign in with your new password.
            </div>
          ) : null}
          {authError ? (
            <div className="sr-only">{authError}</div>
          ) : null}
          <SessionLoginCard
            passwordResetEnabled={Boolean(authProvidersQuery.data?.password_reset_enabled)}
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
          {authProvidersQuery.data?.sso_providers?.length ? (
            <div className="ml-6 w-full max-w-md rounded-3xl border border-white/10 bg-slate-900/55 p-5 backdrop-blur-xl">
              <p className="text-xs uppercase tracking-[0.18em] text-cyan-300/80">Single sign-on</p>
              <h2 className="mt-2 text-xl font-semibold text-white">Continue with your identity provider</h2>
              <p className="mt-2 text-sm text-slate-300">
                Use SSO for browser sessions. API keys stay on API and integration traffic only.
              </p>
              <div className="mt-4 grid gap-3">
                {authProvidersQuery.data.sso_providers.map((provider) => (
                  <a
                    key={provider.key}
                    href={buildSSOStartURL(apiBaseURL, provider.key, requestedNext)}
                    className="inline-flex h-11 items-center justify-center rounded-xl border border-cyan-400/30 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-50 transition hover:bg-cyan-500/20"
                  >
                    Continue with {provider.display_name}
                  </a>
                ))}
              </div>
              {(providerKey || authError) && (
                <p className="mt-3 text-xs text-amber-200">
                  {resolveAuthErrorMessage(providerKey, authError)}
                </p>
              )}
            </div>
          ) : null}
        </section>
      </main>
    </div>
  );
}

function FeatureCard({ icon, title, body }: { icon: React.ReactNode; title: string; body: string }) {
  return (
    <div className="rounded-3xl border border-white/10 bg-slate-900/55 p-5 backdrop-blur-xl">
      <span className="inline-flex rounded-2xl border border-cyan-400/30 bg-cyan-500/10 p-3 text-cyan-100">{icon}</span>
      <h2 className="mt-4 text-lg font-semibold text-white">{title}</h2>
      <p className="mt-2 text-sm text-slate-300">{body}</p>
    </div>
  );
}

function buildSSOStartURL(apiBaseURL: string, providerKey: string, nextPath: string | null): string {
  const baseURL = apiBaseURL.replace(/\/+$/, "");
  const url = new URL(`${baseURL}/v1/ui/auth/sso/${encodeURIComponent(providerKey)}/start`);
  if (nextPath) {
    url.searchParams.set("next", normalizeNextPath(nextPath, "/"));
  }
  return url.toString()
}

function resolveAuthErrorMessage(providerKey: string | null, errorCode: string | null): string {
  const label = providerKey ? ` for ${providerKey}` : "";
  switch (errorCode) {
    case "sso_user_not_provisioned":
      return `No browser account is provisioned${label}. Ask an admin to grant platform or tenant access first.`;
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
