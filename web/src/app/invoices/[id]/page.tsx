import { InvoiceDetailScreen } from "@/components/invoices/invoice-detail-screen";

export default async function InvoiceDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <InvoiceDetailScreen invoiceID={decodeURIComponent(id)} />;
}
