import { Suspense } from "react";

import { PaymentOperationsScreen } from "@/components/payment-ops/payment-operations-screen";

export default function PaymentOperationsPage() {
  return (
    <Suspense fallback={null}>
      <PaymentOperationsScreen />
    </Suspense>
  );
}
