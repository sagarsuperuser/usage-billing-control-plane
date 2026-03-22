import { Suspense } from "react";

import { DunningConsoleScreen } from "@/components/dunning/dunning-console-screen";

export default function DunningPage() {
  return (
    <Suspense fallback={null}>
      <DunningConsoleScreen />
    </Suspense>
  );
}
