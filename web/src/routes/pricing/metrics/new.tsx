import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/metrics/new")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-metric-new-screen"), "PricingMetricNewScreen"),
});
