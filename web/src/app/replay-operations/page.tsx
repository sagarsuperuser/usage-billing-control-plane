import { Suspense } from "react";

import { ReplayOperationsScreen } from "@/components/replay-ops/replay-operations-screen";

export default function ReplayOperationsPage() {
  return (
    <Suspense fallback={null}>
      <ReplayOperationsScreen />
    </Suspense>
  );
}
