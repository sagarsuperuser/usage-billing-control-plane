import { createFileRoute } from "@tanstack/react-router";
import { PricingMetricNewScreen } from "@/components/pricing/pricing-metric-new-screen";

export const Route = createFileRoute("/pricing/metrics/new")({
  component: PricingMetricNewScreen,
});
