import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PricingTaxDetailScreen = lazy(() => import("@/components/pricing/pricing-tax-detail-screen").then(m => ({ default: m.PricingTaxDetailScreen })));

export const Route = createFileRoute("/pricing/taxes/$id")({
  component: function PricingTaxDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PricingTaxDetailScreen taxID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
