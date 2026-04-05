import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/usage-events")({
  component: lazyRouteComponent(() => import("@/components/usage-events/usage-events-screen"), "UsageEventsScreen"),
});
