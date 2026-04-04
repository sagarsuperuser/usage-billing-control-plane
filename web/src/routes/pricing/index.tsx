import { createFileRoute } from "@tanstack/react-router";
import { PricingHomeScreen } from "@/components/pricing/pricing-home-screen";

export const Route = createFileRoute("/pricing/")({
  component: PricingHomeScreen,
});
