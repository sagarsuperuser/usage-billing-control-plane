"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle, PanelsTopLeft } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useUISession } from "@/hooks/use-ui-session";
import { fetchPendingWorkspaceSelection, selectPendingWorkspace } from "@/lib/api";
import { buildLoginPath, getDefaultLandingPath, normalizeNextPath } from "@/lib/session-routing";
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
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
        <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <div className="inline-flex items-center gap-2 rounded-2xl border border-white/10 bg-slate-900/70 px-5 py-4 text-sm text-slate-300 backdrop-blur-xl">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            Loading workspace choices
          </div>
        </main>
      </div>
    );
  }

  if (selectionQuery.isError || !selectionQuery.data?.required) {
    return (
      <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
        <main className="relative mx-auto flex min-h-screen max-w-[720px] items-center justify-center px-4 py-10">
          <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace selection</p>
            <h1 className="mt-2 text-2xl font-semibold text-white">No workspace selection is pending</h1>
            <p className="mt-3 text-sm text-slate-300">
              Start a new browser session first. Alpha only shows this chooser after a user has authenticated and more than one workspace is available.
            </p>
            <div className="mt-5 flex flex-wrap gap-3">
              <Link
                href={buildLoginPath(requestedNext || "/customers")}
                className="inline-flex h-11 items-center rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20"
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
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <main className="relative mx-auto flex min-h-screen max-w-[760px] items-center px-4 py-10">
        <section className="w-full rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workspace selection</p>
          <h1 className="mt-2 text-2xl font-semibold text-white">Choose the workspace you want to open</h1>
          <p className="mt-3 text-sm text-slate-300">
            {selectionQuery.data.user_email || "This account"} has access to more than one workspace. Pick the workspace you want for this browser session.
          </p>

          <div className="mt-5 grid gap-3">
            {selectionQuery.data.items.map((item) => (
              <button
                key={item.tenant_id}
                type="button"
                onClick={() => selectMutation.mutate(item.tenant_id)}
                disabled={selectMutation.isPending}
                className="rounded-2xl border border-white/10 bg-slate-950/55 px-4 py-4 text-left transition hover:bg-white/10 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="flex items-center gap-2 text-sm font-semibold text-white">
                      <PanelsTopLeft className="h-4 w-4 text-cyan-300" />
                      <span className="truncate">{item.name}</span>
                    </p>
                    <p className="mt-1 break-all font-mono text-xs text-slate-400">{item.tenant_id}</p>
                  </div>
                  <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-[11px] uppercase tracking-[0.14em] text-slate-200">
                    {item.role}
                  </span>
                </div>
              </button>
            ))}
          </div>

          {selectMutation.error ? <p className="mt-4 text-xs text-rose-200">{selectMutation.error.message}</p> : null}
          <div className="mt-5 flex flex-wrap gap-3">
            <Link
              href={buildLoginPath(requestedNext || "/customers")}
              className="inline-flex h-11 items-center rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
            >
              Start over
            </Link>
          </div>
        </section>
      </main>
    </div>
  );
}
