import { createFileRoute } from "@tanstack/react-router";
import { PricingAddOnDetailScreen } from "@/components/pricing/pricing-addon-detail-screen";

export const Route = createFileRoute("/pricing/add-ons/$id")({
  component: function PricingAddOnDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PricingAddOnDetailScreen addOnID={decodeURIComponent(id)} />;
  },
});
