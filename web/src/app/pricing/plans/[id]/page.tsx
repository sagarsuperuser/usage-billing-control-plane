import { PricingPlanDetailScreen } from "@/components/pricing/pricing-plan-detail-screen";

export default async function PricingPlanDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PricingPlanDetailScreen planID={id} />;
}
