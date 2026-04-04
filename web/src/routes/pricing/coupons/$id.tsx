import { createFileRoute } from "@tanstack/react-router";
import { PricingCouponDetailScreen } from "@/components/pricing/pricing-coupon-detail-screen";

export const Route = createFileRoute("/pricing/coupons/$id")({
  component: function PricingCouponDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PricingCouponDetailScreen couponID={decodeURIComponent(id)} />;
  },
});
