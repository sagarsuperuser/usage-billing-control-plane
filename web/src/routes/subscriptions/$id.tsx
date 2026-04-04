import { createFileRoute } from "@tanstack/react-router";
import { SubscriptionDetailScreen } from "@/components/subscriptions/subscription-detail-screen";

export const Route = createFileRoute("/subscriptions/$id")({
  component: function SubscriptionDetailPageWrapper() {
    const { id } = Route.useParams();
    return <SubscriptionDetailScreen subscriptionID={decodeURIComponent(id)} />;
  },
});
