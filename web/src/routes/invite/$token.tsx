import { createFileRoute } from "@tanstack/react-router";
import { WorkspaceInvitationScreen } from "@/components/auth/workspace-invitation-screen";

export const Route = createFileRoute("/invite/$token")({
  component: function InvitePageWrapper() {
    const { token } = Route.useParams();
    return <WorkspaceInvitationScreen token={decodeURIComponent(token)} />;
  },
});
