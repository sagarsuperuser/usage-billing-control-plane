import { WorkspaceInvitationScreen } from "@/components/auth/workspace-invitation-screen";

export default async function WorkspaceInvitationPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return <WorkspaceInvitationScreen token={token} />;
}
