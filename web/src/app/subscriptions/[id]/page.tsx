import { SubscriptionDetailScreen } from "@/components/subscriptions/subscription-detail-screen";

export default async function SubscriptionDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <SubscriptionDetailScreen subscriptionID={decodeURIComponent(id)} />;
}
