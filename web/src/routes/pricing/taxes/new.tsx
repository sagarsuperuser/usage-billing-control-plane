import { createFileRoute } from "@tanstack/react-router";
import { PricingTaxNewScreen } from "@/components/pricing/pricing-tax-new-screen";

export const Route = createFileRoute("/pricing/taxes/new")({
  component: PricingTaxNewScreen,
});
