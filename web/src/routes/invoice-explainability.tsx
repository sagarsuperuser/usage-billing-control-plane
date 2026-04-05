import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/invoice-explainability")({
  component: lazyRouteComponent(() => import("@/components/invoice-explainability/invoice-explainability-screen"), "InvoiceExplainabilityScreen"),
});
