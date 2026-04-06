
// ---------------------------------------------------------------------------
// LoadingSkeleton — animated placeholder for loading states
//
// Usage:
//   <LoadingSkeleton />                    // 3 lines default
//   <LoadingSkeleton lines={5} />          // Custom line count
//   <LoadingSkeleton variant="card" />     // Card with header + body
//   <LoadingSkeleton variant="table" />    // Table rows
// ---------------------------------------------------------------------------

type SkeletonVariant = "lines" | "card" | "table";

export function LoadingSkeleton({
  variant = "lines",
  lines = 3,
}: {
  variant?: SkeletonVariant;
  lines?: number;
}) {
  if (variant === "card") {
    return (
      <div className="rounded-lg border border-border bg-surface p-5 shadow-sm">
        <div className="animate-pulse space-y-3">
          <div className="h-6 w-48 rounded bg-surface-secondary" />
          <div className="h-4 w-72 rounded bg-surface-secondary" />
          <div className="h-32 w-full rounded bg-surface-secondary" />
        </div>
      </div>
    );
  }

  if (variant === "table") {
    return (
      <div className="animate-pulse space-y-2 px-5 py-4">
        {Array.from({ length: lines }).map((_, i) => (
          <div key={i} className="flex gap-4">
            <div className="h-4 flex-1 rounded bg-surface-secondary" />
            <div className="h-4 w-20 rounded bg-surface-secondary" />
            <div className="h-4 w-16 rounded bg-surface-secondary" />
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="animate-pulse space-y-2">
      {Array.from({ length: lines }).map((_, i) => (
        <div key={i} className={`h-4 rounded bg-surface-secondary ${i === 0 ? "w-3/4" : i === lines - 1 ? "w-1/2" : "w-full"}`} />
      ))}
    </div>
  );
}
