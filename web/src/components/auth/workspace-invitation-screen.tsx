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
      <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <div className="inline-flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-5 py-4 text-sm text-slate-600 shadow-sm">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspace invitation
          </div>
        </main>
      </div>
    );
  }

  if (previewQuery.isError || !previewQuery.data) {
    return (
      <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
            <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace invitation</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">This invitation is no longer available</h1>
            <p className="mt-3 text-sm text-slate-600">
              The invitation may have expired, been revoked, or already been accepted. Ask the workspace owner to issue a fresh invite if you still need access.
            </p>
            <div className="mt-5">
              <Link
                href="/login"
                className="inline-flex h-11 items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-700 transition hover:border-stone-300 hover:bg-white"
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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace invitation</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">Join {preview.workspace_name}</h1>
          <p className="mt-3 text-sm text-slate-600">
            This invite grants <span className="font-semibold text-slate-950">{preview.invitation.role}</span> access to the workspace.
          </p>

          <div className="mt-5 rounded-2xl border border-stone-200 bg-stone-50 p-4">
            <p className="flex items-center gap-2 text-sm font-semibold text-slate-950">
              <PanelsTopLeft className="h-4 w-4 text-emerald-700" />
              {preview.workspace_name}
            </p>
            <p className="mt-2 break-all text-xs text-slate-500">Workspace ID: {preview.invitation.workspace_id}</p>
            <p className="mt-2 break-all text-xs text-slate-500">Invited email: {preview.invitation.email}</p>
          </div>

          {preview.requires_login && !isAuthenticated ? (
            <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
              Use the invited email to continue. Alpha will return you to this invitation after authentication.
            </div>
          ) : null}

          {emailMismatch ? (
            <div className="mt-5 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
              You are signed in as {session?.user_email}, but this invite is for {preview.invitation.email}. Switch accounts before accepting.
            </div>
          ) : null}

          {acceptMutation.error ? <p className="mt-4 text-xs text-rose-700">{acceptMutation.error.message}</p> : null}
          {registerMutation.error ? <p className="mt-4 text-xs text-rose-700">{registerMutation.error.message}</p> : null}

          {!isAuthenticated && preview.requires_login ? (
            <div className="mt-5 grid gap-4">
              {authProvidersQuery.data?.sso_providers?.length ? (
                <div className="grid gap-3">
                  {authProvidersQuery.data.sso_providers.map((provider) => (
                    <a
                      key={provider.key}
                      href={buildSSOStartURL(apiBaseURL, provider.key, invitePath)}
                      className="inline-flex h-11 items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm font-medium text-slate-800 transition hover:border-stone-300 hover:bg-white"
                    >
                      Continue with {provider.display_name}
                    </a>
                  ))}
                </div>
              ) : null}

              {preview.account_exists ? (
                <Link
                  href={loginHref}
                  className="inline-flex h-11 items-center justify-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-700 transition hover:border-stone-300 hover:bg-white"
                >
                  Sign in with email and password
                </Link>
              ) : null}

              {showRegistrationForm ? (
                <form className="grid gap-3 rounded-2xl border border-stone-200 bg-stone-50 p-4" onSubmit={onRegister}>
                  <div>
                    <p className="text-sm font-semibold text-slate-950">Create your Alpha account</p>
                    <p className="mt-1 text-xs text-slate-500">
                      Use this path if this invited email does not already have an Alpha browser account.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Display name</label>
                    <input
                      type="text"
                      value={displayName}
                      onChange={(event) => setDisplayName(event.target.value)}
                      placeholder="Your name"
                      className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                    />
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-slate-500">Password</label>
                    <input
                      type="password"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      placeholder="At least 12 characters"
                      className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                    />
                  </div>
                  <button
                    type="submit"
                    disabled={registerMutation.isPending || !password.trim()}
                    className="inline-flex h-11 items-center justify-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
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
                className="inline-flex h-11 items-center rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
              >
                Sign in to continue
              </Link>
            ) : (
              <button
                type="button"
                onClick={() => acceptMutation.mutate()}
                disabled={!preview.can_accept || acceptMutation.isPending || !csrfToken}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {acceptMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailCheck className="h-4 w-4" />}
                Accept invitation
              </button>
            )}
            {isAuthenticated ? (
              <Link
                href="/login"
                className="inline-flex h-11 items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-700 transition hover:border-stone-300 hover:bg-white"
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
