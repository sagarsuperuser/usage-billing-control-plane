import { createFileRoute } from "@tanstack/react-router";
import { PaymentDetailScreen } from "@/components/payments/payment-detail-screen";

export const Route = createFileRoute("/payments/$id")({
  component: function PaymentDetailPageWrapper() {
    const { id } = Route.useParams();
    return <PaymentDetailScreen paymentID={decodeURIComponent(id)} />;
  },
});
