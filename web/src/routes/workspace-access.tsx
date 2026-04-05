import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace-access")({
  component: lazyRouteComponent(() => import("@/components/workspaces/tenant-workspace-access-screen"), "TenantWorkspaceAccessScreen"),
});
