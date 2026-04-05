import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/payments/")({
  component: lazyRouteComponent(() => import("@/components/payments/payment-list-screen"), "PaymentListScreen"),
});
