import { createFileRoute } from "@tanstack/react-router";
import { InvoiceDetailScreen } from "@/components/invoices/invoice-detail-screen";

export const Route = createFileRoute("/invoices/$id")({
  component: function InvoiceDetailPageWrapper() {
    const { id } = Route.useParams();
    return <InvoiceDetailScreen invoiceID={decodeURIComponent(id)} />;
  },
});
