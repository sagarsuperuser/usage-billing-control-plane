
import { useState } from "react";
import { ServerCog, ShieldCheck, UserRound } from "lucide-react";

import { WorkspaceMembersTab } from "@/components/workspaces/workspace-members-tab";
import { WorkspaceServiceAccountsTab } from "@/components/workspaces/workspace-service-accounts-tab";
import { WorkspaceAuditTab } from "@/components/workspaces/workspace-audit-tab";

const subtabs = [
  { id: "members", label: "Members", Icon: UserRound },
  { id: "service-accounts", label: "Service accounts", Icon: ServerCog },
  { id: "audit", label: "Audit log", Icon: ShieldCheck },
] as const;

type SubtabID = (typeof subtabs)[number]["id"];

export function SettingsTeamTab({
  apiBaseURL,
  csrfToken,
  isAdmin,
  session,
}: {
  apiBaseURL: string;
  csrfToken: string;
  isAdmin: boolean;
  session: { tenant_id?: string; subject_id?: string } | null;
}) {
  const [active, setActive] = useState<SubtabID>("members");

  return (
    <div>
      <div className="flex gap-1 border-b border-border bg-surface-secondary/50 px-6 pt-2" role="tablist">
        {subtabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={active === tab.id}
            onClick={() => setActive(tab.id)}
            className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-[13px] font-medium transition ${
              active === tab.id
                ? "border-blue-600 text-text-primary dark:border-blue-400"
                : "border-transparent text-text-muted hover:text-text-secondary"
            }`}
          >
            <tab.Icon className="h-3.5 w-3.5" />
            {tab.label}
          </button>
        ))}
      </div>

      {active === "members" && (
        <WorkspaceMembersTab apiBaseURL={apiBaseURL} csrfToken={csrfToken} isAdmin={isAdmin} session={session} />
      )}
      {active === "service-accounts" && (
        <WorkspaceServiceAccountsTab apiBaseURL={apiBaseURL} csrfToken={csrfToken} isAdmin={isAdmin} session={session} />
      )}
      {active === "audit" && (
        <WorkspaceAuditTab apiBaseURL={apiBaseURL} csrfToken={csrfToken} isAdmin={isAdmin} session={session} />
      )}
    </div>
  );
}
