import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/payment-operations")({
  component: lazyRouteComponent(() => import("@/components/payment-ops/payment-operations-screen"), "PaymentOperationsScreen"),
});
