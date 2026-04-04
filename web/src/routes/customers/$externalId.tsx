import { createFileRoute } from "@tanstack/react-router";
import { CustomerDetailScreen } from "@/components/customers/customer-detail-screen";

export const Route = createFileRoute("/customers/$externalId")({
  component: function CustomerDetailPageWrapper() {
    const { externalId } = Route.useParams();
    return <CustomerDetailScreen externalID={decodeURIComponent(externalId)} />;
  },
});
