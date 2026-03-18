import { Suspense } from "react";

import { WorkspaceSelectionScreen } from "@/components/auth/workspace-selection-screen";

export default function WorkspaceSelectionPage() {
  return (
    <Suspense fallback={null}>
      <WorkspaceSelectionScreen />
    </Suspense>
  );
}
