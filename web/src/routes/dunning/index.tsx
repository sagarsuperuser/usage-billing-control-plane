import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/dunning/")({
  component: lazyRouteComponent(() => import("@/components/dunning/dunning-console-screen"), "DunningConsoleScreen"),
});
