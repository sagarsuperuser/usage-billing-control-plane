
import type { ReactNode } from "react";

// ---------------------------------------------------------------------------
// FormField — label + children + hint + error wrapper
//
// Usage:
//   <FormField label="Name" error={errors.name?.message}>
//     <Input {...register("name")} />
//   </FormField>
//
//   <FormField label="Memo" hint="Custom invoice text.">
//     <Textarea {...register("memo")} />
//   </FormField>
// ---------------------------------------------------------------------------

export function FormField({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <label className="grid gap-1.5">
      <span className="text-xs font-medium text-text-muted">
        {label}
        {hint ? <span className="ml-1.5 font-normal text-text-faint">{hint}</span> : null}
      </span>
      {children}
      {error ? <span className="text-xs text-rose-600">{error}</span> : null}
    </label>
  );
}

// ---------------------------------------------------------------------------
// ReadOnlyField — label + value (non-editable display)
// ---------------------------------------------------------------------------

export function ReadOnlyField({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="grid gap-1">
      <span className="text-xs font-medium text-text-muted">{label}</span>
      <p className={`text-sm text-text-secondary ${mono ? "font-mono" : ""}`}>{value}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// SectionHeader — section title + optional description
// ---------------------------------------------------------------------------

export function SectionHeader({
  title,
  description,
}: {
  title: string;
  description?: string;
}) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-text-primary">{title}</h3>
      {description ? <p className="mt-0.5 text-xs text-text-muted">{description}</p> : null}
    </div>
  );
}
