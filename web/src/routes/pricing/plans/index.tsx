import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/plans/")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-plan-list-screen"), "PricingPlanListScreen"),
});
