import { createFileRoute } from "@tanstack/react-router";
import { PricingCouponNewScreen } from "@/components/pricing/pricing-coupon-new-screen";

export const Route = createFileRoute("/pricing/coupons/new")({
  component: PricingCouponNewScreen,
});
