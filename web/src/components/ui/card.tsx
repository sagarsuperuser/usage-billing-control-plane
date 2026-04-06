
import type { ReactNode } from "react";

// ---------------------------------------------------------------------------
// Card — shared container with border, background, and optional shadow
//
// Usage:
//   <Card>Content</Card>
//   <Card className="divide-y divide-border">
//     <div className="p-5">Section 1</div>
//     <div className="p-5">Section 2</div>
//   </Card>
// ---------------------------------------------------------------------------

export function Card({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`overflow-hidden rounded-lg border border-border bg-surface shadow-sm ${className}`}>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// CardHeader — title bar at top of a card
// ---------------------------------------------------------------------------

export function CardHeader({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children?: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between border-b border-border px-5 py-4">
      <div>
        <h2 className="text-base font-semibold text-text-primary">{title}</h2>
        {description ? <p className="mt-0.5 text-xs text-text-muted">{description}</p> : null}
      </div>
      {children}
    </div>
  );
}
