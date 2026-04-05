import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PricingAddOnDetailScreen = lazy(() => import("@/components/pricing/pricing-addon-detail-screen").then(m => ({ default: m.PricingAddOnDetailScreen })));

export const Route = createFileRoute("/pricing/add-ons/$id")({
  component: function PricingAddOnDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PricingAddOnDetailScreen addOnID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
