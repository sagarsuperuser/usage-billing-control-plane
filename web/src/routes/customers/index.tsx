import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/customers/")({
  component: lazyRouteComponent(() => import("@/components/customers/customer-list-screen"), "CustomerListScreen"),
});
