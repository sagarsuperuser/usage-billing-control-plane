"use client";

import { useEffect, useMemo, useState } from "react";
import { LoaderCircle, RefreshCw, Search } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchInvoiceExplainability } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { type InvoiceExplainabilityLineItem } from "@/lib/types";

const sortOptions = [
  "created_at_asc",
  "created_at_desc",
  "amount_cents_asc",
  "amount_cents_desc",
] as const;

type ExplainabilitySort = (typeof sortOptions)[number];

export function InvoiceExplainabilityScreen() {
  const searchParams = useSearchParams();
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const initialInvoiceID = searchParams.get("invoice_id") || "";
  const [invoiceID, setInvoiceID] = useState(initialInvoiceID);
  const [feeTypesInput, setFeeTypesInput] = useState("");
  const [lineItemSort, setLineItemSort] = useState<ExplainabilitySort>("created_at_asc");
  const [page, setPage] = useState("1");
  const [limit, setLimit] = useState("50");
  const [submittedInvoiceID, setSubmittedInvoiceID] = useState(initialInvoiceID);
  const [selectedLineItemID, setSelectedLineItemID] = useState("");

  const normalizedFeeTypes = useMemo(
    () =>
      feeTypesInput
        .split(",")
        .map((part) => part.trim())
        .filter(Boolean),
    [feeTypesInput]
  );

  const parsedPage = useMemo(() => {
    const value = Number(page);
    if (!Number.isFinite(value) || value < 0) return 1;
    return Math.floor(value);
  }, [page]);

  const parsedLimit = useMemo(() => {
    const value = Number(limit);
    if (!Number.isFinite(value) || value < 0) return 50;
    return Math.floor(value);
  }, [limit]);

  const explainabilityQuery = useQuery({
    queryKey: [
      "invoice-explainability",
      apiBaseURL,
      submittedInvoiceID,
      normalizedFeeTypes.join(","),
      lineItemSort,
      parsedPage,
      parsedLimit,
    ],
    queryFn: () =>
      fetchInvoiceExplainability({
        runtimeBaseURL: apiBaseURL,
        invoiceID: submittedInvoiceID,
        feeTypes: normalizedFeeTypes.length > 0 ? normalizedFeeTypes : undefined,
        lineItemSort,
        page: parsedPage,
        limit: parsedLimit,
      }),
    enabled: isTenantSession && submittedInvoiceID.length > 0,
  });

  const lineItems = explainabilityQuery.data?.line_items ?? [];
  const selectedLineItem = lineItems.find((item) => item.fee_id === selectedLineItemID) ?? null;

  useEffect(() => {
    if (lineItems.length === 0) {
      setSelectedLineItemID("");
      return;
    }
    if (selectedLineItemID && !lineItems.some((item) => item.fee_id === selectedLineItemID)) {
      setSelectedLineItemID("");
    }
  }, [lineItems, selectedLineItemID]);

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { href: "/invoices", label: "Invoices" },
            { label: "Explainability" },
          ]}
        />

        <section className="rounded-3xl border border-stone-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-emerald-700">Invoice explainability</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900 md:text-4xl">
                Line Item Computation Trace
              </h1>
              <p className="mt-2 max-w-3xl text-sm text-slate-600 md:text-base">
                Review the computation digest behind each invoice line before escalating a billing dispute or replay issue.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
              <MetricCard label="Line items" value={explainabilityQuery.data?.line_items_count ?? 0} />
              <MetricCard label="Displayed" value={lineItems.length} tone={lineItems.length > 0 ? "normal" : "muted"} />
              <MetricCard
                label="Status"
                value={explainabilityQuery.data?.invoice_status || "idle"}
                tone={explainabilityQuery.data?.invoice_status ? "normal" : "muted"}
              />
            </div>
          </div>

          <div className="mt-5 grid gap-3 lg:grid-cols-3">
            <CompactRule title="Trace source" body="Load one invoice and inspect how each line item was produced." />
            <CompactRule title="Operational use" body="Use this before escalating a billing question or replay issue." />
            <CompactRule title="Output shape" body="Keep the list compact and open one line item when you need raw properties." />
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Explainability is workspace-scoped. Sign in with a workspace account to inspect invoice computation traces."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession && explainabilityQuery.error ? (
          <section data-testid="explainability-error" className="rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
            {(explainabilityQuery.error as Error).message}
          </section>
        ) : null}

        <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Trace Query</p>
              <h2 className="mt-1 text-lg font-semibold text-slate-900">Control the invoice slice you want to inspect</h2>
              <p className="mt-2 max-w-2xl text-sm text-slate-600">
                Narrow the digest by fee type, sort order, and pagination before loading or refreshing the trace.
              </p>
            </div>
            <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3 text-sm text-slate-600 xl:max-w-sm">
              <p className="text-xs uppercase tracking-[0.16em] text-slate-500">Current target</p>
              <p className="mt-2 font-medium text-slate-900">{submittedInvoiceID || invoiceID.trim() || "No invoice selected"}</p>
              <p className="mt-1 text-xs text-slate-500">Load an invoice first, then use refresh to rerun the same trace parameters.</p>
            </div>
          </div>

          <div className="mt-5 grid gap-5 xl:grid-cols-[1.15fr_0.85fr]">
            <div className="grid gap-4">
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                <InputField
                  label="Invoice ID"
                  value={invoiceID}
                  onChange={setInvoiceID}
                  placeholder="inv_123"
                  dataTestID="explainability-invoice-id"
                />
                <InputField
                  label="Fee Types"
                  value={feeTypesInput}
                  onChange={setFeeTypesInput}
                  placeholder="charge,subscription"
                  dataTestID="explainability-fee-types"
                />
                <div className="grid gap-2">
                  <label className="text-xs font-medium uppercase tracking-wider text-slate-600">Sort</label>
                  <select
                    data-testid="explainability-sort"
                    className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2"
                    value={lineItemSort}
                    onChange={(event) => setLineItemSort(event.target.value as ExplainabilitySort)}
                  >
                    {sortOptions.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>
                <InputField
                  label="Page"
                  value={page}
                  onChange={setPage}
                  placeholder="1"
                  className="w-full xl:max-w-[120px]"
                  dataTestID="explainability-page"
                />
                <InputField
                  label="Limit"
                  value={limit}
                  onChange={setLimit}
                  placeholder="50"
                  className="w-full xl:max-w-[120px]"
                  dataTestID="explainability-limit"
                />
              </div>

              <div className="flex flex-wrap items-end gap-3">
                <button
                  type="button"
                  data-testid="explainability-load"
                  onClick={() => setSubmittedInvoiceID(invoiceID.trim())}
                  disabled={!isTenantSession || !invoiceID.trim()}
                  className="inline-flex h-11 items-center gap-2 rounded-xl border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Search className="h-4 w-4" />
                  Load Explainability
                </button>
                <button
                  type="button"
                  data-testid="explainability-refresh"
                  onClick={() => explainabilityQuery.refetch()}
                  disabled={explainabilityQuery.isFetching || !submittedInvoiceID || !isTenantSession}
                  className="inline-flex h-11 items-center gap-2 rounded-xl border border-stone-300 bg-white px-4 text-sm font-medium text-slate-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {explainabilityQuery.isFetching ? (
                    <LoaderCircle className="h-4 w-4 animate-spin" />
                  ) : (
                    <RefreshCw className="h-4 w-4" />
                  )}
                  Refresh
                </button>
              </div>
            </div>

            <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Use this view</p>
              <h3 className="mt-1 text-base font-semibold text-slate-900">Check the invoice before escalating</h3>
              <ul className="mt-3 space-y-2 text-sm text-slate-600">
                <li>Confirm invoice scope and fee filters before reading the raw properties payload.</li>
                <li>Use the digest and generated timestamp to verify the exact computation snapshot under review.</li>
                <li>Escalate to replay or payment recovery only after validating the billing logic itself.</li>
              </ul>
            </div>
          </div>
        </section>

        <section className="rounded-2xl border border-stone-200 bg-white p-4 shadow-sm">
          <div className="flex flex-col gap-3 border-b border-stone-200 pb-4 md:flex-row md:items-end md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Digest Summary</p>
              <h2 className="mt-1 text-lg font-semibold text-slate-900">Invoice metadata and trace fingerprint</h2>
            </div>
            <p className="max-w-xl text-sm text-slate-600">
              Validate invoice identity, currency totals, and digest version before using the line-item breakdown.
            </p>
          </div>
          <div className="mt-4 grid gap-2 md:grid-cols-2">
            <MetaRow label="Invoice" value={explainabilityQuery.data?.invoice_number || submittedInvoiceID || "-"} dataTestID="explainability-meta-invoice" />
            <MetaRow label="Invoice ID" value={explainabilityQuery.data?.invoice_id || "-"} dataTestID="explainability-meta-invoice-id" />
            <MetaRow
              label="Total Amount"
              value={
                explainabilityQuery.data
                  ? formatMoney(explainabilityQuery.data.total_amount_cents, explainabilityQuery.data.currency || "USD")
                  : "-"
              }
              dataTestID="explainability-meta-total"
            />
            <MetaRow
              label="Generated At"
              value={explainabilityQuery.data ? formatExactTimestamp(explainabilityQuery.data.generated_at) : "-"}
              dataTestID="explainability-meta-generated-at"
            />
            <MetaRow label="Digest Version" value={explainabilityQuery.data?.explainability_version || "-"} dataTestID="explainability-meta-version" />
            <MetaRow
              label="Digest"
              value={explainabilityQuery.data?.explainability_digest || "-"}
              mono
              dataTestID="explainability-meta-digest"
            />
          </div>
        </section>

        <section className="rounded-2xl border border-stone-200 bg-white p-3 shadow-sm">
          <div className="flex flex-col gap-3 border-b border-stone-200 px-3 pb-4 pt-1 md:flex-row md:items-end md:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Computation Breakdown</p>
              <h2 className="mt-1 text-lg font-semibold text-slate-900">Line items, rules, and raw properties</h2>
            </div>
            <p className="max-w-2xl text-sm text-slate-600">
              Review the billable item, rule reference, units, and raw properties together so debugging stays tied to the amount outcome.
            </p>
          </div>
          <div className="mt-3 grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
            <div className="overflow-auto">
              <table className="w-full min-w-[920px] border-separate border-spacing-y-2 text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase tracking-wider text-slate-500">
                    <th className="px-3 py-1">Item</th>
                    <th className="px-3 py-1">Computation</th>
                    <th className="px-3 py-1">Amount</th>
                    <th className="px-3 py-1">Period</th>
                    <th className="px-3 py-1">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {lineItems.map((line) => (
                    <tr key={line.fee_id} data-testid={`explainability-line-item-${line.fee_id}`} className="bg-stone-50/80">
                      <td className="rounded-l-xl px-3 py-3 align-top">
                        <p className="font-medium text-slate-900">{line.item_name}</p>
                        <p className="text-xs text-slate-500">{line.item_code || "-"}</p>
                      </td>
                      <td className="px-3 py-3 align-top">
                        <p>{line.computation_mode}</p>
                        <p className="text-xs text-slate-500">{line.fee_type || "-"}</p>
                      </td>
                      <td className="px-3 py-3 align-top">
                        <p>{formatMoney(line.amount_cents, explainabilityQuery.data?.currency || "USD")}</p>
                        <p className="text-xs text-slate-500">
                          Tax {formatMoney(line.taxes_amount_cents, explainabilityQuery.data?.currency || "USD")}
                        </p>
                        <p className="text-xs text-emerald-700">
                          Total {formatMoney(line.total_amount_cents, explainabilityQuery.data?.currency || "USD")}
                        </p>
                      </td>
                      <td className="px-3 py-3 align-top text-xs text-slate-600">
                        <p>{line.from_datetime ? formatExactTimestamp(line.from_datetime) : "-"}</p>
                        <p>{line.to_datetime ? formatExactTimestamp(line.to_datetime) : "-"}</p>
                      </td>
                      <td className="rounded-r-xl px-3 py-3 align-top">
                        <button
                          type="button"
                          data-testid={`explainability-view-line-item-${line.fee_id}`}
                          onClick={() => setSelectedLineItemID(line.fee_id)}
                          className="inline-flex h-9 items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-3 text-xs uppercase tracking-[0.14em] text-emerald-700 transition hover:bg-emerald-100"
                        >
                          View details
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ExplainabilityLineItemDetail
              line={selectedLineItem}
              currency={explainabilityQuery.data?.currency || "USD"}
            />
          </div>
          {lineItems.length === 0 && !explainabilityQuery.isFetching ? (
            <div data-testid="explainability-empty" className="px-4 py-8 text-center text-sm text-slate-600">
              No line items yet. Load an invoice to inspect explainability.
            </div>
          ) : null}
        </section>
      </main>
    </div>
  );
}

function ExplainabilityLineItemDetail({
  line,
  currency,
}: {
  line: InvoiceExplainabilityLineItem | null;
  currency: string;
}) {
  if (!line) {
    return (
      <aside className="rounded-2xl border border-dashed border-stone-300 bg-stone-50 px-4 py-8 text-sm text-slate-600">
        Select a line item to inspect rule references, identifiers, and raw properties.
      </aside>
    );
  }

  return (
    <aside className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
      <p className="text-xs uppercase tracking-[0.18em] text-slate-500">Line item detail</p>
      <div className="mt-3 grid gap-3">
        <MetaRow label="Item" value={line.item_name} />
        <MetaRow label="Fee ID" value={line.fee_id} mono />
        <MetaRow label="Rule reference" value={line.rule_reference} mono />
        <MetaRow label="Units" value={line.units !== undefined ? String(line.units) : "-"} />
        <MetaRow label="Events" value={line.events_count !== undefined ? String(line.events_count) : "-"} />
        <MetaRow label="Charge model" value={line.charge_model || "-"} />
        <MetaRow label="Subscription" value={line.subscription_id || "-"} mono />
        <MetaRow label="Charge ID" value={line.charge_id || "-"} mono />
        <MetaRow label="Billable metric" value={line.billable_metric_code || "-"} mono />
        <MetaRow label="Line total" value={formatMoney(line.total_amount_cents, currency)} />
      </div>
      <div className="mt-4 rounded-xl border border-stone-200 bg-white p-3">
        <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">Properties</p>
        <pre className="mt-2 max-h-[280px] overflow-auto whitespace-pre-wrap break-words text-[11px] leading-4 text-slate-700">
          {JSON.stringify(line.properties ?? {}, null, 2)}
        </pre>
      </div>
    </aside>
  );
}

function MetricCard({
  label,
  value,
  tone = "normal",
}: {
  label: string;
  value: string | number;
  tone?: "normal" | "muted";
}) {
  return (
    <div
      className={`rounded-xl border px-3 py-2 ${
        tone === "normal" ? "border-emerald-200 bg-emerald-50" : "border-stone-200 bg-stone-50"
      }`}
    >
      <p className="text-[10px] uppercase tracking-[0.16em] text-slate-600">{label}</p>
      <p className="mt-1 text-lg font-semibold text-slate-900">{value}</p>
    </div>
  );
}

function CompactRule({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
      <p className="text-xs uppercase tracking-[0.18em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-600">{body}</p>
    </div>
  );
}

function MetaRow({ label, value, mono, dataTestID }: { label: string; value: string; mono?: boolean; dataTestID?: string }) {
  return (
    <div data-testid={dataTestID} className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-2">
      <p className="text-[10px] uppercase tracking-[0.16em] text-slate-500">{label}</p>
      <p className={`mt-1 text-sm text-slate-700 ${mono ? "break-all font-mono" : ""}`}>{value}</p>
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
  sensitive,
  className,
  dataTestID,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  sensitive?: boolean;
  className?: string;
  dataTestID?: string;
}) {
  return (
    <div className={`grid gap-2 ${className || ""}`}>
      <label className="text-xs font-medium uppercase tracking-wider text-slate-600">{label}</label>
      <input
        type={sensitive ? "password" : "text"}
        data-testid={dataTestID}
        className="h-11 rounded-xl border border-stone-200 bg-white px-3 text-sm text-slate-900 outline-none ring-emerald-500 transition focus:ring-2"
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
