import { createFileRoute } from "@tanstack/react-router";
import { PricingPlanNewScreen } from "@/components/pricing/pricing-plan-new-screen";

export const Route = createFileRoute("/pricing/plans/new")({
  component: PricingPlanNewScreen,
});
