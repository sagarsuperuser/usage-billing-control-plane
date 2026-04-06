
import type { ReactNode } from "react";

// ---------------------------------------------------------------------------
// PageContainer — standard page wrapper with max-width + padding
//
// Usage:
//   <PageContainer>
//     <AppBreadcrumbs items={[...]} />
//     {/* page content */}
//   </PageContainer>
// ---------------------------------------------------------------------------

export function PageContainer({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className="text-text-primary">
      <main className={`mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-8 lg:px-10 ${className}`}>
        {children}
      </main>
    </div>
  );
}
