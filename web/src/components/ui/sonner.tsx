
import { Toaster as Sonner, type ToasterProps } from "sonner";

export function Toaster({ ...props }: ToasterProps) {
  return (
    <Sonner
      theme="light"
      richColors
      position="bottom-right"
      toastOptions={{
        classNames: {
          toast:
            "rounded-2xl border border-border bg-surface shadow-[0_8px_30px_rgba(15,23,42,0.10)] text-sm text-text-primary",
          title: "font-semibold text-text-primary",
          description: "text-text-muted",
          actionButton: "rounded-xl bg-slate-900 text-white text-xs font-semibold",
          cancelButton: "rounded-xl bg-surface-tertiary text-text-secondary text-xs font-semibold",
          closeButton: "rounded-lg border border-border bg-surface text-text-muted hover:bg-surface-secondary",
        },
      }}
      {...props}
    />
  );
}
