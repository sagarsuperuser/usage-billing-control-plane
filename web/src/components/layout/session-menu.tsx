"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LoaderCircle, LogOut, PanelsTopLeft, UserRoundCog } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";
import { buildAccessSwitchPath } from "@/lib/session-routing";

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
  const accessSwitchHref = buildAccessSwitchPath(pathname || homeHref);
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
      <div className="absolute right-0 z-30 mt-2 w-[300px] rounded-3xl border border-stone-200 bg-white p-3 shadow-[0_24px_60px_rgba(15,23,42,0.12)]">
        <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3">
          <p className="text-[10px] uppercase tracking-[0.16em] text-slate-500">Signed in surface</p>
          <p className="mt-1 text-sm font-semibold text-slate-950">{scope === "platform" ? "Platform administration" : "Workspace operations"}</p>
          <p className="mt-1 text-xs text-slate-600">{identityLabel}</p>
          <p className="mt-2 text-xs text-slate-500">{contextLabel}</p>
        </div>
        <div className="mt-3 grid gap-2">
          <Link
            href={homeHref}
            prefetch={false}
            onClick={closeMenu}
            className="inline-flex h-10 items-center gap-2 rounded-2xl border border-stone-200 bg-stone-50 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-slate-800 transition hover:border-stone-300 hover:bg-white"
          >
            <PanelsTopLeft className="h-3.5 w-3.5 text-slate-500" />
            Open role home
          </Link>
          <Link
            href={secondaryHref}
            prefetch={false}
            onClick={closeMenu}
            className="inline-flex h-10 items-center gap-2 rounded-2xl border border-stone-200 bg-stone-50 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-slate-800 transition hover:border-stone-300 hover:bg-white"
          >
            {secondaryLabel}
          </Link>
          <Link
            href={accessSwitchHref}
            prefetch={false}
            onClick={closeMenu}
            className="inline-flex h-10 items-center gap-2 rounded-2xl border border-amber-200 bg-amber-50 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-amber-800 transition hover:bg-amber-100"
          >
            Sign in with different access
          </Link>
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => {
              closeMenu();
              void logout(csrfToken);
            }}
            className="inline-flex h-10 items-center justify-center gap-2 rounded-2xl border border-rose-200 bg-rose-50 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-rose-800 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <LogOut className="h-3.5 w-3.5" />}
            End session
          </button>
        </div>
      </div>
    </details>
  );
}
