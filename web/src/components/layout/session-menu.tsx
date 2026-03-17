"use client";

import Link from "next/link";
import { LoaderCircle, LogOut, PanelsTopLeft, UserRoundCog } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";

export function SessionMenu() {
  const { session, scope, platformRole, csrfToken, logout, loggingOut } = useUISession();

  if (!session?.authenticated) {
    return null;
  }

  const accessLabel = scope === "platform" ? platformRole ?? "platform" : session.role ?? "reader";
  const contextLabel = scope === "platform" ? "Cross-workspace control" : session.tenant_id || "Tenant workspace";
  const homeHref = scope === "platform" ? "/billing-connections" : "/customers";
  const secondaryHref = scope === "platform" ? "/workspaces" : "/payment-operations";
  const secondaryLabel = scope === "platform" ? "Open workspaces" : "Open payments";

  return (
    <details className="group relative">
      <summary data-testid="session-menu-toggle" className="flex cursor-pointer list-none items-center gap-3 rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-left transition hover:bg-white/10">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border border-cyan-400/30 bg-cyan-500/10 text-cyan-100">
          <UserRoundCog className="h-4 w-4" />
        </span>
        <span className="min-w-0">
          <span className="block text-[10px] uppercase tracking-[0.16em] text-slate-400">Current access</span>
          <span className="block truncate text-xs font-semibold uppercase tracking-[0.12em] text-emerald-100">{accessLabel}</span>
          <span className="block truncate text-[11px] text-slate-300">{contextLabel}</span>
        </span>
      </summary>
      <div className="absolute right-0 z-30 mt-2 w-[280px] rounded-2xl border border-white/10 bg-slate-950/95 p-3 shadow-2xl backdrop-blur-xl">
        <div className="rounded-xl border border-white/10 bg-white/5 px-3 py-3">
          <p className="text-[10px] uppercase tracking-[0.16em] text-slate-400">Signed in surface</p>
          <p className="mt-1 text-sm font-semibold text-white">{scope === "platform" ? "Platform administration" : "Tenant operations"}</p>
          <p className="mt-1 text-xs text-slate-300">{contextLabel}</p>
        </div>
        <div className="mt-3 grid gap-2">
          <Link href={homeHref} className="inline-flex h-10 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-slate-100 transition hover:bg-white/10">
            <PanelsTopLeft className="h-3.5 w-3.5" />
            Open role home
          </Link>
          <Link href={secondaryHref} className="inline-flex h-10 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-slate-100 transition hover:bg-white/10">
            {secondaryLabel}
          </Link>
          <Link href="/login" className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-400/30 bg-amber-500/10 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-amber-100 transition hover:bg-amber-500/20">
            Sign in with different access
          </Link>
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => {
              void logout(csrfToken);
            }}
            className="inline-flex h-10 items-center justify-center gap-2 rounded-xl border border-rose-400/40 bg-rose-500/10 px-3 text-xs font-semibold uppercase tracking-[0.14em] text-rose-100 transition hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <LogOut className="h-3.5 w-3.5" />}
            End session
          </button>
        </div>
      </div>
    </details>
  );
}
