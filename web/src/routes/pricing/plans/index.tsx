import { createFileRoute } from "@tanstack/react-router";
import { PricingPlanListScreen } from "@/components/pricing/pricing-plan-list-screen";

export const Route = createFileRoute("/pricing/plans/")({
  component: PricingPlanListScreen,
});
