"use client";

import type { ComponentType } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Activity,
  ArrowRightLeft,
  BellRing,
  Building2,
  CircleDollarSign,
  CreditCard,
  FileSearch,
  Home,
  Layers3,
  ReceiptText,
  ShieldCheck,
  UserRoundPlus,
  Workflow,
} from "lucide-react";

import { useUISession } from "@/hooks/use-ui-session";
import { SessionMenu } from "@/components/layout/session-menu";

type NavScope = "platform" | "tenant";

type NavItem = {
  href: string;
  label: string;
  scope: NavScope;
  icon: ComponentType<{ className?: string }>;
};

const platformItems: NavItem[] = [
  { href: "/control-plane", label: "Overview", scope: "platform", icon: Home },
  { href: "/billing-connections", label: "Billing Connections", scope: "platform", icon: CreditCard },
  { href: "/workspaces", label: "Workspaces", scope: "platform", icon: Building2 },
  { href: "/tenant-audit", label: "Audit", scope: "platform", icon: FileSearch },
  { href: "/workspaces/new", label: "Workspace Setup", scope: "platform", icon: Layers3 },
];

const tenantRevenueItems: NavItem[] = [
  { href: "/pricing", label: "Pricing", scope: "tenant", icon: CircleDollarSign },
  { href: "/customers", label: "Customers", scope: "tenant", icon: UserRoundPlus },
  { href: "/subscriptions", label: "Subscriptions", scope: "tenant", icon: ArrowRightLeft },
  { href: "/invoices", label: "Invoices", scope: "tenant", icon: ReceiptText },
  { href: "/payments", label: "Payments", scope: "tenant", icon: CreditCard },
];

const tenantOperationsItems: NavItem[] = [
  { href: "/workspace-access", label: "Access", scope: "tenant", icon: ShieldCheck },
  { href: "/usage-events", label: "Usage", scope: "tenant", icon: Activity },
  { href: "/dunning", label: "Dunning", scope: "tenant", icon: BellRing },
  { href: "/replay-operations", label: "Replay", scope: "tenant", icon: Workflow },
  { href: "/invoice-explainability", label: "Explainability", scope: "tenant", icon: Layers3 },
];

function isActivePath(pathname: string, href: string): boolean {
  if (href === "/billing-connections") {
    return pathname === "/billing-connections" || (pathname.startsWith("/billing-connections/") && pathname !== "/billing-connections/new");
  }
  if (href === "/workspaces") {
    return pathname === "/workspaces" || (pathname.startsWith("/workspaces/") && pathname !== "/workspaces/new");
  }
  if (href === "/tenant-audit") {
    return pathname === "/tenant-audit";
  }
  if (href === "/customers") {
    return pathname === "/customers" || (pathname.startsWith("/customers/") && pathname !== "/customers/new");
  }
  if (href === "/subscriptions") {
    return pathname === "/subscriptions" || pathname.startsWith("/subscriptions/");
  }
  if (href === "/invoices") {
    return pathname === "/invoices" || pathname.startsWith("/invoices/");
  }
  if (href === "/payments") {
    return pathname === "/payments" || pathname.startsWith("/payments/") || pathname === "/payment-operations";
  }
  if (href === "/usage-events") {
    return pathname === "/usage-events";
  }
  if (href === "/dunning") {
    return pathname === "/dunning" || pathname.startsWith("/dunning/");
  }
  if (href === "/pricing") {
    return pathname === "/pricing" || pathname.startsWith("/pricing/");
  }
  return pathname === href;
}

function NavSection({
  title,
  items,
  pathname,
}: {
  title: string;
  items: NavItem[];
  pathname: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-400">{title}</p>
      <div className="flex flex-wrap gap-1.5">
        {items.map((item) => {
          const Icon = item.icon;
          const active = isActivePath(pathname, item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              prefetch={false}
              className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition ${
                active
                  ? "border-slate-900 bg-slate-900 text-white"
                  : "border-stone-200 bg-stone-50 text-slate-600 hover:border-stone-300 hover:bg-white hover:text-slate-900"
              }`}
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export function ControlPlaneNav() {
  const pathname = usePathname();
  const { isAuthenticated, isLoading, session, scope } = useUISession();

  const contextLabel = isLoading
    ? "Restoring session"
    : scope === "platform"
      ? "Platform admin"
      : session?.tenant_id
        ? `Workspace console · ${session.tenant_id}`
        : "Workspace console";

  const showPlatform = !isLoading && (!isAuthenticated || scope === "platform");
  const showTenant = !isLoading && (!isAuthenticated || scope === "tenant");

  return (
    <nav className="rounded-2xl border border-stone-200 bg-white shadow-sm">
      <div className="flex flex-col gap-3 p-4 md:p-4">
        <div className="flex items-center justify-between gap-4">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-slate-900">
              <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.5"/>
                <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.75"/>
                <rect x="12" y="2" width="3" height="14" rx="1" fill="white"/>
              </svg>
            </div>
            <div className="flex min-w-0 items-center gap-2.5">
              <p className="text-sm font-semibold tracking-tight text-slate-950">Alpha</p>
              <span className="hidden rounded-md border border-stone-200 bg-stone-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-500 sm:inline">
                {contextLabel}
              </span>
            </div>
          </div>
          {isAuthenticated ? <SessionMenu /> : null}
        </div>

        {isLoading ? (
          <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3 text-xs text-slate-500">
            Loading
          </div>
        ) : (
          <div className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            {showPlatform ? <NavSection title="Platform" items={platformItems} pathname={pathname} /> : null}
            {showTenant ? (
              <div className="grid gap-3">
                <NavSection title="Revenue" items={tenantRevenueItems} pathname={pathname} />
                <NavSection title="Operations" items={tenantOperationsItems} pathname={pathname} />
              </div>
            ) : null}
          </div>
        )}
      </div>
    </nav>
  );
}
