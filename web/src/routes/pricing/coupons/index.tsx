import { createFileRoute } from "@tanstack/react-router";
import { PricingCouponListScreen } from "@/components/pricing/pricing-coupon-list-screen";

export const Route = createFileRoute("/pricing/coupons/")({
  component: PricingCouponListScreen,
});
