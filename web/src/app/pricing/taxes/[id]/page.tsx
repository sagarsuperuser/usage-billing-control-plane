import { PricingTaxDetailScreen } from "@/components/pricing/pricing-tax-detail-screen";

export default async function PricingTaxDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PricingTaxDetailScreen taxID={decodeURIComponent(id)} />;
}
