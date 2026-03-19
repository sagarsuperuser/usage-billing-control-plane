import { Suspense } from "react";

import { InvoiceListScreen } from "@/components/invoices/invoice-list-screen";

export default function InvoicesPage() {
  return (
    <Suspense fallback={null}>
      <InvoiceListScreen />
    </Suspense>
  );
}
