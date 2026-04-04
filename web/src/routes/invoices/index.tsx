import { createFileRoute } from "@tanstack/react-router";
import { InvoiceListScreen } from "@/components/invoices/invoice-list-screen";

export const Route = createFileRoute("/invoices/")({
  component: InvoiceListScreen,
});
