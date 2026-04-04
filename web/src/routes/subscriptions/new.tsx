import { createFileRoute } from "@tanstack/react-router";
import { SubscriptionNewScreen } from "@/components/subscriptions/subscription-new-screen";

export const Route = createFileRoute("/subscriptions/new")({
  component: SubscriptionNewScreen,
});
