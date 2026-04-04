import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  beforeLoad: () => {
    // The root page checks auth and redirects accordingly.
    // With TanStack Router, this is handled by the auth guard in __root.
    // Default redirect to login; authenticated users are redirected by the root layout.
    throw redirect({ to: "/login" });
  },
});
