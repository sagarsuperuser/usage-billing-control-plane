import { createFileRoute } from "@tanstack/react-router";
import { PricingPlanDetailScreen } from "@/components/pricing/pricing-plan-detail-screen";

export const Route = createFileRoute("/pricing/plans/$id")({
  component: function PricingPlanDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PricingPlanDetailScreen planID={id} />;
  },
});
