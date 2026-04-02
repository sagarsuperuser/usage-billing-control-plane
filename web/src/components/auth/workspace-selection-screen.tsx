"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle, PanelsTopLeft } from "lucide-react";
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
      <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <div className="inline-flex items-center gap-2 rounded-2xl border border-stone-200 bg-white px-5 py-4 text-sm text-slate-600 shadow-sm">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspace choices
          </div>
        </main>
      </div>
    );
  }

  if (selectionQuery.isError || !selectionQuery.data?.required) {
    return (
      <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
        <main className="mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
            <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace selection</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">No workspace selection is pending</h1>
            <p className="mt-3 text-sm text-slate-600">
              Start a new browser session first. Alpha only shows this chooser after a user has authenticated and more than one workspace is available.
            </p>
            <div className="mt-5 flex flex-wrap gap-3">
              <Link
                href={buildAccessSwitchPath(requestedNext || "/customers")}
                className="inline-flex h-11 items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm font-medium text-slate-800 transition hover:border-stone-300 hover:bg-white"
              >
                Return to login
              </Link>
            </div>
          </section>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <p className="text-xs uppercase tracking-[0.2em] text-slate-500">Workspace selection</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">Choose the workspace you want to open</h1>
          <p className="mt-3 text-sm text-slate-600">
            {selectionQuery.data.user_email || "This account"} has access to more than one workspace. Pick the workspace you want for this browser session.
          </p>
          <div className="mt-5 grid gap-3 sm:grid-cols-3">
            <OperatorHint title="Selection rule" body="Pick the workspace you need for this session. Alpha scopes navigation and permissions to that choice." />
            <OperatorHint title="Role signal" body="Read the role chip before entering. It tells you the access level for that workspace only." />
            <OperatorHint title="Start-over rule" body="Use start over only when the current session choice is wrong or the requested path needs a different account." />
          </div>

          <div className="mt-5 grid gap-3">
            {selectionQuery.data.items.map((item) => (
              <button
                key={item.tenant_id}
                type="button"
                onClick={() => selectMutation.mutate(item.tenant_id)}
                disabled={selectMutation.isPending}
                className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-left transition hover:border-stone-300 hover:bg-white disabled:cursor-not-allowed disabled:opacity-60"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="flex items-center gap-2 text-sm font-semibold text-slate-950">
                      <PanelsTopLeft className="h-4 w-4 text-emerald-700" />
                      <span className="truncate">{item.name}</span>
                    </p>
                    <p className="mt-1 break-all font-mono text-xs text-slate-500">{item.tenant_id}</p>
                  </div>
                  <span className="rounded-full border border-stone-200 bg-white px-2 py-1 text-[11px] uppercase tracking-[0.14em] text-slate-700">
                    {item.role}
                  </span>
                </div>
              </button>
            ))}
          </div>

          {selectMutation.error ? <p className="mt-4 text-xs text-rose-700">{selectMutation.error.message}</p> : null}
          <div className="mt-5 flex flex-wrap gap-3">
            <Link
              href={buildAccessSwitchPath(requestedNext || "/customers")}
              className="inline-flex h-11 items-center rounded-xl border border-stone-200 bg-stone-50 px-4 text-sm text-slate-700 transition hover:border-stone-300 hover:bg-white"
            >
              Start over
            </Link>
          </div>
        </section>
      </main>
    </div>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
      <p className="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}
