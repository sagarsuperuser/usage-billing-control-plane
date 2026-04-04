import { createFileRoute } from "@tanstack/react-router";
import { ControlPlaneOverviewScreen } from "@/components/overview/control-plane-overview-screen";

export const Route = createFileRoute("/control-plane")({
  component: ControlPlaneOverviewScreen,
});
