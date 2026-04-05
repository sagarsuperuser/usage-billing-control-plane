import { createFileRoute, lazyRouteComponent } from "@tanstack/react-router";

export const Route = createFileRoute("/invoices/")({
  component: lazyRouteComponent(() => import("@/components/invoices/invoice-list-screen"), "InvoiceListScreen"),
});
