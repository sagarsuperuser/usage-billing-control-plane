import { createFileRoute } from "@tanstack/react-router";
import { SubscriptionListScreen } from "@/components/subscriptions/subscription-list-screen";

export const Route = createFileRoute("/subscriptions/")({
  component: SubscriptionListScreen,
});
