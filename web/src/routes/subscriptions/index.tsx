import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/subscriptions/")({
  component: lazyRouteComponent(() => import("@/components/subscriptions/subscription-list-screen"), "SubscriptionListScreen"),
});
