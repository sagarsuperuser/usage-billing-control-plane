
import { useEffect, useRef, useState } from "react";
import { Calendar, X } from "lucide-react";
import { DayPicker } from "react-day-picker";
import { format, parse, isValid, setHours, setMinutes } from "date-fns";

/* ------------------------------------------------------------------ */
/*  Shared styles for react-day-picker (Stripe/Vercel pattern)        */
/* ------------------------------------------------------------------ */

const calendarLabels = {
  labelPrevious: () => "Go to previous month",
  labelNext: () => "Go to next month",
};

const calendarClassNames = {
  root: "text-text-primary",
  months: "relative",
  month_caption: "flex items-center justify-center py-1",
  caption_label: "text-xs font-semibold text-text-secondary",
  nav: "absolute inset-x-1 top-0 flex items-center justify-between",
  button_previous: "inline-flex h-7 w-7 items-center justify-center rounded-md text-text-faint transition hover:bg-surface-tertiary hover:text-text-muted",
  button_next: "inline-flex h-7 w-7 items-center justify-center rounded-md text-text-faint transition hover:bg-surface-tertiary hover:text-text-muted",
  weekdays: "flex",
  weekday: "w-8 py-1 text-center text-[10px] font-medium text-text-faint",
  week: "flex",
  day: "flex h-8 w-8 items-center justify-center text-xs",
  day_button: "h-7 w-7 rounded-md transition hover:bg-surface-tertiary",
  selected: "bg-slate-900 text-white rounded-md hover:bg-slate-800",
  today: "font-bold text-text-primary",
  outside: "text-slate-300",
  disabled: "text-slate-200",
};

/* ------------------------------------------------------------------ */
/*  DatePicker — date only (audit tab, filters)                       */
/* ------------------------------------------------------------------ */

export function DatePicker({
  value,
  onChange,
  placeholder = "Pick date",
  "aria-label": ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  "aria-label"?: string;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const selectedDate = value ? parse(value, "yyyy-MM-dd", new Date()) : undefined;
  const displayValue = selectedDate && isValid(selectedDate) ? format(selectedDate, "MMM d, yyyy") : "";

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false);
    };
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", handleClick);
    document.addEventListener("keydown", handleKey);
    return () => { document.removeEventListener("pointerdown", handleClick); document.removeEventListener("keydown", handleKey); };
  }, [open]);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={ariaLabel}
        onClick={() => setOpen(!open)}
        className="inline-flex h-7 items-center gap-1.5 rounded border border-border bg-surface px-2 text-xs text-text-secondary outline-none ring-slate-400 transition hover:bg-surface-secondary focus:ring-1"
      >
        <Calendar className="h-3 w-3 text-text-faint" />
        {displayValue || <span className="text-text-faint">{placeholder}</span>}
        {displayValue && (
          <span
            role="button"
            tabIndex={-1}
            onClick={(e) => { e.stopPropagation(); onChange(""); }}
            className="ml-0.5 text-slate-300 hover:text-text-muted"
          >
            <X className="h-3 w-3" />
          </span>
        )}
      </button>

      {open && (
        <div className="absolute left-0 top-full z-40 mt-1 rounded-xl border border-border bg-surface p-3 shadow-xl">
          <DayPicker
            mode="single"
            selected={selectedDate}
            onSelect={(day) => {
              if (day) {
                onChange(format(day, "yyyy-MM-dd"));
                setOpen(false);
              }
            }}
            defaultMonth={selectedDate}
            classNames={calendarClassNames}
            labels={calendarLabels}
          />
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  DateTimePicker — date + time (usage events, replay, coupons)      */
/* ------------------------------------------------------------------ */

export function DateTimePicker({
  value,
  onChange,
  placeholder = "Pick date & time",
  "aria-label": ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  "aria-label"?: string;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // value format: "2026-04-05T14:30" (datetime-local compatible)
  const parsed = value ? new Date(value) : null;
  const selectedDate = parsed && isValid(parsed) ? parsed : undefined;
  const displayValue = selectedDate ? format(selectedDate, "MMM d, yyyy HH:mm") : "";

  const hours = selectedDate ? selectedDate.getHours() : 0;
  const minutes = selectedDate ? selectedDate.getMinutes() : 0;

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false);
    };
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", handleClick);
    document.addEventListener("keydown", handleKey);
    return () => { document.removeEventListener("pointerdown", handleClick); document.removeEventListener("keydown", handleKey); };
  }, [open]);

  const emitValue = (date: Date) => {
    const pad = (n: number) => String(n).padStart(2, "0");
    onChange(`${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`);
  };

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={ariaLabel}
        onClick={() => setOpen(!open)}
        className="inline-flex h-7 items-center gap-1.5 rounded border border-border bg-surface px-2 text-xs text-text-secondary outline-none ring-slate-400 transition hover:bg-surface-secondary focus:ring-1"
      >
        <Calendar className="h-3 w-3 text-text-faint" />
        {displayValue || <span className="text-text-faint">{placeholder}</span>}
        {displayValue && (
          <span
            role="button"
            tabIndex={-1}
            onClick={(e) => { e.stopPropagation(); onChange(""); }}
            className="ml-0.5 text-slate-300 hover:text-text-muted"
          >
            <X className="h-3 w-3" />
          </span>
        )}
      </button>

      {open && (
        <div className="absolute left-0 top-full z-40 mt-1 rounded-xl border border-border bg-surface p-3 shadow-xl">
          <DayPicker
            mode="single"
            selected={selectedDate}
            onSelect={(day) => {
              if (day) {
                const withTime = setMinutes(setHours(day, hours), minutes);
                emitValue(withTime);
              }
            }}
            defaultMonth={selectedDate}
            classNames={calendarClassNames}
            labels={calendarLabels}
          />

          {/* Time selector */}
          <div className="mt-2 flex items-center gap-2 border-t border-border-light pt-2">
            <span className="text-[11px] font-medium text-text-faint">Time</span>
            <div className="flex items-center gap-1">
              <input
                type="number"
                min={0}
                max={23}
                value={String(hours).padStart(2, "0")}
                onChange={(e) => {
                  const h = Math.min(23, Math.max(0, Number(e.target.value) || 0));
                  const base = selectedDate || new Date();
                  emitValue(setMinutes(setHours(base, h), minutes));
                }}
                className="h-7 w-10 rounded border border-border text-center text-xs text-text-secondary outline-none focus:ring-1 focus:ring-slate-400"
              />
              <span className="text-xs text-text-faint">:</span>
              <input
                type="number"
                min={0}
                max={59}
                value={String(minutes).padStart(2, "0")}
                onChange={(e) => {
                  const m = Math.min(59, Math.max(0, Number(e.target.value) || 0));
                  const base = selectedDate || new Date();
                  emitValue(setMinutes(setHours(base, hours), m));
                }}
                className="h-7 w-10 rounded border border-border text-center text-xs text-text-secondary outline-none focus:ring-1 focus:ring-slate-400"
              />
            </div>
            {selectedDate && (
              <button
                type="button"
                onClick={() => { onChange(""); setOpen(false); }}
                className="ml-auto text-[11px] text-text-faint hover:text-text-muted"
              >
                Clear
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  DateTimeInput — fallback styled native input (for react-hook-form)*/
/* ------------------------------------------------------------------ */

export function DateTimeInput({
  value,
  onChange,
  "aria-label": ariaLabel,
  className = "",
}: {
  value: string;
  onChange: (value: string) => void;
  "aria-label"?: string;
  className?: string;
}) {
  return (
    <input
      type="datetime-local"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-label={ariaLabel}
      className={`h-7 rounded border border-border bg-surface px-2 text-xs text-text-secondary outline-none ring-slate-400 transition focus:ring-1 ${className}`}
    />
  );
}
