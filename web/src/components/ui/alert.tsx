
import type { ReactNode } from "react";

// ---------------------------------------------------------------------------
// Alert — inline banner for success/error/warning/info messages
//
// Usage:
//   <Alert tone="danger">Connection failed.</Alert>
//   <Alert tone="success">Settings saved.</Alert>
//   <Alert tone="warning">You have unsaved changes.</Alert>
//   <Alert tone="info">Password reset email sent.</Alert>
// ---------------------------------------------------------------------------

type AlertTone = "danger" | "success" | "warning" | "info";

const toneClasses: Record<AlertTone, string> = {
  danger:  "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300",
  success: "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300",
  warning: "border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
  info:    "border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950 dark:text-blue-300",
};

export function Alert({
  tone = "info",
  children,
  className = "",
}: {
  tone?: AlertTone;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border px-4 py-3 text-sm ${toneClasses[tone]} ${className}`}>
      {children}
    </div>
  );
}
