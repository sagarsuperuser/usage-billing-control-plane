import { CustomerDetailScreen } from "@/components/customers/customer-detail-screen";

export default async function CustomerDetailPage({ params }: { params: Promise<{ externalId: string }> }) {
  const { externalId } = await params;
  return <CustomerDetailScreen externalID={decodeURIComponent(externalId)} />;
}
