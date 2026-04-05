import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PricingCouponDetailScreen = lazy(() => import("@/components/pricing/pricing-coupon-detail-screen").then(m => ({ default: m.PricingCouponDetailScreen })));

export const Route = createFileRoute("/pricing/coupons/$id")({
  component: function PricingCouponDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PricingCouponDetailScreen couponID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
