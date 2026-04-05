import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const CustomerDetailScreen = lazy(() => import("@/components/customers/customer-detail-screen").then(m => ({ default: m.CustomerDetailScreen })));

export const Route = createFileRoute("/customers/$externalId")({
  component: function CustomerDetailPageWrapper() {
    const { externalId } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <CustomerDetailScreen externalID={decodeURIComponent(externalId)} />
      </Suspense>
    );
  },
});
