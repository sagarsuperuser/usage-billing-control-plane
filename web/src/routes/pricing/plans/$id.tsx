import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PricingPlanDetailScreen = lazy(() => import("@/components/pricing/pricing-plan-detail-screen").then(m => ({ default: m.PricingPlanDetailScreen })));

export const Route = createFileRoute("/pricing/plans/$id")({
  component: function PricingPlanDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PricingPlanDetailScreen planID={id} />
      </Suspense>
    );
  },
});
