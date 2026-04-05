
import { useEffect, useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  Building2,
  Check,
  ChevronRight,
  LoaderCircle,
  LogOut,
  Plus,
  Settings,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { fetchSessionWorkspaces, switchWorkspace } from "@/lib/api";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

export function SessionMenu() {
  const { session, apiBaseURL, csrfToken, logout, loggingOut, setSession } = useUISession();
  const queryClient = useQueryClient();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const [showWorkspaces, setShowWorkspaces] = useState(false);

  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (event: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
        setShowWorkspaces(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        setShowWorkspaces(false);
      }
    };
    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

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
      setOpen(false);
      setShowWorkspaces(false);
      window.location.assign("/control-plane");
    },
    onError: (err: Error) => showError(err.message),
  });

  if (!session?.authenticated) return null;

  const role = session.role ?? "reader";
  const tenantID = session.tenant_id || "";
  const email = session.user_email || "";
  const closeMenu = () => { setOpen(false); setShowWorkspaces(false); };

  const workspaces = workspacesQuery.data?.items ?? [];
  const currentTenantID = workspacesQuery.data?.current_tenant_id ?? tenantID;

  return (
    <div ref={containerRef} className="relative">
      {/* Trigger */}
      <button
        type="button"
        data-testid="session-menu-toggle"
        onClick={() => { setOpen(!open); if (open) setShowWorkspaces(false); }}
        className="flex w-full items-center gap-2.5 rounded-lg px-2 py-2 text-left transition hover:bg-stone-100/80"
      >
        <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-slate-700 to-slate-900 text-[11px] font-bold text-white">
          {(tenantID || email).charAt(0).toUpperCase()}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13px] font-semibold text-slate-800">{tenantID || "Workspace"}</span>
          <span className="block truncate text-[11px] text-slate-400">{email}</span>
        </span>
      </button>

      {/* Popover */}
      {open && (
        <div className="absolute bottom-full left-0 z-30 mb-1.5 w-[260px] overflow-hidden rounded-xl border border-stone-200 bg-white shadow-xl">
          {/* Workspace header */}
          <div className="bg-stone-50/80 px-3.5 py-3">
            <div className="flex items-center gap-2.5">
              <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-slate-700 to-slate-900 text-[11px] font-bold text-white">
                {(tenantID || email).charAt(0).toUpperCase()}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold text-slate-900">{tenantID || "Workspace"}</p>
                <p className="truncate text-[11px] text-slate-400">{email}</p>
              </div>
            </div>
            <div className="mt-2 flex items-center gap-1.5">
              <span className="inline-flex items-center rounded-full bg-slate-200/80 px-2 py-0.5 text-[10px] font-semibold text-slate-600">{role}</span>
              <span className="text-[10px] text-slate-400">access level</span>
            </div>
          </div>

          {/* Workspace switcher */}
          <div className="border-t border-stone-100">
            <button
              type="button"
              onClick={() => setShowWorkspaces(!showWorkspaces)}
              className="flex h-9 w-full items-center gap-2.5 px-3.5 text-xs text-slate-600 transition hover:bg-stone-50"
            >
              <Building2 className="h-3.5 w-3.5 text-slate-400" />
              <span className="flex-1 text-left">Switch workspace</span>
              <ChevronRight className={`h-3 w-3 text-slate-400 transition ${showWorkspaces ? "rotate-90" : ""}`} />
            </button>

            {showWorkspaces && (
              <div className="border-t border-stone-100 bg-stone-50/50 py-1">
                {workspacesQuery.isLoading ? (
                  <div className="flex items-center gap-2 px-3.5 py-2 text-[11px] text-slate-400">
                    <LoaderCircle className="h-3 w-3 animate-spin" />
                    Loading workspaces...
                  </div>
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
                          className={`flex h-8 w-full items-center gap-2.5 px-3.5 text-left text-xs transition ${
                            isCurrent ? "font-medium text-slate-900" : "text-slate-600 hover:bg-stone-100 disabled:opacity-50"
                          }`}
                        >
                          <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded bg-white text-[9px] font-bold text-slate-500 ring-1 ring-stone-200">
                            {ws.name.charAt(0).toUpperCase()}
                          </span>
                          <span className="min-w-0 flex-1 truncate">{ws.name}</span>
                          {isCurrent ? <Check className="h-3.5 w-3.5 shrink-0 text-emerald-500" /> : null}
                        </button>
                      );
                    })}
                    <Link
                      to="/workspace-setup"
                      onClick={closeMenu}
                      className="flex h-8 w-full items-center gap-2.5 px-3.5 text-xs text-slate-500 transition hover:bg-stone-100"
                    >
                      <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded border border-dashed border-stone-300">
                        <Plus className="h-2.5 w-2.5 text-slate-400" />
                      </span>
                      New workspace
                    </Link>
                  </>
                )}
              </div>
            )}
          </div>

          {/* Links */}
          <div className="border-t border-stone-100">
            <Link
              to="/workspace-access"
              onClick={closeMenu}
              className="flex h-9 items-center gap-2.5 px-3.5 text-xs text-slate-600 transition hover:bg-stone-50"
            >
              <Settings className="h-3.5 w-3.5 text-slate-400" />
              Workspace settings
            </Link>
          </div>

          {/* Sign out */}
          <div className="border-t border-stone-100">
            <button
              type="button"
              data-testid="session-logout"
              disabled={loggingOut || !csrfToken}
              onClick={() => { closeMenu(); void logout(csrfToken); }}
              className="flex h-9 w-full items-center gap-2.5 px-3.5 text-xs text-slate-600 transition hover:bg-stone-50 disabled:opacity-50"
            >
              {loggingOut ? <LoaderCircle className="h-3.5 w-3.5 animate-spin text-slate-400" /> : <LogOut className="h-3.5 w-3.5 text-slate-400" />}
              Sign out
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
