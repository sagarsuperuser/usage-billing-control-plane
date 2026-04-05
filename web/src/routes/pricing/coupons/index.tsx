import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/coupons/")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-coupon-list-screen"), "PricingCouponListScreen"),
});
