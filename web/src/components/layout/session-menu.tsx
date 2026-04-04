"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LoaderCircle, LogOut, PanelsTopLeft, UserRoundCog } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";

export function SessionMenu() {
  const { session, scope, platformRole, csrfToken, logout, loggingOut } = useUISession();
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const pathname = usePathname();

  useEffect(() => {
    const handlePointerDown = (event: PointerEvent) => {
      if (!detailsRef.current?.open) return;
      if (event.target instanceof Node && !detailsRef.current.contains(event.target)) {
        detailsRef.current.open = false;
      }
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && detailsRef.current?.open) {
        detailsRef.current.open = false;
      }
    };

    document.addEventListener("pointerdown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("pointerdown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  if (!session?.authenticated) {
    return null;
  }

  const accessLabel = scope === "platform" ? platformRole ?? "platform" : session.role ?? "reader";
  const contextLabel = scope === "platform" ? "Platform admin" : session.tenant_id || "Workspace";
  const identityLabel = session.user_email || contextLabel;
  const homeHref = "/control-plane";
  const secondaryHref = scope === "platform" ? "/workspaces" : "/workspace-access";
  const secondaryLabel = scope === "platform" ? "Workspaces" : "Access";
  const closeMenu = () => {
    if (detailsRef.current?.open) {
      detailsRef.current.open = false;
    }
  };

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
      <div className="absolute bottom-full left-0 z-30 mb-1 w-[200px] rounded-lg border border-stone-200 bg-white shadow-lg">
        <div className="px-3 py-2.5">
          <p className="truncate text-xs font-medium text-slate-900">{identityLabel}</p>
          <p className="mt-0.5 text-[11px] text-slate-400">{contextLabel}</p>
        </div>
        <div className="border-t border-stone-100" />
        <div className="p-1">
          <Link
            href={homeHref}
            prefetch={false}
            onClick={closeMenu}
            className="flex h-7 items-center gap-2 rounded-md px-2 text-xs text-slate-600 transition hover:bg-stone-50"
          >
            <PanelsTopLeft className="h-3 w-3 text-slate-400" />
            Home
          </Link>
          <Link
            href={secondaryHref}
            prefetch={false}
            onClick={closeMenu}
            className="flex h-7 items-center gap-2 rounded-md px-2 text-xs text-slate-600 transition hover:bg-stone-50"
          >
            {secondaryLabel}
          </Link>
        </div>
        <div className="border-t border-stone-100" />
        <div className="p-1">
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => {
              closeMenu();
              void logout(csrfToken);
            }}
            className="flex h-7 w-full items-center gap-2 rounded-md px-2 text-xs text-rose-600 transition hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <LogOut className="h-3 w-3" />}
            Sign out
          </button>
        </div>
      </div>
    </details>
  );
}
