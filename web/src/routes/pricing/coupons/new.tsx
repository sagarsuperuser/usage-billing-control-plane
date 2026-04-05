import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/coupons/new")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-coupon-new-screen"), "PricingCouponNewScreen"),
});
