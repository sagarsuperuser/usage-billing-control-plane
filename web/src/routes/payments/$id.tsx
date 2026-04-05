import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const PaymentDetailScreen = lazy(() => import("@/components/payments/payment-detail-screen").then(m => ({ default: m.PaymentDetailScreen })));

export const Route = createFileRoute("/payments/$id")({
  component: function PaymentDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <PaymentDetailScreen paymentID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
