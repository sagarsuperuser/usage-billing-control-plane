
import { forwardRef, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes } from "react";

// ---------------------------------------------------------------------------
// Input, Select, Textarea — shared form primitives
//
// Usage:
//   <Input placeholder="Search..." />
//   <Input size="sm" error />
//   <Select size="md"><option>Draft</option></Select>
//   <Textarea rows={2} placeholder="Notes..." />
// ---------------------------------------------------------------------------

type InputSize = "sm" | "md" | "lg";

const sizeClasses: Record<InputSize, string> = {
  sm: "h-8 text-xs",
  md: "h-9 text-sm",
  lg: "h-10 text-sm",
};

const baseClasses =
  "w-full rounded-lg border bg-surface px-3 text-text-primary outline-none transition placeholder:text-text-faint focus:ring-2";

const normalBorder = "border-border ring-slate-400";
const errorBorder = "border-rose-300 ring-rose-200";

// ── Input ─────────────────────────────────────────────────────────────

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  inputSize?: InputSize;
  error?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ inputSize = "lg", error, className = "", ...props }, ref) => (
    <input
      ref={ref}
      className={`${baseClasses} ${sizeClasses[inputSize]} ${error ? errorBorder : normalBorder} ${className}`}
      {...props}
    />
  ),
);
Input.displayName = "Input";

// ── Select ────────────────────────────────────────────────────────────

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  inputSize?: InputSize;
  error?: boolean;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ inputSize = "lg", error, className = "", children, ...props }, ref) => (
    <select
      ref={ref}
      className={`${baseClasses} ${sizeClasses[inputSize]} ${error ? errorBorder : normalBorder} ${className}`}
      {...props}
    >
      {children}
    </select>
  ),
);
Select.displayName = "Select";

// ── Textarea ──────────────────────────────────────────────────────────

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ error, className = "", ...props }, ref) => (
    <textarea
      ref={ref}
      className={`${baseClasses} py-2.5 text-sm ${error ? errorBorder : normalBorder} ${className}`}
      {...props}
    />
  ),
);
Textarea.displayName = "Textarea";
