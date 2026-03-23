"use client";

import { useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
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
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [filters, setFilters] = useState<FilterState>(defaultFilters);
  const [submitted, setSubmitted] = useState<FilterState>(defaultFilters);
  const [cursor, setCursor] = useState("");
  const [cursorTrail, setCursorTrail] = useState<string[]>([]);

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
    enabled: isAuthenticated && scope === "tenant",
  });

  const items = query.data?.items || [];
  const stats = useMemo(() => ({
    visible: items.length,
    quantity: items.reduce((sum, item) => sum + item.quantity, 0),
    customers: new Set(items.map((item) => item.customer_id)).size,
    meters: new Set(items.map((item) => item.meter_id)).size,
  }), [items]);

  const applyFilters = () => {
    setSubmitted(filters);
    setCursor("");
    setCursorTrail([]);
  };

  const resetFilters = () => {
    const next = defaultFilters();
    setFilters(next);
    setSubmitted(next);
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
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/customers", label: "Tenant" }, { label: "Usage Events" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Usage Events</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Raw operational event view</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Inspect recent raw usage events for support, onboarding, and billing disputes. This page is intentionally bounded to filtered, paginated operational reads.
              </p>
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Usage events are tenant-scoped. Sign in with a tenant session to inspect event ingestion."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <MetricCard label="Visible events" value={String(stats.visible)} />
          <MetricCard label="Visible quantity" value={String(stats.quantity)} />
          <MetricCard label="Customers on page" value={String(stats.customers)} />
          <MetricCard label="Meters on page" value={String(stats.meters)} />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Filters</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Bound the query before you inspect events</h2>
            </div>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                onClick={applyFilters}
                className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800"
              >
                Apply filters
              </button>
              <button
                type="button"
                onClick={resetFilters}
                className="inline-flex h-10 items-center justify-center rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"
              >
                Reset to last 7 days
              </button>
            </div>
          </div>
          <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <InputField label="Customer ID" value={filters.customerID} onChange={(value) => setFilters((current) => ({ ...current, customerID: value }))} placeholder="cust_123" />
            <InputField label="Meter ID" value={filters.meterID} onChange={(value) => setFilters((current) => ({ ...current, meterID: value }))} placeholder="mtr_123" />
            <DateField label="From" value={filters.from} onChange={(value) => setFilters((current) => ({ ...current, from: value }))} />
            <DateField label="To" value={filters.to} onChange={(value) => setFilters((current) => ({ ...current, to: value }))} />
            <label className="grid gap-2 text-sm text-slate-700">
              <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Order</span>
              <select
                value={filters.order}
                onChange={(event) => setFilters((current) => ({ ...current, order: event.target.value as "asc" | "desc" }))}
                className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
              >
                <option value="desc">Newest first</option>
                <option value="asc">Oldest first</option>
              </select>
            </label>
          </div>
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Event stream</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Recent raw events</h2>
            </div>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                onClick={openPreviousPage}
                disabled={cursorTrail.length === 0 || query.isLoading}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4" />
                Previous
              </button>
              <button
                type="button"
                onClick={openNextPage}
                disabled={!query.data?.next_cursor || query.isLoading}
                className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Next
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          </div>

          <div className="mt-5 grid gap-3">
            {query.isLoading ? <LoadingState /> : null}
            {query.isError ? <ErrorState message={query.error instanceof Error ? query.error.message : "Loading usage events failed."} /> : null}
            {!query.isLoading && !query.isError && items.length === 0 ? <EmptyState /> : null}
            {!query.isLoading && !query.isError ? items.map((item) => <UsageEventRow key={item.id} item={item} />) : null}
          </div>
        </section>
      </main>
    </div>
  );
}

function UsageEventRow({ item }: { item: UsageEvent }) {
  return (
    <div className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1.1fr)_repeat(5,minmax(0,0.58fr))] lg:items-center">
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 className="truncate text-base font-semibold text-slate-950">{item.customer_id}</h3>
          <span className="rounded-full border border-slate-200 bg-white px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-600">{item.quantity} units</span>
        </div>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{item.id}</p>
        <p className="mt-2 text-sm text-slate-600">Observed {formatRelativeTimestamp(item.timestamp)}.</p>
      </div>
      <StatusCell label="Meter" value={item.meter_id} mono />
      <StatusCell label="Subscription" value={item.subscription_id || "-"} mono />
      <StatusCell label="Occurred at" value={formatExactTimestamp(item.timestamp)} />
      <StatusCell label="Idempotency key" value={item.idempotency_key || "-"} mono />
      <StatusCell label="Quantity" value={String(item.quantity)} />
      <StatusCell label="Tenant" value={item.tenant_id || "-"} mono />
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm">
      <p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-950">{value}</p>
    </div>
  );
}

function StatusCell({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 break-all text-sm text-slate-950 ${mono ? "font-mono" : ""}`}>{value}</p>
    </div>
  );
}

function InputField({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
      />
    </label>
  );
}

function DateField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="grid gap-2 text-sm text-slate-700">
      <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</span>
      <input
        type="datetime-local"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-10 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
      />
    </label>
  );
}

function LoadingState() {
  return (
    <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600">
      <LoaderCircle className="h-4 w-4 animate-spin" />
      Loading usage events
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600">
      <p className="font-semibold text-slate-950">No usage events matched the current filters.</p>
      <p className="mt-2">Narrowing by customer, meter, and time window is expected for this operational view.</p>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-rose-200 bg-rose-50 px-5 py-4 text-sm text-rose-700">
      <p className="font-semibold text-rose-900">Usage events could not be loaded</p>
      <p className="mt-2">{message}</p>
    </div>
  );
}
