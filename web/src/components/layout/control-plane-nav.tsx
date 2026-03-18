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
  description: string;
  scope: NavScope;
  icon: ComponentType<{ className?: string }>;
};

const items: NavItem[] = [
  {
    href: "/control-plane",
    label: "Home",
    description: "Operating view and next actions.",
    scope: "platform",
    icon: Home,
  },
  {
    href: "/billing-connections",
    label: "Billing Connections",
    description: "Provider credentials, sync state, and reuse.",
    scope: "platform",
    icon: CreditCard,
  },
  {
    href: "/workspaces",
    label: "Workspaces",
    description: "Workspace health, billing, and handoff.",
    scope: "platform",
    icon: Building2,
  },
  {
    href: "/workspaces/new",
    label: "Workspace Setup",
    description: "Create and activate a new workspace.",
    scope: "platform",
    icon: Layers3,
  },
  {
    href: "/pricing",
    label: "Pricing",
    description: "Metrics and plans used inside the workspace.",
    scope: "tenant",
    icon: CircleDollarSign,
  },
  {
    href: "/customers",
    label: "Customers",
    description: "Billable customer records and readiness.",
    scope: "tenant",
    icon: UserRoundPlus,
  },
  {
    href: "/subscriptions",
    label: "Subscriptions",
    description: "Activation, payment setup, and status.",
    scope: "tenant",
    icon: ArrowRightLeft,
  },
  {
    href: "/workspace-access",
    label: "Access",
    description: "Members, invitations, and roles.",
    scope: "tenant",
    icon: ShieldCheck,
  },
  {
    href: "/payment-operations",
    label: "Payments",
    description: "Operational payment issues and retries.",
    scope: "tenant",
    icon: ReceiptText,
  },
  {
    href: "/replay-operations",
    label: "Recovery",
    description: "Replay and recover failed processing.",
    scope: "tenant",
    icon: Workflow,
  },
  {
    href: "/invoice-explainability",
    label: "Explainability",
    description: "Trace invoice construction and outcomes.",
    scope: "tenant",
    icon: Layers3,
  },
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
  if (href === "/pricing") {
    return pathname === "/pricing" || pathname.startsWith("/pricing/");
  }
  return pathname === href;
}

function shellItemsForScope(scope: NavScope | null, isAuthenticated: boolean): NavItem[] {
  if (!isAuthenticated || !scope) {
    return items;
  }
  return items.filter((item) => item.scope === scope || item.href === "/control-plane");
}

export function ControlPlaneNav() {
  const pathname = usePathname();
  const { isAuthenticated, session, scope } = useUISession();

  const activeScope = isAuthenticated ? scope : "platform";
  const visibleItems = shellItemsForScope(scope ?? null, isAuthenticated);
  const groupedItems = [
    {
      title: activeScope === "platform" ? "Platform command" : "Workspace command",
      eyebrow: activeScope === "platform" ? "Platform surface" : "Workspace surface",
      items: visibleItems,
    },
  ];

  const scopeLabel =
    scope === "platform"
      ? "Platform operator"
      : session?.tenant_id
        ? `Workspace ${session.tenant_id}`
        : "Workspace operator";

  return (
    <nav className="rounded-[28px] border border-emerald-900/10 bg-[#f7f3eb]/95 text-slate-900 shadow-[0_30px_80px_rgba(15,23,42,0.08)] backdrop-blur">
      <div className="grid gap-5 p-4 md:p-5 xl:grid-cols-[240px_minmax(0,1fr)_260px] xl:items-start">
        <div className="rounded-[24px] border border-emerald-900/10 bg-[linear-gradient(160deg,#0f3b2f_0%,#174e3d_100%)] p-5 text-emerald-50 shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/12 text-lg font-semibold">A</div>
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.22em] text-emerald-100/80">Alpha</p>
              <p className="mt-1 text-xl font-semibold tracking-tight">Control Plane</p>
            </div>
          </div>
          <p className="mt-4 text-sm leading-6 text-emerald-50/78">
            Product-owned billing operations on top of Lago. Platform sets the system up. Workspaces run it.
          </p>
        </div>

        <div className="grid gap-4">
          {groupedItems.map((group) => (
            <section key={group.title} className="rounded-[24px] border border-stone-200 bg-white/88 p-4 shadow-[0_10px_30px_rgba(15,23,42,0.05)]">
              <div className="mb-4 flex flex-col gap-1 md:flex-row md:items-end md:justify-between">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-emerald-700/70">{group.eyebrow}</p>
                  <h2 className="mt-1 text-lg font-semibold tracking-tight text-slate-900">{group.title}</h2>
                </div>
                <p className="text-xs text-slate-500">Use the surface that matches the ownership boundary.</p>
              </div>
              <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
                {group.items.map((item) => {
                  const active = isActivePath(pathname, item.href);
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`group rounded-[20px] border p-4 transition ${
                        active
                          ? "border-emerald-300 bg-emerald-50 shadow-[0_8px_20px_rgba(22,101,52,0.08)]"
                          : "border-stone-200 bg-stone-50/80 hover:border-emerald-200 hover:bg-white"
                      }`}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <span
                          className={`inline-flex h-11 w-11 items-center justify-center rounded-2xl border ${
                            active
                              ? "border-emerald-200 bg-white text-emerald-700"
                              : "border-stone-200 bg-white text-slate-600 group-hover:text-emerald-700"
                          }`}
                        >
                          <Icon className="h-5 w-5" />
                        </span>
                        {active ? (
                          <span className="rounded-full bg-emerald-700 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-white">
                            Open
                          </span>
                        ) : null}
                      </div>
                      <h3 className="mt-4 text-sm font-semibold text-slate-900">{item.label}</h3>
                      <p className="mt-1 text-sm leading-6 text-slate-600">{item.description}</p>
                    </Link>
                  );
                })}
              </div>
            </section>
          ))}
        </div>

        <div className="grid gap-4">
          <section className="rounded-[24px] border border-stone-200 bg-white/88 p-4 shadow-[0_10px_30px_rgba(15,23,42,0.05)]">
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Current context</p>
            <h2 className="mt-2 text-lg font-semibold tracking-tight text-slate-900">{scopeLabel}</h2>
            <p className="mt-2 text-sm leading-6 text-slate-600">
              {scope === "platform"
                ? "Use this surface to manage billing connections, workspace rollout, and delegated access."
                : "Use this surface to run pricing, customers, subscriptions, and workspace operations."}
            </p>
            {session?.role || session?.tenant_id ? (
              <div className="mt-4 flex flex-wrap gap-2">
                {session?.role ? (
                  <span className="rounded-full border border-stone-200 bg-stone-100 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700">
                    {session.role}
                  </span>
                ) : null}
                {session?.tenant_id ? (
                  <span className="rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">
                    {session.tenant_id}
                  </span>
                ) : null}
              </div>
            ) : null}
          </section>
          {isAuthenticated ? (
            <section className="rounded-[24px] border border-stone-200 bg-white/88 p-4 shadow-[0_10px_30px_rgba(15,23,42,0.05)]">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Session</p>
              <div className="mt-3 flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-slate-900">Authenticated</p>
                  <p className="mt-1 text-sm text-slate-600">Switch context or sign out from the active browser session.</p>
                </div>
                <SessionMenu />
              </div>
            </section>
          ) : null}
        </div>
      </div>
    </nav>
  );
}
