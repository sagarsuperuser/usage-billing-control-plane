import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace-access")({
  beforeLoad: () => {
    throw redirect({ to: "/settings", search: { tab: "team" } });
  },
});
