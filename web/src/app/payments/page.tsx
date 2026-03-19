import { Suspense } from "react";

import { PaymentListScreen } from "@/components/payments/payment-list-screen";

export default function PaymentsPage() {
  return (
    <Suspense fallback={null}>
      <PaymentListScreen />
    </Suspense>
  );
}
