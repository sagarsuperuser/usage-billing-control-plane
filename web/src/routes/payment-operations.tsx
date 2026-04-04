import { createFileRoute } from "@tanstack/react-router";
import { PaymentOperationsScreen } from "@/components/payment-ops/payment-operations-screen";

export const Route = createFileRoute("/payment-operations")({
  component: PaymentOperationsScreen,
});
