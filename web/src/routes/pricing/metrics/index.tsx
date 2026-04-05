import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/metrics/")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-metric-list-screen"), "PricingMetricListScreen"),
});
