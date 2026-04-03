"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useUISession } from "@/hooks/use-ui-session";
import { fetchPendingWorkspaceSelection, selectPendingWorkspace } from "@/lib/api";
import { buildAccessSwitchPath, buildLoginPath, getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
import { useSessionStore } from "@/store/use-session-store";

export function WorkspaceSelectionScreen() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const { session, isAuthenticated, isLoading, apiBaseURL } = useUISession();
  const { setSession } = useSessionStore();
  const requestedNext = searchParams.get("next");

  const selectionQuery = useQuery({
    queryKey: ["ui-workspace-selection", apiBaseURL],
    queryFn: () => fetchPendingWorkspaceSelection({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(apiBaseURL),
    retry: false,
  });

  useEffect(() => {
    if (!isLoading && isAuthenticated && !selectionQuery.data?.required) {
      router.replace(normalizeNextPath(requestedNext, getDefaultLandingPath(session)));
    }
  }, [isAuthenticated, isLoading, requestedNext, router, selectionQuery.data?.required, session]);

  const selectMutation = useMutation({
    mutationFn: (tenantID: string) =>
      selectPendingWorkspace({
        runtimeBaseURL: apiBaseURL,
        csrfToken: selectionQuery.data?.csrf_token ?? "",
        tenantID,
      }),
    onSuccess: (nextSession) => {
      setSession(nextSession);
      queryClient.setQueryData(["ui-session", apiBaseURL], nextSession);
      router.replace(normalizeNextPath(requestedNext, getDefaultLandingPath(nextSession)));
    },
  });

  if (isLoading || selectionQuery.isLoading) {
    return (
      <div className="min-h-screen bg-[#f5f7fb]">
        <main className="flex min-h-screen items-center justify-center px-4">
          <div className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-5 py-4 text-sm text-slate-600 shadow-sm">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspaces…
          </div>
        </main>
      </div>
    );
  }

  if (selectionQuery.isError || !selectionQuery.data?.required) {
    return (
      <div className="min-h-screen bg-[#f5f7fb]">
        <main className="flex min-h-screen items-center justify-center px-4">
          <div className="w-full max-w-[440px]">
            <Logo />
            <div className="mt-8">
              <h1 className="text-base font-semibold text-slate-950">No workspace to select</h1>
              <p className="mt-2 text-sm text-slate-500">
                Sign in again — this chooser only appears when your account has access to more than one workspace.
              </p>
              <div className="mt-6">
                <Link
                  href={buildAccessSwitchPath(requestedNext || "/customers")}
                  className="inline-flex h-11 w-full items-center justify-center rounded-xl bg-slate-900 px-4 text-sm font-semibold text-white transition hover:bg-slate-800"
                >
                  Back to sign in
                </Link>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f5f7fb]">
      <main className="flex min-h-screen items-center justify-center px-4 py-10">
        <div className="w-full max-w-[440px]">
          <Logo />
          <div className="mt-8">
            <h1 className="text-base font-semibold text-slate-950">Choose a workspace</h1>
            <p className="mt-1.5 text-sm text-slate-500">
              {selectionQuery.data.user_email
                ? <>{selectionQuery.data.user_email} has access to multiple workspaces.</>
                : <>Your account has access to multiple workspaces.</>}
              {" "}Select one to continue.
            </p>

            <div className="mt-6 grid gap-2">
              {selectionQuery.data.items.map((item) => (
                <button
                  key={item.tenant_id}
                  type="button"
                  onClick={() => selectMutation.mutate(item.tenant_id)}
                  disabled={selectMutation.isPending}
                  className="group flex w-full items-center gap-3.5 rounded-xl border border-stone-200 bg-white px-4 py-3.5 text-left transition hover:border-slate-300 hover:shadow-sm disabled:cursor-not-allowed disabled:opacity-60"
                >
                  <span className="shrink-0 inline-flex h-9 w-9 items-center justify-center rounded-lg bg-slate-900 text-sm font-semibold text-white">
                    {item.name.charAt(0).toUpperCase()}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-semibold text-slate-900 truncate">{item.name}</p>
                    <p className="mt-0.5 font-mono text-[11px] text-slate-400 truncate">{item.tenant_id}</p>
                  </div>
                  <div className="shrink-0 flex items-center gap-2">
                    <span className="inline-flex items-center rounded-md bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-500">
                      {item.role}
                    </span>
                    <svg className="h-4 w-4 text-slate-300 transition group-hover:text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                    </svg>
                  </div>
                </button>
              ))}
            </div>

            {selectMutation.error ? (
              <p className="mt-4 text-xs text-rose-600">{selectMutation.error.message}</p>
            ) : null}

            <div className="mt-5 border-t border-stone-200 pt-5">
              <Link
                href={buildAccessSwitchPath(requestedNext || "/customers")}
                className="text-sm text-slate-400 transition hover:text-slate-700"
              >
                Sign in with a different account
              </Link>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

function Logo() {
  return (
    <div className="flex items-center gap-2.5">
      <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-900">
        <svg width="16" height="16" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.4"/>
          <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.65"/>
          <rect x="12" y="2" width="3" height="14" rx="1" fill="white"/>
        </svg>
      </div>
      <span className="text-sm font-semibold text-slate-900">Alpha</span>
    </div>
  );
}
