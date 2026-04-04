import { createFileRoute } from "@tanstack/react-router";
import { ReplayOperationsScreen } from "@/components/replay-ops/replay-operations-screen";

export const Route = createFileRoute("/replay-operations")({
  component: ReplayOperationsScreen,
});
