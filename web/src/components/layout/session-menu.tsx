"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  Check,
  ChevronsUpDown,
  LoaderCircle,
  LogOut,
  PanelsTopLeft,
  Plus,
  UserRoundCog,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchSessionWorkspaces, switchWorkspace } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function SessionMenu() {
  const { session, apiBaseURL, csrfToken, logout, loggingOut, setSession } = useUISession();
  const queryClient = useQueryClient();
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const [showWorkspaces, setShowWorkspaces] = useState(false);

  useEffect(() => {
    const handlePointerDown = (event: PointerEvent) => {
      if (!detailsRef.current?.open) return;
      if (event.target instanceof Node && !detailsRef.current.contains(event.target)) {
        detailsRef.current.open = false;
        setShowWorkspaces(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && detailsRef.current?.open) {
        detailsRef.current.open = false;
        setShowWorkspaces(false);
      }
    };
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  const workspacesQuery = useQuery({
    queryKey: ["session-workspaces", apiBaseURL],
    queryFn: () => fetchSessionWorkspaces({ runtimeBaseURL: apiBaseURL }),
    enabled: Boolean(session?.authenticated) && showWorkspaces,
    staleTime: 30_000,
  });

  const switchMutation = useMutation({
    mutationFn: (tenantID: string) => switchWorkspace({ runtimeBaseURL: apiBaseURL, csrfToken, tenantID }),
    onSuccess: (newSession) => {
      setSession(newSession);
      queryClient.setQueryData(["ui-session", apiBaseURL], newSession);
      queryClient.invalidateQueries();
      setShowWorkspaces(false);
      if (detailsRef.current) detailsRef.current.open = false;
      window.location.assign("/control-plane");
    },
  });

  if (!session?.authenticated) return null;

  const accessLabel = session.role ?? "reader";
  const contextLabel = session.tenant_id || "Workspace";
  const identityLabel = session.user_email || contextLabel;
  const closeMenu = () => {
    if (detailsRef.current?.open) detailsRef.current.open = false;
    setShowWorkspaces(false);
  };

  const workspaces = workspacesQuery.data?.items ?? [];
  const currentTenantID = workspacesQuery.data?.current_tenant_id ?? session.tenant_id ?? "";
  const hasMultipleWorkspaces = workspaces.length > 1;

  return (
    <details ref={detailsRef} className="group relative">
      <summary
        data-testid="session-menu-toggle"
        className="flex w-full cursor-pointer list-none items-center gap-2.5 rounded-md px-2 py-1.5 text-left transition hover:bg-stone-50"
      >
        <span className="inline-flex h-6 w-6 items-center justify-center rounded-md bg-slate-100 text-slate-600">
          <UserRoundCog className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13px] font-medium text-slate-700">{accessLabel}</span>
          <span className="block truncate text-[11px] text-slate-400">{identityLabel}</span>
        </span>
      </summary>
      <div className="absolute bottom-full left-0 z-30 mb-1 w-[220px] rounded-lg border border-stone-200 bg-white shadow-lg">
        {/* Identity */}
        <div className="px-3 py-2.5">
          <p className="truncate text-xs font-medium text-slate-900">{identityLabel}</p>
          <p className="mt-0.5 truncate text-[11px] text-slate-400">{contextLabel}</p>
        </div>

        <div className="border-t border-stone-100" />

        {/* Workspace switcher */}
        <div className="p-1">
          <button
            type="button"
            onClick={() => setShowWorkspaces(!showWorkspaces)}
            className="flex h-7 w-full items-center justify-between gap-2 rounded-md px-2 text-xs text-slate-600 transition hover:bg-stone-50"
          >
            <span className="flex items-center gap-2">
              <ChevronsUpDown className="h-3 w-3 text-slate-400" />
              Switch workspace
            </span>
          </button>

          {showWorkspaces && (
            <div className="mt-1 rounded-md border border-stone-100 bg-stone-50 p-1">
              {workspacesQuery.isLoading ? (
                <div className="flex items-center gap-2 px-2 py-2 text-[11px] text-slate-400">
                  <LoaderCircle className="h-3 w-3 animate-spin" />
                  Loading...
                </div>
              ) : workspaces.length === 0 ? (
                <p className="px-2 py-2 text-[11px] text-slate-400">No workspaces available.</p>
              ) : (
                <>
                  {workspaces.map((ws) => {
                    const isCurrent = ws.tenant_id === currentTenantID;
                    return (
                      <button
                        key={ws.tenant_id}
                        type="button"
                        disabled={isCurrent || switchMutation.isPending}
                        onClick={() => switchMutation.mutate(ws.tenant_id)}
                        className={`flex h-7 w-full items-center gap-2 rounded px-2 text-left text-[11px] transition ${
                          isCurrent
                            ? "font-medium text-slate-900"
                            : "text-slate-600 hover:bg-white disabled:opacity-50"
                        }`}
                      >
                        {isCurrent ? <Check className="h-3 w-3 text-emerald-600" /> : <span className="h-3 w-3" />}
                        <span className="min-w-0 flex-1 truncate">{ws.name}</span>
                        <span className="text-[10px] text-slate-400">{ws.role}</span>
                      </button>
                    );
                  })}
                  {hasMultipleWorkspaces ? <div className="my-0.5 border-t border-stone-200" /> : null}
                  <Link
                    href="/workspace-setup"
                    onClick={closeMenu}
                    className="flex h-7 w-full items-center gap-2 rounded px-2 text-[11px] text-slate-500 transition hover:bg-white"
                  >
                    <Plus className="h-3 w-3" />
                    New workspace
                  </Link>
                </>
              )}
            </div>
          )}
        </div>

        <div className="border-t border-stone-100" />

        {/* Navigation */}
        <div className="p-1">
          <Link
            href="/control-plane"
            prefetch={false}
            onClick={closeMenu}
            className="flex h-7 items-center gap-2 rounded-md px-2 text-xs text-slate-600 transition hover:bg-stone-50"
          >
            <PanelsTopLeft className="h-3 w-3 text-slate-400" />
            Home
          </Link>
        </div>

        <div className="border-t border-stone-100" />

        {/* Sign out */}
        <div className="p-1">
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => { closeMenu(); void logout(csrfToken); }}
            className="flex h-7 w-full items-center gap-2 rounded-md px-2 text-xs text-rose-600 transition hover:bg-rose-50 disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <LogOut className="h-3 w-3" />}
            Sign out
          </button>
        </div>
      </div>
    </details>
  );
}
