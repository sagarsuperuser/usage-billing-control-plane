import { createFileRoute } from "@tanstack/react-router";
import { PricingTaxDetailScreen } from "@/components/pricing/pricing-tax-detail-screen";

export const Route = createFileRoute("/pricing/taxes/$id")({
  component: function PricingTaxDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PricingTaxDetailScreen taxID={decodeURIComponent(id)} />;
  },
});
