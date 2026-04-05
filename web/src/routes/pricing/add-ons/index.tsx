import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/pricing/add-ons/")({
  component: lazyRouteComponent(() => import("@/components/pricing/pricing-addon-list-screen"), "PricingAddOnListScreen"),
});
