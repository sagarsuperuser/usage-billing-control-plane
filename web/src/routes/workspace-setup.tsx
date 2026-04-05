import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace-setup")({
  component: lazyRouteComponent(() => import("@/components/workspaces/workspace-setup-screen"), "WorkspaceSetupScreen"),
});
