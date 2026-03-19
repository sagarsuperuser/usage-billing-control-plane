import { PaymentDetailScreen } from "@/components/payments/payment-detail-screen";

export default async function PaymentDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <PaymentDetailScreen paymentID={decodeURIComponent(id)} />;
}
