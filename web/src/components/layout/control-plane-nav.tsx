"use client";

import type { ComponentType } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ArrowRightLeft,
  Building2,
  CircleDollarSign,
  CreditCard,
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
  { href: "/workspaces/new", label: "Workspace Setup", scope: "platform", icon: Layers3 },
];

const tenantItems: NavItem[] = [
  { href: "/pricing", label: "Pricing", scope: "tenant", icon: CircleDollarSign },
  { href: "/customers", label: "Customers", scope: "tenant", icon: UserRoundPlus },
  { href: "/subscriptions", label: "Subscriptions", scope: "tenant", icon: ArrowRightLeft },
  { href: "/invoices", label: "Invoices", scope: "tenant", icon: ReceiptText },
  { href: "/workspace-access", label: "Access", scope: "tenant", icon: ShieldCheck },
  { href: "/payment-operations", label: "Payments", scope: "tenant", icon: CreditCard },
  { href: "/replay-operations", label: "Recovery", scope: "tenant", icon: Workflow },
  { href: "/invoice-explainability", label: "Explainability", scope: "tenant", icon: Layers3 },
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
  if (href === "/subscriptions") {
    return pathname === "/subscriptions" || pathname.startsWith("/subscriptions/");
  }
  if (href === "/invoices") {
    return pathname === "/invoices" || pathname.startsWith("/invoices/");
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
    <div className="flex flex-col gap-2">
      <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">{title}</p>
      <div className="flex flex-wrap gap-2">
        {items.map((item) => {
          const Icon = item.icon;
          const active = isActivePath(pathname, item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              prefetch={false}
              className={`inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium transition ${
                active
                  ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                  : "border-stone-200 bg-stone-50 text-slate-700 hover:border-stone-300 hover:bg-white"
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
      ? "Platform operator"
      : session?.tenant_id
        ? `Workspace ${session.tenant_id}`
        : "Workspace operator";

  const showPlatform = !isLoading && (!isAuthenticated || scope === "platform");
  const showTenant = !isLoading && (!isAuthenticated || scope === "tenant");

  return (
    <nav className="rounded-3xl border border-stone-200 bg-white/92 shadow-[0_18px_50px_rgba(15,23,42,0.06)] backdrop-blur">
      <div className="flex flex-col gap-4 p-4 md:p-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="flex min-w-0 items-start gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-emerald-700 text-lg font-semibold text-white">
              A
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-lg font-semibold tracking-tight text-slate-950">Alpha Control Plane</p>
                <span className="rounded-full border border-stone-200 bg-stone-50 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-600">
                  {contextLabel}
                </span>
              </div>
              <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">
                Product-owned billing administration on top of Lago. Platform configures reusable billing assets. Workspaces run pricing, customer, and subscription operations.
              </p>
              {isLoading ? (
                <p className="mt-2 text-xs uppercase tracking-[0.14em] text-slate-500">
                  Restoring browser session before resolving platform or workspace navigation.
                </p>
              ) : null}
            </div>
          </div>
          {isAuthenticated ? (
            <div className="flex items-center justify-end lg:pt-1">
              <SessionMenu />
            </div>
          ) : null}
        </div>

        {isLoading ? (
          <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-slate-600">
            Loading navigation context
          </div>
        ) : (
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            {showPlatform ? <NavSection title="Platform" items={platformItems} pathname={pathname} /> : null}
            {showTenant ? <NavSection title="Workspace" items={tenantItems} pathname={pathname} /> : null}
          </div>
        )}
      </div>
    </nav>
  );
}
