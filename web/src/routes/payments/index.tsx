import { createFileRoute } from "@tanstack/react-router";
import { PaymentListScreen } from "@/components/payments/payment-list-screen";

export const Route = createFileRoute("/payments/")({
  component: PaymentListScreen,
});
