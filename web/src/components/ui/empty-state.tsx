import { type ReactNode } from "react";
import { Link } from "@tanstack/react-router";

type EmptyStateProps = {
  icon?: ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  actionHref?: string;
};

export function EmptyState({ icon, title, description, actionLabel, actionHref }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 px-5 py-14 text-center">
      {icon || <EmptyIllustration />}
      <p className="text-sm font-medium text-text-secondary">{title}</p>
      {description ? <p className="max-w-xs text-xs text-text-muted">{description}</p> : null}
      {actionLabel && actionHref ? (
        <Link to={actionHref} className="mt-1 inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700">
          {actionLabel}
        </Link>
      ) : null}
    </div>
  );
}

function EmptyIllustration() {
  return (
    <svg width="64" height="64" viewBox="0 0 64 64" fill="none" xmlns="http://www.w3.org/2000/svg" className="text-slate-200">
      {/* Folder/empty box illustration */}
      <rect x="12" y="20" width="40" height="28" rx="3" stroke="currentColor" strokeWidth="1.5" fill="none" />
      <path d="M12 27h40" stroke="currentColor" strokeWidth="1.5" />
      <path d="M12 23c0-1.7 1.3-3 3-3h10l3 4h21c1.7 0 3 1.3 3 3" stroke="currentColor" strokeWidth="1.5" fill="none" />
      <circle cx="32" cy="37" r="5" stroke="currentColor" strokeWidth="1.5" fill="none" />
      <path d="M30 37h4M32 35v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
