import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { LoaderCircle, MailCheck, PanelsTopLeft } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { acceptWorkspaceInvitation, fetchUIAuthProviders, fetchWorkspaceInvitationPreview, registerWorkspaceInvitation } from "@/lib/api";
import { buildAccessSwitchPath, buildLoginPath, getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";
import { useSessionStore } from "@/store/use-session-store";

export function WorkspaceInvitationScreen({ token }: { token: string }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, session, isAuthenticated, isLoading } = useUISession();
  const { setSession } = useSessionStore();
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const autoAcceptStartedRef = useRef(false);

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
  const effectiveCSRFToken = csrfToken;
  const acceptMutation = useMutation({
    mutationFn: () =>
      acceptWorkspaceInvitation({
        runtimeBaseURL: apiBaseURL,
        csrfToken: effectiveCSRFToken,
        token,
      }),
    onSuccess: (payload) => {
      setSession(payload.session);
      queryClient.setQueryData(["ui-session", apiBaseURL], payload.session);
      navigate({ to: normalizeNextPath("/customers", getDefaultLandingPath(payload.session)), replace: true });
    },
    onError: (err: Error) => showError(err.message),
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
      navigate({ to: normalizeNextPath("/customers", getDefaultLandingPath(payload.session)), replace: true });
    },
    onError: (err: Error) => showError(err.message),
  });

  const onRegister = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    registerMutation.mutate();
  };

  useEffect(() => {
    const canAutoAccept = previewQuery.data?.can_accept && isAuthenticated;
    if (!canAutoAccept || !effectiveCSRFToken) {
      autoAcceptStartedRef.current = false;
      return;
    }
    if (acceptMutation.isPending || acceptMutation.isSuccess || autoAcceptStartedRef.current) {
      return;
    }
    autoAcceptStartedRef.current = true;
    acceptMutation.mutate();
  }, [acceptMutation, effectiveCSRFToken, isAuthenticated, previewQuery.data]);

  if (isLoading || previewQuery.isLoading) {
    return (
      <div className="text-text-primary">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <div className="inline-flex items-center gap-2 rounded-2xl border border-border bg-surface px-5 py-4 text-sm text-text-muted shadow-sm">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspace invitation
          </div>
        </main>
      </div>
    );
  }

  if (previewQuery.isError || !previewQuery.data) {
    return (
      <div className="text-text-primary">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <section className="w-full rounded-3xl border border-border bg-surface p-6 shadow-sm">
            <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Workspace invitation</p>
            <h1 className="mt-2 text-2xl font-semibold text-text-primary">This invitation is no longer available</h1>
            <p className="mt-3 text-sm text-text-muted">
              The invitation may have expired, been revoked, or already been accepted. Ask the workspace owner to issue a fresh invite if you still need access.
            </p>
            <div className="mt-5">
              <Link
                to="/login"
                className="inline-flex h-11 items-center rounded-xl border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:border-border hover:bg-surface"
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
  const accessSwitchHref = buildAccessSwitchPath(invitePath);
  const emailMismatch = isAuthenticated && preview.authenticated && !preview.email_matches_session;
  const showRegistrationForm = preview.requires_login && !isAuthenticated && !preview.account_exists;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex min-h-screen max-w-2xl items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-border bg-surface p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-text-muted">Workspace invitation</p>
          <h1 className="mt-2 text-2xl font-semibold text-text-primary">Join {preview.workspace_name}</h1>
          <p className="mt-3 text-sm text-text-muted">
            This invite grants <span className="font-semibold text-text-primary">{preview.invitation.role}</span> access to the workspace.
          </p>
          <div className="mt-5 grid gap-3 sm:grid-cols-3">
            <OperatorHint title="Invite rule" body="Use the invited email exactly. Alpha ties the invitation to that identity before granting workspace access." />
            <OperatorHint title="Auth rule" body="Continue with SSO or password only through this invite flow so Alpha can preserve the invitation context." />
            <OperatorHint title="Acceptance rule" body="If the account and email already match, Alpha should accept the invitation and route you into the workspace." />
          </div>

          <div className="mt-5 rounded-2xl border border-border bg-surface-secondary p-4">
            <p className="flex items-center gap-2 text-sm font-semibold text-text-primary">
              <PanelsTopLeft className="h-4 w-4 text-emerald-700" />
              {preview.workspace_name}
            </p>
            <p className="mt-2 break-all text-xs text-text-muted">Workspace ID: {preview.invitation.workspace_id}</p>
            <p className="mt-2 break-all text-xs text-text-muted">Invited email: {preview.invitation.email}</p>
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
                      className="inline-flex h-11 items-center justify-center rounded-xl border border-border bg-surface-secondary px-4 text-sm font-medium text-text-secondary transition hover:border-border hover:bg-surface"
                    >
                      Continue with {provider.display_name}
                    </a>
                  ))}
                </div>
              ) : null}

              {preview.account_exists ? (
                <Link
                  to={loginHref}
                  className="inline-flex h-11 items-center justify-center rounded-xl border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:border-border hover:bg-surface"
                >
                  Sign in with email and password
                </Link>
              ) : null}

              {showRegistrationForm ? (
                <form className="grid gap-3 rounded-2xl border border-border bg-surface-secondary p-4" onSubmit={onRegister}>
                  <div>
                    <p className="text-sm font-semibold text-text-primary">Create your Alpha account</p>
                    <p className="mt-1 text-xs text-text-muted">
                      Use this path if this invited email does not already have an Alpha browser account.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-text-muted">Display name</label>
                    <input
                      type="text"
                      value={displayName}
                      onChange={(event) => setDisplayName(event.target.value)}
                      placeholder="Your name"
                      className="h-11 rounded-xl border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                    />
                  </div>
                  <div className="grid gap-2">
                    <label className="text-xs font-medium uppercase tracking-wider text-text-muted">Password</label>
                    <input
                      type="password"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      placeholder="At least 12 characters"
                      className="h-11 rounded-xl border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
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

          {isAuthenticated ? (
            <div className="mt-5 flex flex-wrap gap-3">
              <button
                type="button"
                onClick={() => acceptMutation.mutate()}
                disabled={!preview.can_accept || acceptMutation.isPending || !effectiveCSRFToken}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {acceptMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <MailCheck className="h-4 w-4" />}
                Accept invitation
              </button>
              <Link
                to={accessSwitchHref}
                className="inline-flex h-11 items-center rounded-xl border border-border bg-surface-secondary px-4 text-sm text-text-secondary transition hover:border-border hover:bg-surface"
              >
                Switch account
              </Link>
            </div>
          ) : null}
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

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-border bg-surface-secondary px-4 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-text-muted">{title}</p>
      <p className="mt-2 text-sm leading-6 text-text-secondary">{body}</p>
    </div>
  );
}
