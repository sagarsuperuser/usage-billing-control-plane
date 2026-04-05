import { type ReactNode } from "react";

const toneClasses = {
  success: "border-emerald-200 bg-emerald-50 text-emerald-700",
  warning: "border-amber-200 bg-amber-50 text-amber-700",
  danger: "border-rose-200 bg-rose-50 text-rose-700",
  info: "border-sky-200 bg-sky-50 text-sky-700",
  neutral: "border-border bg-surface-tertiary text-text-muted",
} as const;

export type StatusChipTone = keyof typeof toneClasses;

export function StatusChip({ tone, children }: { tone: StatusChipTone; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold leading-tight ${toneClasses[tone]}`}>
      {children}
    </span>
  );
}
