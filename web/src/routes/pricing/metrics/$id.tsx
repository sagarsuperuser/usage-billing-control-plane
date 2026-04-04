import { createFileRoute } from "@tanstack/react-router";
import { PricingMetricDetailScreen } from "@/components/pricing/pricing-metric-detail-screen";

export const Route = createFileRoute("/pricing/metrics/$id")({
  component: function PricingMetricDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PricingMetricDetailScreen metricID={id} />;
  },
});
