import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/plans/new")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-plan-new-screen"), "PricingPlanNewScreen"),
});
