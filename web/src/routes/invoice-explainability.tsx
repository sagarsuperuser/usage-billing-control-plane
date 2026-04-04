import { createFileRoute } from "@tanstack/react-router";
import { InvoiceExplainabilityScreen } from "@/components/invoice-explainability/invoice-explainability-screen";

export const Route = createFileRoute("/invoice-explainability")({
  component: InvoiceExplainabilityScreen,
});
