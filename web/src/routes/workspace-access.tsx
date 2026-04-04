import { createFileRoute } from "@tanstack/react-router";
import { TenantWorkspaceAccessScreen } from "@/components/workspaces/tenant-workspace-access-screen";

export const Route = createFileRoute("/workspace-access")({
  component: TenantWorkspaceAccessScreen,
});
