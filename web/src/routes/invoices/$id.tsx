import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const InvoiceDetailScreen = lazy(() => import("@/components/invoices/invoice-detail-screen").then(m => ({ default: m.InvoiceDetailScreen })));

export const Route = createFileRoute("/invoices/$id")({
  component: function InvoiceDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <InvoiceDetailScreen invoiceID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
