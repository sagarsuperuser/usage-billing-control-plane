
import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";

// ---------------------------------------------------------------------------
// Button — shared primitive for all interactive buttons
//
// Usage:
//   <Button variant="primary">Connect</Button>
//   <Button variant="secondary" size="sm">Cancel</Button>
//   <Button variant="ghost">Edit</Button>
//   <Button variant="danger">Remove</Button>
//   <Button loading>Saving...</Button>
// ---------------------------------------------------------------------------

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
type ButtonSize = "xs" | "sm" | "md" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  children: ReactNode;
}

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    "bg-blue-600 text-white shadow-sm hover:bg-blue-700 disabled:bg-blue-600/50",
  secondary:
    "border border-border bg-surface text-text-secondary shadow-sm hover:bg-surface-secondary disabled:opacity-50",
  ghost:
    "text-text-muted hover:text-text-secondary hover:bg-surface-secondary disabled:opacity-50",
  danger:
    "bg-rose-600 text-white shadow-sm hover:bg-rose-700 disabled:bg-rose-600/50",
};

const sizeClasses: Record<ButtonSize, string> = {
  xs: "h-6 px-2 text-[11px] gap-1 rounded-md",
  sm: "h-7 px-2.5 text-xs gap-1.5 rounded-md",
  md: "h-8 px-3.5 text-sm gap-1.5 rounded-lg",
  lg: "h-10 px-5 text-sm gap-2 rounded-lg",
};

const spinnerSizes: Record<ButtonSize, string> = {
  xs: "h-3 w-3",
  sm: "h-3 w-3",
  md: "h-3.5 w-3.5",
  lg: "h-4 w-4",
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", size = "md", loading, disabled, children, className = "", ...props }, ref) => {
    const isDisabled = disabled || loading;

    return (
      <button
        ref={ref}
        disabled={isDisabled}
        className={`inline-flex items-center justify-center font-semibold transition disabled:cursor-not-allowed ${variantClasses[variant]} ${sizeClasses[size]} ${className}`}
        {...props}
      >
        {loading ? <LoaderCircle className={`animate-spin ${spinnerSizes[size]}`} /> : null}
        {children}
      </button>
    );
  },
);

Button.displayName = "Button";
