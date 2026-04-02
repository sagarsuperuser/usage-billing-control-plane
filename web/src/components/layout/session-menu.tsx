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
  const contextLabel = scope === "platform" ? "Cross-workspace control" : session.tenant_id || "Tenant workspace";
  const identityLabel = session.user_email || contextLabel;
  const homeHref = scope === "platform" ? "/billing-connections" : "/customers";
  const secondaryHref = scope === "platform" ? "/workspaces" : "/workspace-access";
  const secondaryLabel = scope === "platform" ? "Open workspaces" : "Open access";
  const closeMenu = () => {
    if (detailsRef.current?.open) {
      detailsRef.current.open = false;
    }
  };

  return (
    <details ref={detailsRef} className="group relative">
      <summary
        data-testid="session-menu-toggle"
        className="flex cursor-pointer list-none items-center gap-2 rounded-lg border border-stone-200 bg-stone-50 px-2.5 py-2 text-left transition hover:border-stone-300 hover:bg-white"
      >
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-slate-900 text-white">
          <UserRoundCog className="h-3.5 w-3.5" />
        </span>
        <span className="min-w-0 hidden sm:block">
          <span className="block truncate text-xs font-semibold text-slate-900">{accessLabel}</span>
          <span className="block truncate text-[11px] text-slate-500">{identityLabel}</span>
        </span>
      </summary>
      <div className="absolute right-0 z-30 mt-2 w-[260px] rounded-xl border border-stone-200 bg-white shadow-[0_8px_32px_rgba(15,23,42,0.10)]">
        <div className="px-4 py-3.5">
          <p className="text-sm font-semibold text-slate-900 truncate">{identityLabel}</p>
          <div className="mt-1.5 flex items-center gap-1.5">
            <span className="inline-flex items-center rounded-md bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600">
              {accessLabel}
            </span>
            {scope !== "platform" && contextLabel && (
              <span className="text-[11px] text-slate-400 truncate">{contextLabel}</span>
            )}
          </div>
        </div>
        <div className="border-t border-stone-100" />
        <div className="p-1.5">
          <Link
            href={homeHref}
            prefetch={false}
            onClick={closeMenu}
            className="flex h-8 items-center gap-2.5 rounded-lg px-2.5 text-sm text-slate-700 transition hover:bg-stone-50"
          >
            <PanelsTopLeft className="h-3.5 w-3.5 text-slate-400" />
            Home
          </Link>
          <Link
            href={secondaryHref}
            prefetch={false}
            onClick={closeMenu}
            className="flex h-8 items-center gap-2.5 rounded-lg px-2.5 text-sm text-slate-700 transition hover:bg-stone-50"
          >
            {secondaryLabel}
          </Link>
        </div>
        <div className="border-t border-stone-100" />
        <div className="p-1.5">
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => {
              closeMenu();
              void logout(csrfToken);
            }}
            className="flex h-8 w-full items-center gap-2.5 rounded-lg px-2.5 text-sm text-rose-600 transition hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <LogOut className="h-3.5 w-3.5" />}
            Sign out
          </button>
        </div>
      </div>
    </details>
  );
}
