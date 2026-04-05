import { useEffect, type ComponentType } from "react";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";
import {
  Activity,
  ArrowRightLeft,
  BellRing,
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
import { buildLoginPath } from "@/lib/session-routing";

// Pages that render without the sidebar (auth flow).
const AUTH_PATHS = ["/login", "/register", "/forgot-password", "/reset-password", "/invite", "/workspace-setup"];

type NavItem = {
  href: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
};

const revenueItems: NavItem[] = [
  { href: "/control-plane", label: "Overview", icon: Home },
  { href: "/pricing", label: "Pricing", icon: CircleDollarSign },
  { href: "/customers", label: "Customers", icon: UserRoundPlus },
  { href: "/subscriptions", label: "Subscriptions", icon: ArrowRightLeft },
  { href: "/invoices", label: "Invoices", icon: ReceiptText },
  { href: "/payments", label: "Payments", icon: CreditCard },
];

const operationsItems: NavItem[] = [
  { href: "/workspace-access", label: "Access", icon: ShieldCheck },
  { href: "/usage-events", label: "Usage", icon: Activity },
  { href: "/dunning", label: "Dunning", icon: BellRing },
  { href: "/replay-operations", label: "Replay", icon: Workflow },
  { href: "/invoice-explainability", label: "Explainability", icon: Layers3 },
];

function isActive(pathname: string, href: string): boolean {
  if (pathname === href) return true;
  if (href !== "/" && pathname.startsWith(href + "/")) return true;
  if (href === "/payments" && pathname === "/payment-operations") return true;
  return false;
}

export function AppSidebar() {
  const { pathname } = useLocation();
  const { isAuthenticated, isLoading } = useUISession();

  if (AUTH_PATHS.some((p) => pathname.startsWith(p))) return null;

  return (
    <aside className="fixed inset-y-0 left-0 z-30 flex w-[220px] flex-col border-r border-border bg-surface">
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-4 py-4">
        <Link to="/control-plane" className="flex items-center gap-2.5">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-blue-600 to-violet-600">
            <svg width="14" height="14" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="2" y="9" width="3" height="7" rx="1" fill="white" fillOpacity="0.5"/>
              <rect x="7" y="5" width="3" height="11" rx="1" fill="white" fillOpacity="0.75"/>
              <rect x="12" y="2" width="3" height="14" rx="1" fill="white"/>
            </svg>
          </div>
          <span className="text-sm font-semibold text-text-primary">Alpha</span>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto border-t border-border-light px-3 py-3">
        {isLoading ? (
          <div className="space-y-2 px-1">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="h-8 rounded-md bg-surface-secondary animate-pulse" />
            ))}
          </div>
        ) : (
          <div className="space-y-5">
            <NavGroup label="Revenue" items={revenueItems} pathname={pathname} />
            <NavGroup label="Operations" items={operationsItems} pathname={pathname} />
          </div>
        )}
      </nav>

      {/* Session — pinned to bottom (Stripe/Linear pattern) */}
      {isAuthenticated ? (
        <div className="border-t border-border px-3 py-3">
          <SessionMenu />
        </div>
      ) : null}
    </aside>
  );
}

function NavGroup({ label, items, pathname }: { label: string; items: NavItem[]; pathname: string }) {
  return (
    <div>
      <p className="mb-1 px-2 text-[11px] font-semibold uppercase tracking-wide text-text-faint">{label}</p>
      <div className="space-y-0.5">
        {items.map((item) => {
          const Icon = item.icon;
          const active = isActive(pathname, item.href);
          return (
            <Link
              key={item.href}
              to={item.href}
             
              className={`flex items-center gap-2.5 rounded-md px-2 py-1.5 text-[13px] font-medium transition-colors ${
                active
                  ? "bg-surface-tertiary text-text-primary"
                  : "text-text-muted hover:bg-surface-secondary hover:text-text-primary"
              }`}
            >
              <Icon className={`h-4 w-4 shrink-0 ${active ? "text-text-secondary" : "text-text-faint"}`} />
              {item.label}
            </Link>
          );
        })}
      </div>
    </div>
  );
}

/** Wrapper that adds left padding when the sidebar is visible. */
export function SidebarLayout({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation();
  const isAuthPage = AUTH_PATHS.some((p) => pathname.startsWith(p));

  if (isAuthPage) {
    return <>{children}</>;
  }

  return (
    <div className="min-h-screen bg-background">
      <AuthGuard />
      <AppSidebar />
      <div className="pl-[220px]">{children}</div>
    </div>
  );
}

/**
 * Centralized auth guard for all protected routes.
 * Redirects to /login if the session check completes and the user is not
 * authenticated. Replaces per-component LoginRedirectNotice (35 call sites).
 */
function AuthGuard() {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const { isLoading, isAuthenticated } = useUISession();

  useEffect(() => {
    if (isLoading || isAuthenticated) return;
    navigate({ to: buildLoginPath(pathname), replace: true });
  }, [isLoading, isAuthenticated, pathname, navigate]);

  return null;
}
