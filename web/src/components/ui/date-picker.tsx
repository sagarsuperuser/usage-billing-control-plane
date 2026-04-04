"use client";

import { useEffect, useRef, useState } from "react";
import { Calendar, ChevronLeft, ChevronRight } from "lucide-react";
import {
  format,
  startOfMonth,
  endOfMonth,
  eachDayOfInterval,
  addMonths,
  subMonths,
  isSameDay,
  isSameMonth,
  getDay,
  startOfWeek,
} from "date-fns";

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
  const [viewMonth, setViewMonth] = useState(() =>
    value ? startOfMonth(new Date(value)) : startOfMonth(new Date())
  );
  const containerRef = useRef<HTMLDivElement>(null);

  const selectedDate = value ? new Date(value) : null;

  useEffect(() => {
    const handler = (e: PointerEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", handler);
    document.addEventListener("keydown", keyHandler);
    return () => {
      document.removeEventListener("pointerdown", handler);
      document.removeEventListener("keydown", keyHandler);
    };
  }, []);

  const monthStart = startOfMonth(viewMonth);
  const monthEnd = endOfMonth(viewMonth);
  const days = eachDayOfInterval({ start: monthStart, end: monthEnd });
  const weekStartDay = getDay(startOfWeek(monthStart, { weekStartsOn: 0 }));
  const leadingBlanks = (getDay(monthStart) - weekStartDay + 7) % 7;

  const selectDay = (day: Date) => {
    onChange(format(day, "yyyy-MM-dd"));
    setOpen(false);
  };

  const displayValue = selectedDate ? format(selectedDate, "MMM d, yyyy") : "";

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        aria-label={ariaLabel}
        onClick={() => {
          if (!open && selectedDate) setViewMonth(startOfMonth(selectedDate));
          setOpen(!open);
        }}
        className="inline-flex h-7 items-center gap-1.5 rounded border border-stone-200 bg-white px-2 text-xs text-slate-700 outline-none ring-slate-400 transition hover:bg-stone-50 focus:ring-1"
      >
        <Calendar className="h-3 w-3 text-slate-400" />
        {displayValue || <span className="text-slate-400">{placeholder}</span>}
      </button>

      {open && (
        <div className="absolute left-0 top-full z-40 mt-1 w-[256px] rounded-lg border border-stone-200 bg-white p-3 shadow-lg">
          {/* Month nav */}
          <div className="flex items-center justify-between">
            <button type="button" onClick={() => setViewMonth(subMonths(viewMonth, 1))} className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 hover:bg-stone-100 hover:text-slate-600">
              <ChevronLeft className="h-3.5 w-3.5" />
            </button>
            <span className="text-xs font-semibold text-slate-800">{format(viewMonth, "MMMM yyyy")}</span>
            <button type="button" onClick={() => setViewMonth(addMonths(viewMonth, 1))} className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 hover:bg-stone-100 hover:text-slate-600">
              <ChevronRight className="h-3.5 w-3.5" />
            </button>
          </div>

          {/* Day headers */}
          <div className="mt-2 grid grid-cols-7 text-center text-[10px] font-medium text-slate-400">
            {["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"].map((d) => (
              <span key={d} className="py-1">{d}</span>
            ))}
          </div>

          {/* Day grid */}
          <div className="grid grid-cols-7">
            {Array.from({ length: leadingBlanks }).map((_, i) => (
              <span key={`blank-${i}`} />
            ))}
            {days.map((day) => {
              const isSelected = selectedDate && isSameDay(day, selectedDate);
              const isCurrentMonth = isSameMonth(day, viewMonth);
              const isToday = isSameDay(day, new Date());
              return (
                <button
                  key={day.toISOString()}
                  type="button"
                  onClick={() => selectDay(day)}
                  className={`h-8 w-full rounded text-xs transition ${
                    isSelected
                      ? "bg-slate-900 font-semibold text-white"
                      : isToday
                        ? "font-semibold text-slate-900 hover:bg-stone-100"
                        : isCurrentMonth
                          ? "text-slate-700 hover:bg-stone-100"
                          : "text-slate-300"
                  }`}
                >
                  {format(day, "d")}
                </button>
              );
            })}
          </div>

          {/* Clear */}
          {selectedDate && (
            <div className="mt-2 border-t border-stone-100 pt-2">
              <button
                type="button"
                onClick={() => { onChange(""); setOpen(false); }}
                className="text-[11px] text-slate-400 hover:text-slate-600"
              >
                Clear date
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
