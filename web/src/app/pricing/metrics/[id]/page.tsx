import { PricingMetricDetailScreen } from "@/components/pricing/pricing-metric-detail-screen";

export default async function PricingMetricDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PricingMetricDetailScreen metricID={id} />;
}
