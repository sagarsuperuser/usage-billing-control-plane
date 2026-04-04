import { createFileRoute } from "@tanstack/react-router";
import { PricingTaxListScreen } from "@/components/pricing/pricing-tax-list-screen";

export const Route = createFileRoute("/pricing/taxes/")({
  component: PricingTaxListScreen,
});
