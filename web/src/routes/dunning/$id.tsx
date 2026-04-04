import { createFileRoute } from "@tanstack/react-router";
import { DunningRunDetailScreen } from "@/components/dunning/dunning-run-detail-screen";

export const Route = createFileRoute("/dunning/$id")({
  component: function DunningRunDetailPageWrapper() {
    const { id } = Route.useParams();
    return <DunningRunDetailScreen runID={decodeURIComponent(id)} />;
  },
});
