import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/new")({
  component: lazyRouteComponent(() => import("@/components/subscriptions/subscription-new-screen"), "SubscriptionNewScreen"),
});
