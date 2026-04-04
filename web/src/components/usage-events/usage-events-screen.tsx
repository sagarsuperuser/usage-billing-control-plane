"use client";

import { useState } from "react";
import { ChevronLeft, ChevronRight, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { fetchUsageEvents } from "@/lib/api";
import { formatExactTimestamp, formatRelativeTimestamp } from "@/lib/format";
import type { UsageEvent } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

const defaultLimit = 100;

type FilterState = {
  customerID: string;
  meterID: string;
  from: string;
  to: string;
  order: "asc" | "desc";
};

function toDateTimeLocal(value: Date): string {
  const pad = (input: number) => String(input).padStart(2, "0");
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}`;
}

function defaultFilters(): FilterState {
  const now = new Date();
  const from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
  return {
    customerID: "",
    meterID: "",
    from: toDateTimeLocal(from),
    to: toDateTimeLocal(now),
    order: "desc",
  };
}

function toISOOrUndefined(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const date = new Date(trimmed);
  if (Number.isNaN(date.valueOf())) return undefined;
  return date.toISOString();
}

export function UsageEventsScreen() {
  const { apiBaseURL, isAuthenticated, isLoading: sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [filters, setFilters] = useState<FilterState>(defaultFilters);
  const [submitted, setSubmitted] = useState<FilterState>(defaultFilters);
  const [cursor, setCursor] = useState("");
  const [cursorTrail, setCursorTrail] = useState<string[]>([]);
  const [selectedEventID, setSelectedEventID] = useState("");

  const query = useQuery({
    queryKey: ["usage-events", apiBaseURL, submitted, cursor],
    queryFn: () =>
      fetchUsageEvents({
        runtimeBaseURL: apiBaseURL,
        customerID: submitted.customerID || undefined,
        meterID: submitted.meterID || undefined,
        from: toISOOrUndefined(submitted.from),
        to: toISOOrUndefined(submitted.to),
        order: submitted.order,
        limit: defaultLimit,
        cursor: cursor || undefined,
      }),
    enabled: isTenantSession,
  });

  const items = query.data?.items ?? [];
  const selectedEvent = items.find((item) => item.id === selectedEventID) ?? null;

  const applyFilters = () => {
    setSubmitted(filters);
    setCursor("");
    setCursorTrail([]);
  };

  const openNextPage = () => {
    if (!query.data?.next_cursor) return;
    setCursorTrail((current) => [...current, cursor]);
    setCursor(query.data.next_cursor || "");
  };

  const openPreviousPage = () => {
    setCursorTrail((current) => {
      if (current.length === 0) {
        setCursor("");
        return current;
      }
      const next = current.slice(0, -1);
      setCursor(current[current.length - 1] || "");
      return next;
    });
  };

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ label: "Usage events" }]} />

        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            {/* Header with title + inline filters */}
            <div className="flex flex-wrap items-center gap-2 border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Usage events{items.length > 0 ? ` (${items.length})` : ""}</h1>
              <div className="ml-auto flex flex-wrap items-center gap-2">
                <input
                  value={filters.customerID}
                  onChange={(event) => setFilters((current) => ({ ...current, customerID: event.target.value }))}
                  aria-label="Customer ID"
                  placeholder="Customer ID..."
                  className="h-8 w-36 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <input
                  value={filters.meterID}
                  onChange={(event) => setFilters((current) => ({ ...current, meterID: event.target.value }))}
                  placeholder="Meter ID..."
                  className="h-8 w-36 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <input
                  type="datetime-local"
                  value={filters.from}
                  onChange={(event) => setFilters((current) => ({ ...current, from: event.target.value }))}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                />
                <input
                  type="datetime-local"
                  value={filters.to}
                  onChange={(event) => setFilters((current) => ({ ...current, to: event.target.value }))}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                />
                <select
                  value={filters.order}
                  onChange={(event) => setFilters((current) => ({ ...current, order: event.target.value as "asc" | "desc" }))}
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="desc">Newest first</option>
                  <option value="asc">Oldest first</option>
                </select>
                <button
                  type="button"
                  onClick={applyFilters}
                  className="inline-flex h-8 items-center rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800"
                >
                  Apply
                </button>
              </div>
            </div>

            {/* Table body */}
            {sessionLoading || query.isLoading ? (
              <LoadingState />
            ) : query.isError ? (
              <ErrorState message={query.error instanceof Error ? query.error.message : "Loading usage events failed."} />
            ) : items.length === 0 ? (
              <EmptyState />
            ) : (
              <>
                <div className="grid grid-cols-[1fr] xl:grid-cols-[minmax(0,1fr)_320px]">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                        <th className="px-5 py-2.5 font-semibold">ID</th>
                        <th className="px-4 py-2.5 font-semibold">Customer</th>
                        <th className="px-4 py-2.5 font-semibold">Meter</th>
                        <th className="px-4 py-2.5 font-semibold text-right">Qty</th>
                        <th className="px-4 py-2.5 font-semibold">Occurred</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-stone-100">
                      {items.map((item) => (
                        <tr
                          key={item.id}
                          onClick={() => setSelectedEventID(item.id)}
                          className={`cursor-pointer transition ${item.id === selectedEventID ? "bg-emerald-50" : "hover:bg-stone-50"}`}
                        >
                          <td className="px-5 py-3 font-mono text-xs text-slate-700 truncate max-w-[160px]">{item.id}</td>
                          <td className="px-4 py-3 text-slate-900 truncate max-w-[140px]">{item.customer_id}</td>
                          <td className="px-4 py-3 font-mono text-xs text-slate-600 truncate max-w-[140px]">{item.meter_id}</td>
                          <td className="px-4 py-3 text-right font-medium text-slate-900">{item.quantity}</td>
                          <td className="px-4 py-3 text-xs text-slate-500">{formatRelativeTimestamp(item.timestamp)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>

                  {/* Detail panel (side) */}
                  <div className="hidden border-l border-stone-200 xl:block">
                    <UsageEventDetail item={selectedEvent} />
                  </div>
                </div>

                {/* Detail panel (below on small screens) */}
                {selectedEvent ? (
                  <div className="border-t border-stone-200 xl:hidden">
                    <UsageEventDetail item={selectedEvent} />
                  </div>
                ) : null}
              </>
            )}

            {/* Pagination */}
            {!query.isLoading && items.length > 0 ? (
              <div className="flex items-center justify-between border-t border-stone-200 px-5 py-3">
                <button
                  type="button"
                  onClick={openPreviousPage}
                  disabled={cursorTrail.length === 0 || query.isLoading}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  Previous
                </button>
                <button
                  type="button"
                  onClick={openNextPage}
                  disabled={!query.data?.next_cursor || query.isLoading}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-sm text-slate-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Next
                  <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </div>
            ) : null}
          </div>
      </main>
    </div>
  );
}

function UsageEventDetail({ item }: { item: UsageEvent | null }) {
  if (!item) {
    return (
      <div className="px-5 py-8 text-center text-sm text-slate-500">
        Select an event to view details.
      </div>
    );
  }

  return (
    <div className="px-5 py-4">
      <h2 className="text-xs font-semibold text-slate-500">Event detail</h2>
      <div className="mt-3 grid gap-2">
        <DetailField label="Event ID" value={item.id} mono />
        <DetailField label="Customer" value={item.customer_id} mono />
        <DetailField label="Meter" value={item.meter_id} mono />
        <DetailField label="Subscription" value={item.subscription_id || "-"} mono />
        <DetailField label="Workspace ID" value={item.tenant_id || "-"} mono />
        <DetailField label="Quantity" value={String(item.quantity)} />
        <DetailField label="Occurred at" value={formatExactTimestamp(item.timestamp)} />
        <DetailField label="Idempotency key" value={item.idempotency_key || "-"} mono />
      </div>
    </div>
  );
}

function DetailField({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-stone-100 bg-stone-50 px-3 py-2">
      <p className="text-[11px] font-medium text-slate-400">{label}</p>
      <p className={`mt-0.5 break-all text-sm text-slate-900 ${mono ? "font-mono text-xs" : ""}`.trim()}>{value}</p>
    </div>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 px-5 py-8 text-sm text-slate-500">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading usage events...
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-5 py-16 text-center">
      <p className="text-sm font-medium text-slate-700">No usage events matched</p>
      <p className="text-xs text-slate-500">Try adjusting the customer, meter, or time range filters.</p>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="px-5 py-6">
      <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
        <p className="font-medium text-rose-900">Usage events could not be loaded</p>
        <p className="mt-1">{message}</p>
      </div>
    </div>
  );
}
