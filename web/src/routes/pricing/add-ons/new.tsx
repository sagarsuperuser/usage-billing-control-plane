import { createFileRoute } from "@tanstack/react-router";
import { PricingAddOnNewScreen } from "@/components/pricing/pricing-addon-new-screen";

export const Route = createFileRoute("/pricing/add-ons/new")({
  component: PricingAddOnNewScreen,
});
