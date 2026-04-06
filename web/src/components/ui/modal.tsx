
import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";
import { Button } from "./button";

// ---------------------------------------------------------------------------
// Modal — centered dialog with backdrop
//
// Usage:
//   <Modal open={showInvite} onClose={() => setShowInvite(false)} title="Invite member">
//     <form>...</form>
//   </Modal>
// ---------------------------------------------------------------------------

export function Modal({
  open,
  onClose,
  title,
  description,
  size = "md",
  children,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  description?: string;
  size?: "sm" | "md" | "lg";
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  if (!open) return null;

  const sizeClasses = {
    sm: "max-w-sm",
    md: "max-w-md",
    lg: "max-w-lg",
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className={`w-full ${sizeClasses[size]} rounded-xl bg-surface shadow-2xl ring-1 ring-black/10`}>
        {title ? (
          <div className="flex items-center justify-between border-b border-border px-6 py-4">
            <div>
              <p className="font-semibold text-text-primary">{title}</p>
              {description ? <p className="mt-0.5 text-xs text-text-muted">{description}</p> : null}
            </div>
            <Button variant="ghost" size="xs" onClick={onClose} className="!px-1">
              <X className="h-4 w-4" />
            </Button>
          </div>
        ) : null}
        {children}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Drawer — right-side slide panel
//
// Usage:
//   <Drawer open={!!selectedID} onClose={() => setSelectedID("")} title="Member">
//     <div>content</div>
//   </Drawer>
// ---------------------------------------------------------------------------

export function Drawer({
  open,
  onClose,
  title,
  width = "md",
  children,
  headerRight,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  width?: "sm" | "md" | "lg";
  children: ReactNode;
  headerRight?: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, onClose]);

  if (!open) return null;

  const widthClasses = {
    sm: "max-w-sm",
    md: "max-w-[480px]",
    lg: "max-w-xl",
  };

  return (
    <div
      className="fixed inset-0 z-40 flex justify-end"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className={`h-full w-full ${widthClasses[width]} border-l border-border bg-surface shadow-xl overflow-y-auto`}>
        {title ? (
          <div className="flex items-center justify-between border-b border-border px-5 py-3.5">
            <p className="font-semibold text-text-primary">{title}</p>
            <div className="flex items-center gap-2">
              {headerRight}
              <Button variant="ghost" size="xs" onClick={onClose} className="!px-1">
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          </div>
        ) : null}
        {children}
      </div>
    </div>
  );
}
