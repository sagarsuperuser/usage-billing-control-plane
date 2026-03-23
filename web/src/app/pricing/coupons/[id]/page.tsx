import { PricingCouponDetailScreen } from "@/components/pricing/pricing-coupon-detail-screen";

export default async function PricingCouponDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PricingCouponDetailScreen couponID={decodeURIComponent(id)} />;
}
