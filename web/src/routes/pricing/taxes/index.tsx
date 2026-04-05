import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/taxes/")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-tax-list-screen"), "PricingTaxListScreen"),
});
