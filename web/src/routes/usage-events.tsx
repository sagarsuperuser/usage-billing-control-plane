import { createFileRoute } from "@tanstack/react-router";
import { UsageEventsScreen } from "@/components/usage-events/usage-events-screen";

export const Route = createFileRoute("/usage-events")({
  component: UsageEventsScreen,
});
