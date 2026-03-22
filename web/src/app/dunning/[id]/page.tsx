import { DunningRunDetailScreen } from "@/components/dunning/dunning-run-detail-screen";

export default async function DunningRunDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <DunningRunDetailScreen runID={decodeURIComponent(id)} />;
}
