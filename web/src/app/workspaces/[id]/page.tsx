import { WorkspaceDetailScreen } from "@/components/workspaces/workspace-detail-screen";

export default async function WorkspaceDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <WorkspaceDetailScreen tenantID={decodeURIComponent(id)} />;
}
