import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/taxes/new")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-tax-new-screen"), "PricingTaxNewScreen"),
});
