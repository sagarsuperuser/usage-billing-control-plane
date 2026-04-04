import { createFileRoute } from "@tanstack/react-router";
import { PricingAddOnListScreen } from "@/components/pricing/pricing-addon-list-screen";

export const Route = createFileRoute("/pricing/add-ons/")({
  component: PricingAddOnListScreen,
});
