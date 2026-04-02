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
      <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
        <main className="mx-auto flex min-h-screen max-w-[1200px] items-center justify-center px-4 py-10 md:px-8 lg:px-10">
          <div className="rounded-2xl border border-stone-200 bg-white px-6 py-5 text-sm text-slate-600 shadow-sm">
            Redirecting to your workspace
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto grid min-h-screen max-w-[1360px] gap-10 px-4 py-10 md:px-8 xl:grid-cols-[minmax(0,1fr)_460px] xl:gap-12 xl:px-10">
        <section className="flex flex-col justify-center xl:pr-8">
          <p className="text-xs uppercase tracking-[0.24em] text-slate-500">Alpha Control Plane</p>
          <h1 className="mt-3 max-w-[11ch] text-4xl font-semibold tracking-tight text-slate-950 md:text-5xl xl:text-6xl">
            Role-aware billing operations without exposing the engine behind it
          </h1>
          <p className="mt-4 max-w-3xl text-base leading-8 text-slate-600">
            Sign in with your account credentials to open the correct control surface. Platform accounts cover billing connections and workspaces. Tenant accounts cover customers, payments, recovery, and explainability inside assigned workspaces.
          </p>
          <div className="mt-6 grid gap-3 lg:grid-cols-3">
            <OperatorHint title="Identity rule" body="Browser login is only for human operators. API keys stay on API and integration traffic." />
            <OperatorHint title="Scope rule" body="Platform accounts open cross-workspace administration. Workspace accounts open one assigned workspace surface at a time." />
            <OperatorHint title="Session rule" body="Use the requested path when present. Alpha will route the session to the correct surface after authentication." />
          </div>

          <section className="mt-8 rounded-3xl border border-stone-200 bg-white p-5 shadow-sm">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Operating surfaces</p>
            <div className="mt-4 grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
              <FeatureCard icon={<CreditCard className="h-5 w-5" />} title="Platform billing" body="Own Stripe connection records in Alpha, then assign those connections to workspaces." />
              <FeatureCard icon={<Building2 className="h-5 w-5" />} title="Workspace setup" body="Create workspace boundaries, attach connected billing, and hand off the first admin credential." />
              <FeatureCard icon={<UserRoundPlus className="h-5 w-5" />} title="Tenant operations" body="Run customer onboarding, payment setup, diagnostics, and recovery inside one workspace surface." />
            </div>
          </section>

          <p className="mt-6 text-xs uppercase tracking-[0.14em] text-slate-500">
            API keys remain for API and integration traffic only. Browser sessions are now derived from human account credentials.
          </p>
          <Link
            href="/control-plane"
            prefetch={false}
            className="mt-6 inline-flex h-10 w-fit items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-slate-700 transition hover:border-stone-300 hover:bg-white"
          >
            Open product overview
          </Link>
        </section>

        <section className="flex flex-col justify-center gap-4 xl:max-w-[460px] xl:justify-center">
          {accessSwitch ? (
            <div className="w-full rounded-3xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-800 shadow-sm">
              Sign in with the account you want to use next. Alpha will replace the current browser session after successful login.
            </div>
          ) : null}
          {resetState === "success" ? (
            <div className="w-full rounded-3xl border border-emerald-200 bg-emerald-50 p-5 text-sm text-emerald-700 shadow-sm">
              Password updated. Sign in with your new password.
            </div>
          ) : null}
          {authError ? (
            <div className="sr-only">{authError}</div>
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
        </section>
      </main>
    </div>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}

function FeatureCard({ icon, title, body }: { icon: React.ReactNode; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
      <div className="flex items-start gap-3">
        <span className="inline-flex rounded-2xl border border-emerald-200 bg-emerald-50 p-3 text-emerald-700">{icon}</span>
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-slate-950">{title}</h2>
          <p className="mt-2 text-sm leading-7 text-slate-600">{body}</p>
        </div>
      </div>
    </div>
  );
}
