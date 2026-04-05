import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PricingMetricDetailScreen = lazy(() => import("@/components/pricing/pricing-metric-detail-screen").then(m => ({ default: m.PricingMetricDetailScreen })));

export const Route = createFileRoute("/pricing/metrics/$id")({
  component: function PricingMetricDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PricingMetricDetailScreen metricID={id} />
      </Suspense>
    );
  },
});
