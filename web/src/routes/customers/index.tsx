import { createFileRoute } from "@tanstack/react-router";
import { CustomerListScreen } from "@/components/customers/customer-list-screen";

export const Route = createFileRoute("/customers/")({
  component: CustomerListScreen,
});
