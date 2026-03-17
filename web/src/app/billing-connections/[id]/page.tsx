import { BillingConnectionDetailScreen } from "@/components/billing-connections/billing-connection-detail-screen";

export default async function BillingConnectionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <BillingConnectionDetailScreen connectionID={decodeURIComponent(id)} />;
}
