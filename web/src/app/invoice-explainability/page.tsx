import { Suspense } from "react";

import { InvoiceExplainabilityScreen } from "@/components/invoice-explainability/invoice-explainability-screen";

export default function InvoiceExplainabilityPage() {
  return (
    <Suspense fallback={null}>
      <InvoiceExplainabilityScreen />
    </Suspense>
  );
}
