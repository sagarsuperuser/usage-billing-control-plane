import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const SubscriptionDetailScreen = lazy(() => import("@/components/subscriptions/subscription-detail-screen").then(m => ({ default: m.SubscriptionDetailScreen })));

export const Route = createFileRoute("/subscriptions/$id")({
  component: function SubscriptionDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <SubscriptionDetailScreen subscriptionID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
