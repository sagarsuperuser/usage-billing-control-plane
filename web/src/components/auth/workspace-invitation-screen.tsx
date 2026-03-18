"use client";

import { FormEvent, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { LoaderCircle, MailCheck, PanelsTopLeft } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { acceptWorkspaceInvitation, fetchUIAuthProviders, fetchWorkspaceInvitationPreview, registerWorkspaceInvitation } from "@/lib/api";
import { buildLoginPath, getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
import { useUISession } from "@/hooks/use-ui-session";
import { useSessionStore } from "@/store/use-session-store";

export function WorkspaceInvitationScreen({ token }: { token: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, session, isAuthenticated, isLoading } = useUISession();
  const { setSession } = useSessionStore();
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");

  const previewQuery = useQuery({
    queryKey: ["workspace-invitation", apiBaseURL, token],
    queryFn: () => fetchWorkspaceInvitationPreview({ runtimeBaseURL: apiBaseURL, token }),
    enabled: Boolean(apiBaseURL) && token.trim().length > 0,
    retry: false,
  });
  const authProvidersQuery = useQuery({
    queryKey: ["ui-auth-providers", apiBaseURL],
    queryFn: () => fetchUIAuthProviders({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(apiBaseURL),
    staleTime: 60_000,
  });

  const invitePath = useMemo(() => `/invite/${encodeURIComponent(token)}`, [token]);
  const acceptMutation = useMutation({
    mutationFn: () =>
      acceptWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        token,
      }),
    onSuccess: (payload) => {
      setSession(payload.session);
      queryClient.setQueryData(["ui-session", apiBaseURL], payload.session);
      router.replace(normalizeNextPath("/customers", getDefaultLandingPath(payload.session)));
    },
  });
  const registerMutation = useMutation({
    mutationFn: () =>
      registerWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        token,
        displayName,
        password,
      }),
    onSuccess: (payload) => {
      setSession(payload.session);
      queryClient.setQueryData(["ui-session", apiBaseURL], payload.session);
      setPassword("");
      router.replace(normalizeNextPath("/customers", getDefaultLandingPath(payload.session)));
    },
  });

  if (isLoading || previewQuery.isLoading) {
    return (
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
        <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <div className="inline-flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-900/70 px-5 py-4 text-sm text-slate-300 backdrop-blur-xl">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspace invitation
          </div>
        </main>
      </div>
    );
  }

  if (previewQuery.isError || !previewQuery.data) {
    return (
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
        <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace invitation</p>
            <h1 className="mt-2 text-2xl font-semibold text-white">This invitation is no longer available</h1>
            <p className="mt-3 text-sm text-slate-300">
              The invitation may have expired, been revoked, or already been accepted. Ask the workspace owner to issue a fresh invite if you still need access.
            </p>
            <div className="mt-5">
              <Link
                href="/login"
                className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Open login
              </Link>
            </div>
          </section>
        </main>
      </div>
    );
  }

  const preview = previewQuery.data;
  const loginHref = buildLoginPath(invitePath);
  const emailMismatch = isAuthenticated && preview.authenticated && !preview.email_matches_session;
  const showRegistrationForm = preview.requires_login && !isAuthenticated && !preview.account_exists;

  const onRegister = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    registerMutation.mutate();
  };

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <main className="relative mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace invitation</p>
          <h1 className="mt-2 text-2xl font-semibold text-white">Join {preview.workspace_name}</h1>
          <p className="mt-3 text-sm text-slate-300">
            This invite grants <span className="font-semibold text-white">{preview.invitation.role}</span> access to the workspace.
          </p>

          <div className="mt-5 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
            <p className="flex items-center gap-2 text-sm font-semibold text-white">
              <PanelsTopLeft className="h-4 w-4 text-cyan-300" />
              {preview.workspace_name}
            </p>
            <p className="mt-2 break-all text-xs text-slate-400">Workspace ID: {preview.invitation.workspace_id}</p>
            <p className="mt-2 break-all text-xs text-slate-400">Invited email: {preview.invitation.email}</p>
          </div>

          {preview.requires_login && !isAuthenticated ? (
            <div className="mt-5 rounded-2xl border border-amber-400/30 bg-amber-500/10 p-4 text-sm text-amber-100">
              Use the invited email to continue. Alpha will return you to this invitation after authentication.
            </div>
          ) : null}

          {emailMismatch ? (
            <div className="mt-5 rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
              You are signed in as {session?.user_email}, but this invite is for {preview.invitation.email}. Switch accounts before accepting.
            </div>
          ) : null}

          {acceptMutation.error ? <p className="mt-4 text-xs text-rose-200">{acceptMutation.error.message}</p> : null}
          {registerMutation.error ? <p className="mt-4 text-xs text-rose-200">{registerMutation.error.message}</p> : null}

          {!isAuthenticated && preview.requires_login ? (
            <div className="mt-5 grid gap-4">
              {authProvidersQuery.data?.sso_providers?.length ? (
                <div className="grid gap-3">
                  {authProvidersQuery.data.sso_providers.map((provider) => (
                    <a
                      key={provider.key}
                      href={buildSSOStartURL(apiBaseURL, provider.key, invitePath)}
                      className="inline-flex h-11 items-center justify-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-50 transition hover:bg-cyan-500/20"
                    >
                      Continue with {provider.display_name}
                    </a>
                  ))}
                </div>
              ) : null}

              {preview.account_exists ? (
                <Link
                  href={loginHref}
                  className="inline-flex h-11 items-center justify-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
                >
                  Sign in with email and password
                </Link>
              ) : null}

              {showRegistrationForm ? (
                <form className="grid gap-3 rounded-2xl border border-white/10 bg-slate-950/55 p-4" onSubmit={onRegister}>
                  <div>
                    <p className="text-sm font-semibold text-white">Create your Alpha account</p>
                    <p className="mt-1 text-xs text-slate-400">
                      Use this path if this invited email does not already have an Alpha browser account.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Display name</label>
                    <input
                      type="text"
                      value={displayName}
                      onChange={(event) => setDisplayName(event.target.value)}
                      placeholder="Your name"
                      className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
                    />
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-cyan-200">Password</label>
                    <input
                      type="password"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      placeholder="At least 12 characters"
                      className="h-11 rounded-xl border border-white/20 bg-slate-950/60 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
                    />
                  </div>
                  <button
                    type="submit"
                    disabled={registerMutation.isPending || !password.trim()}
                    className="inline-flex h-11 items-center justify-center rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm font-medium text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {registerMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                    Create account and join workspace
                  </button>
                </form>
              ) : null}
            </div>
          ) : null}

          <div className="mt-5 flex flex-wrap gap-3">
            {!isAuthenticated || preview.requires_login ? (
              <Link
                href={loginHref}
                className="inline-flex h-11 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
              >
                Sign in to continue
              </Link>
            ) : (
              <button
                type="button"
                onClick={() => acceptMutation.mutate()}
                disabled={!preview.can_accept || acceptMutation.isPending || !csrfToken}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm font-medium text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {acceptMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailCheck className="h-4 w-4" />}
                Accept invitation
              </button>
            )}
            {isAuthenticated ? (
              <Link
                href="/login"
                className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Switch account
              </Link>
            ) : null}
          </div>
        </section>
      </main>
    </div>
  );
}

function buildSSOStartURL(apiBaseURL: string, providerKey: string, nextPath: string): string {
  const baseURL = apiBaseURL.replace(/\/+$/, "");
  const url = new URL(`${baseURL}/v1/ui/auth/sso/${encodeURIComponent(providerKey)}/start`);
  url.searchParams.set("next", normalizeNextPath(nextPath, "/"));
  return url.toString();
}
