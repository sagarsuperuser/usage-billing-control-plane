import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/add-ons/new")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-addon-new-screen"), "PricingAddOnNewScreen"),
});
