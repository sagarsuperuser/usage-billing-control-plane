"use client";

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
            "rounded-2xl border border-stone-200 bg-white shadow-[0_8px_30px_rgba(15,23,42,0.10)] text-sm text-slate-900",
          title: "font-semibold text-slate-950",
          description: "text-slate-600",
          actionButton: "rounded-xl bg-slate-900 text-white text-xs font-semibold",
          cancelButton: "rounded-xl bg-stone-100 text-slate-700 text-xs font-semibold",
          closeButton: "rounded-lg border border-stone-200 bg-white text-slate-600 hover:bg-stone-50",
        },
      }}
      {...props}
    />
  );
}
