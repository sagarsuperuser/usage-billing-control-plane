"use client";

import { type ReactNode, useState } from "react";
import { AlertTriangle } from "lucide-react";

type ConfirmDialogProps = {
  title: string;
  description: string;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: "danger" | "warning";
  children: (open: () => void) => ReactNode;
  onConfirm: () => void | Promise<void>;
  disabled?: boolean;
};

export function ConfirmDialog({
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  tone = "danger",
  children,
  onConfirm,
  disabled,
}: ConfirmDialogProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isPending, setIsPending] = useState(false);

  const handleConfirm = async () => {
    setIsPending(true);
    try {
      await onConfirm();
      setIsOpen(false);
    } finally {
      setIsPending(false);
    }
  };

  const confirmClass =
    tone === "danger"
      ? "border-rose-700 bg-rose-700 text-white hover:bg-rose-800"
      : "border-amber-600 bg-amber-600 text-white hover:bg-amber-700";

  return (
    <>
      {children(() => setIsOpen(true))}
      {isOpen ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={(e) => { if (e.target === e.currentTarget && !isPending) setIsOpen(false); }}
        >
          <div className="w-full max-w-sm rounded-lg bg-white shadow-xl ring-1 ring-black/10">
            <div className="p-5">
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-rose-50">
                  <AlertTriangle className="h-4 w-4 text-rose-600" />
                </div>
                <div>
                  <p className="text-sm font-semibold text-slate-900">{title}</p>
                  <p className="mt-1 text-xs text-slate-500">{description}</p>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-2 border-t border-stone-100 px-5 py-3">
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                disabled={isPending}
                className="inline-flex h-8 items-center rounded-md border border-stone-200 px-3 text-xs font-medium text-slate-700 transition hover:bg-stone-50 disabled:opacity-50"
              >
                {cancelLabel}
              </button>
              <button
                type="button"
                onClick={handleConfirm}
                disabled={disabled || isPending}
                className={`inline-flex h-8 items-center rounded-md border px-3 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${confirmClass}`}
              >
                {isPending ? "..." : confirmLabel}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
