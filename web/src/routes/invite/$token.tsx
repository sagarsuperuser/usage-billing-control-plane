import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const WorkspaceInvitationScreen = lazy(() => import("@/components/auth/workspace-invitation-screen").then(m => ({ default: m.WorkspaceInvitationScreen })));

export const Route = createFileRoute("/invite/$token")({
  component: function InvitePageWrapper() {
    const { token } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <WorkspaceInvitationScreen token={decodeURIComponent(token)} />
      </Suspense>
    );
  },
});
