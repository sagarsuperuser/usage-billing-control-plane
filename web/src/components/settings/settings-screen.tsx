
import { Building2, CreditCard, Users } from "lucide-react";
import { useNavigate } from "@tanstack/react-router";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { useUISession } from "@/hooks/use-ui-session";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";

import { SettingsGeneralTab } from "./settings-general-tab";
import { SettingsBillingTab } from "./settings-billing-tab";
import { SettingsTeamTab } from "./settings-team-tab";

const tabs = [
  { id: "general", label: "General", Icon: Building2 },
  { id: "billing", label: "Billing", Icon: CreditCard },
  { id: "team", label: "Team", Icon: Users },
] as const;

type TabID = (typeof tabs)[number]["id"];

const validTabs = new Set<string>(tabs.map((t) => t.id));

function resolveTab(raw: string | null): TabID {
  return raw && validTabs.has(raw) ? (raw as TabID) : "general";
}

export function SettingsScreen() {
  const navigate = useNavigate();
  const searchParams = useSearchParamsCompat();
  const activeTab = resolveTab(searchParams.get("tab"));
  const { apiBaseURL, csrfToken, isAuthenticated, scope, isAdmin, session } = useUISession();

  const isTenantSession = isAuthenticated && scope === "tenant";

  const setTab = (id: TabID) => {
    navigate({ to: "/settings", search: { tab: id === "general" ? undefined : id }, replace: true });
  };

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-8 lg:px-10">
        <AppBreadcrumbs items={[{ label: "Settings" }]} />

        {/* Page header */}
        <div>
          <h1 className="text-lg font-semibold text-text-primary">Settings</h1>
          <p className="mt-0.5 text-sm text-text-muted">Manage your workspace, billing configuration, and team access.</p>
        </div>

        {isTenantSession && !isAdmin ? (
          <ScopeNotice
            title="Workspace admin role required"
            body="Only workspace admins can manage settings, billing, and team access."
            actionHref="/control-plane"
            actionLabel="Back to overview"
          />
        ) : null}

        {isTenantSession && isAdmin ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex border-b border-border" role="tablist">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === tab.id}
                  onClick={() => setTab(tab.id)}
                  className={`flex items-center gap-2 border-b-[3px] px-5 py-3.5 text-sm font-semibold transition ${
                    activeTab === tab.id
                      ? "border-blue-600 text-text-primary dark:border-blue-400"
                      : "border-transparent text-text-muted hover:text-text-secondary"
                  }`}
                >
                  <tab.Icon className="h-3.5 w-3.5" />
                  {tab.label}
                </button>
              ))}
            </div>

            {activeTab === "general" && (
              <SettingsGeneralTab apiBaseURL={apiBaseURL} csrfToken={csrfToken} session={session} />
            )}
            {activeTab === "billing" && (
              <SettingsBillingTab apiBaseURL={apiBaseURL} csrfToken={csrfToken} />
            )}
            {activeTab === "team" && (
              <SettingsTeamTab
                apiBaseURL={apiBaseURL}
                csrfToken={csrfToken}
                isAdmin={isAdmin}
                session={session ? { tenant_id: session.tenant_id, subject_id: session.subject_id } : null}
              />
            )}
          </div>
        ) : null}
      </main>
    </div>
  );
}
