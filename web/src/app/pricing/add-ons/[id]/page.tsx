import { PricingAddOnDetailScreen } from "@/components/pricing/pricing-addon-detail-screen";

export default async function PricingAddOnDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PricingAddOnDetailScreen addOnID={decodeURIComponent(id)} />;
}
