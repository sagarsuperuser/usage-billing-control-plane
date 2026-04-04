import { ChevronLeft, ChevronRight } from "lucide-react";

export function MiniPagination({
  page,
  totalPages,
  onPageChange,
  label,
}: {
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  label: string;
}) {
  if (totalPages <= 1) return null;

  return (
    <div className="inline-flex items-center gap-1">
      <button
        type="button"
        onClick={() => onPageChange(page - 1)}
        disabled={page <= 1}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 transition hover:bg-stone-100 hover:text-slate-600 disabled:opacity-40"
      >
        <ChevronLeft className="h-3.5 w-3.5" />
      </button>
      <span className="min-w-[56px] text-center text-[11px] text-slate-400">
        {page} / {totalPages}
      </span>
      <button
        type="button"
        onClick={() => onPageChange(page + 1)}
        disabled={page >= totalPages}
        aria-label={`Next ${label} page`}
        className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 transition hover:bg-stone-100 hover:text-slate-600 disabled:opacity-40"
      >
        <ChevronRight className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

export function CursorPagination({
  page,
  hasPrevious,
  hasNext,
  onPrevious,
  onNext,
  label,
}: {
  page: number;
  hasPrevious: boolean;
  hasNext: boolean;
  onPrevious: () => void;
  onNext: () => void;
  label: string;
}) {
  if (!hasPrevious && !hasNext) return null;

  return (
    <div className="inline-flex items-center gap-1">
      <button
        type="button"
        onClick={onPrevious}
        disabled={!hasPrevious}
        aria-label={`Previous ${label} page`}
        className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 transition hover:bg-stone-100 hover:text-slate-600 disabled:opacity-40"
      >
        <ChevronLeft className="h-3.5 w-3.5" />
      </button>
      <span className="min-w-[40px] text-center text-[11px] text-slate-400">
        Page {page}
      </span>
      <button
        type="button"
        onClick={onNext}
        disabled={!hasNext}
        aria-label={`Next ${label} page`}
        className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 transition hover:bg-stone-100 hover:text-slate-600 disabled:opacity-40"
      >
        <ChevronRight className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}
