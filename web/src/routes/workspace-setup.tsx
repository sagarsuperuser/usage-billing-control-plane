import { createFileRoute } from "@tanstack/react-router";
import { WorkspaceSetupScreen } from "@/components/workspaces/workspace-setup-screen";

export const Route = createFileRoute("/workspace-setup")({
  component: WorkspaceSetupScreen,
});
