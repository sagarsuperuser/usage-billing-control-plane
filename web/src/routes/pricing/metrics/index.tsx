import { createFileRoute } from "@tanstack/react-router";
import { PricingMetricListScreen } from "@/components/pricing/pricing-metric-list-screen";

export const Route = createFileRoute("/pricing/metrics/")({
  component: PricingMetricListScreen,
});
