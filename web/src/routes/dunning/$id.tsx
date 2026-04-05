import { createFileRoute } from "@tanstack/react-router";
import { lazy, Suspense } from "react";

const DunningRunDetailScreen = lazy(() => import("@/components/dunning/dunning-run-detail-screen").then(m => ({ default: m.DunningRunDetailScreen })));

export const Route = createFileRoute("/dunning/$id")({
  component: function DunningRunDetailPageWrapper() {
    const { id } = Route.useParams();
    return (
      <Suspense fallback={null}>
        <DunningRunDetailScreen runID={decodeURIComponent(id)} />
      </Suspense>
    );
  },
});
