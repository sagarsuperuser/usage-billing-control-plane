"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { LoaderCircle, LogOut } from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";

const links = [
  { href: "/control-plane", label: "Overview" },
  { href: "/tenant-onboarding", label: "Workspace Setup", scope: "platform" as const },
  { href: "/customer-onboarding", label: "Customers", scope: "tenant" as const },
  { href: "/payment-operations", label: "Payments", scope: "tenant" as const },
  { href: "/replay-operations", label: "Recovery", scope: "tenant" as const },
  { href: "/invoice-explainability", label: "Explainability", scope: "tenant" as const },
];

export function ControlPlaneNav() {
  const pathname = usePathname();
  const { isAuthenticated, session, scope, platformRole, csrfToken, logout, loggingOut } = useUISession();
  const visibleLinks = links.filter((link) => {
    if (!("scope" in link) || !link.scope) {
      return true;
    }
    if (!isAuthenticated) {
      return true;
    }
    return link.scope === scope;
  });

  return (
    <nav className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-white/10 bg-slate-950/60 p-2">
      <div className="flex items-center gap-3">
        <div className="rounded-xl border border-cyan-400/30 bg-cyan-500/10 px-3 py-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-cyan-100">Alpha</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
        {visibleLinks.map((link) => {
          const active = pathname === link.href;
          return (
            <Link
              key={link.href}
              href={link.href}
              className={`rounded-xl px-3 py-2 text-xs font-semibold uppercase tracking-[0.14em] transition ${
                active
                  ? "bg-cyan-400/20 text-cyan-100"
                  : "text-slate-300 hover:bg-white/10 hover:text-white"
              }`}
            >
              {link.label}
            </Link>
          );
        })}
        </div>
      </div>
      {isAuthenticated ? (
        <div className="flex items-center gap-2">
          <span className="rounded-lg border border-emerald-400/40 bg-emerald-500/15 px-2 py-1 text-[11px] uppercase tracking-[0.12em] text-emerald-100">
            {scope === "platform" ? platformRole ?? "platform" : session?.role ?? "reader"}
          </span>
          <button
            type="button"
            data-testid="session-logout"
            disabled={loggingOut || !csrfToken}
            onClick={() => {
              void logout(csrfToken);
            }}
            className="inline-flex h-8 items-center gap-1 rounded-lg border border-rose-400/40 bg-rose-500/10 px-2 text-xs uppercase tracking-[0.12em] text-rose-100 transition hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loggingOut ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <LogOut className="h-3.5 w-3.5" />}
            Logout
          </button>
        </div>
      ) : null}
    </nav>
  );
}
