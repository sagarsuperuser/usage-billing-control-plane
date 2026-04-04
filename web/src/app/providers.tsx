import { ReactNode } from "react";

import { PageErrorBoundary } from "@/components/ui/error-boundary";
import { Toaster } from "@/components/ui/sonner";
import { QueryProvider } from "@/providers/query-provider";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <PageErrorBoundary>
      <QueryProvider>
        {children}
        <Toaster />
      </QueryProvider>
    </PageErrorBoundary>
  );
}
