
import { useState } from "react";
import {
  ServerCog,
  ShieldCheck,
  UserRound,
} from "lucide-react";

import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { useUISession } from "@/hooks/use-ui-session";

import { WorkspaceMembersTab } from "./workspace-members-tab";
import { WorkspaceServiceAccountsTab } from "./workspace-service-accounts-tab";
import { WorkspaceAuditTab } from "./workspace-audit-tab";

export function TenantWorkspaceAccessScreen() {
  const { apiBaseURL, csrfToken, isAuthenticated, scope, isAdmin, session } = useUISession();
  const [activeTab, setActiveTab] = useState<"members" | "service-accounts" | "audit">("members");

  const tabProps = {
    apiBaseURL,
    csrfToken,
    isAdmin,
    session: session ? { tenant_id: session.tenant_id, subject_id: session.subject_id } : null,
  };

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-8 lg:px-10">
        <AppBreadcrumbs items={[{ label: "Access" }]} />

        {isAuthenticated && scope === "tenant" && !isAdmin ? (
          <ScopeNotice
            title="Workspace admin role required"
            body={`Only workspace admins can manage invitations, service accounts, roles, and member removal.`}
            actionHref="/customers"
            actionLabel="Open workspace home"
          />
        ) : null}

        {isAuthenticated && scope === "tenant" && isAdmin ? (
          <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            <div className="flex border-b border-border" role="tablist">
              {(
                [
                  { id: "members", label: "Members", Icon: UserRound },
                  { id: "service-accounts", label: "Service accounts", Icon: ServerCog },
                  { id: "audit", label: "Audit log", Icon: ShieldCheck },
                ] as const
              ).map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`flex items-center gap-2 border-b-2 px-5 py-3.5 text-sm font-medium transition ${
                    activeTab === tab.id
                      ? "border-slate-900 text-text-primary"
                      : "border-transparent text-text-muted hover:border-border hover:text-text-secondary"
                  }`}
                >
                  <tab.Icon className="h-3.5 w-3.5" />
                  {tab.label}
                </button>
              ))}
            </div>

            {activeTab === "members" && <WorkspaceMembersTab {...tabProps} />}
            {activeTab === "service-accounts" && <WorkspaceServiceAccountsTab {...tabProps} />}
            {activeTab === "audit" && <WorkspaceAuditTab {...tabProps} />}
          </div>
        ) : null}
      </main>
    </div>
  );
}
