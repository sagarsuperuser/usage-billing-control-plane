import { createFileRoute } from "@tanstack/react-router";
import { DunningConsoleScreen } from "@/components/dunning/dunning-console-screen";

export const Route = createFileRoute("/dunning/")({
  component: DunningConsoleScreen,
});
