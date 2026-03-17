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
  const sessionLabel =
    scope === "platform"
      ? "Platform admin"
      : `${session?.role ?? "reader"}${session?.tenant_id ? ` · ${session.tenant_id}` : ""}`;
  const scopeLabel = scope === "platform" ? "Cross-workspace control" : "Tenant workspace tools";
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
    <nav className="grid gap-3 rounded-2xl border border-white/10 bg-slate-950/60 p-2 xl:grid-cols-[minmax(0,1fr)_auto] xl:items-center">
      <div className="flex min-w-0 flex-wrap items-center gap-3">
        <div className="shrink-0 rounded-xl border border-cyan-400/30 bg-cyan-500/10 px-3 py-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-cyan-100">Alpha</p>
          <p className="mt-1 text-[10px] uppercase tracking-[0.16em] text-cyan-200/80">Control Plane</p>
        </div>
        <div className="flex min-w-0 flex-wrap items-center gap-2">
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
        <div className="flex flex-wrap items-center justify-start gap-2 xl:justify-end">
          <div className="rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-right">
            <p className="text-[10px] uppercase tracking-[0.16em] text-slate-400">{scopeLabel}</p>
            <p className="mt-1 text-xs font-semibold uppercase tracking-[0.12em] text-emerald-100">
              {scope === "platform" ? platformRole ?? "platform" : sessionLabel}
            </p>
          </div>
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
