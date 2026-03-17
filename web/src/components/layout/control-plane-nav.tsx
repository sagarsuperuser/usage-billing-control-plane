"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useUISession } from "@/hooks/use-ui-session";
import { SessionMenu } from "@/components/layout/session-menu";

const links = [
  { href: "/control-plane", label: "Overview" },
  { href: "/billing-connections", label: "Billing Connections", scope: "platform" as const },
  { href: "/workspaces", label: "Workspaces", scope: "platform" as const },
  { href: "/workspaces/new", label: "Workspace Setup", scope: "platform" as const },
  { href: "/pricing", label: "Pricing", scope: "tenant" as const },
  { href: "/customers", label: "Customers", scope: "tenant" as const },
  { href: "/customers/new", label: "Customer Setup", scope: "tenant" as const },
  { href: "/payment-operations", label: "Payments", scope: "tenant" as const },
  { href: "/replay-operations", label: "Recovery", scope: "tenant" as const },
  { href: "/invoice-explainability", label: "Explainability", scope: "tenant" as const },
];

function isActivePath(pathname: string, href: string): boolean {
  if (href === "/billing-connections") {
    return pathname === "/billing-connections" || (pathname.startsWith("/billing-connections/") && pathname !== "/billing-connections/new");
  }
  if (href === "/workspaces") {
    return pathname === "/workspaces" || (pathname.startsWith("/workspaces/") && pathname !== "/workspaces/new");
  }
  if (href === "/customers") {
    return pathname === "/customers" || (pathname.startsWith("/customers/") && pathname !== "/customers/new");
  }
  if (href === "/pricing") {
    return pathname === "/pricing" || pathname.startsWith("/pricing/");
  }
  return pathname === href;
}

export function ControlPlaneNav() {
  const pathname = usePathname();
  const { isAuthenticated, session, scope } = useUISession();
  const scopeLabel = scope === "platform" ? "Platform shell" : session?.tenant_id ? `Workspace ${session.tenant_id}` : "Tenant shell";
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
            const active = isActivePath(pathname, link.href);
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
            <p className="text-[10px] uppercase tracking-[0.16em] text-slate-400">Current context</p>
            <p className="mt-1 text-xs font-semibold uppercase tracking-[0.12em] text-cyan-100">{scopeLabel}</p>
          </div>
          <SessionMenu />
        </div>
      ) : null}
    </nav>
  );
}
