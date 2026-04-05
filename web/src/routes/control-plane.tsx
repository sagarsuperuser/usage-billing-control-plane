import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/control-plane")({
  component: lazyRouteComponent(() => import("@/components/overview/control-plane-overview-screen"), "ControlPlaneOverviewScreen"),
});
