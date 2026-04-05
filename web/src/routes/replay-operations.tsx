import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/replay-operations")({
  component: lazyRouteComponent(() => import("@/components/replay-ops/replay-operations-screen"), "ReplayOperationsScreen"),
});
