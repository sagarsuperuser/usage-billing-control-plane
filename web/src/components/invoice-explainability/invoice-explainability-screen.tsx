import { useState } from "react";
import { LoaderCircle, Search, X } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParamsCompat } from "@/hooks/use-search-params-compat";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchInvoiceExplainability } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";

const feeTypeOptions = ["", "charge", "subscription", "add_on", "credit", "minimum_commitment"] as const;

export function InvoiceExplainabilityScreen() {
  const searchParams = useSearchParamsCompat();
  const { apiBaseURL, isAuthenticated, isLoading: _sessionLoading, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const initialInvoiceID = searchParams.get("invoice_id") || "";
  const [invoiceID, setInvoiceID] = useState(initialInvoiceID);
  const [feeType, setFeeType] = useState("");
  const [submittedInvoiceID, setSubmittedInvoiceID] = useState(initialInvoiceID);
  const [selectedLineItemID, setSelectedLineItemID] = useState("");

  const explainabilityQuery = useQuery({
    queryKey: ["invoice-explainability", apiBaseURL, submittedInvoiceID, feeType],
    queryFn: () =>
      fetchInvoiceExplainability({
        runtimeBaseURL: apiBaseURL,
        invoiceID: submittedInvoiceID,
        feeTypes: feeType ? [feeType] : undefined,
        lineItemSort: "created_at_asc",
        page: 1,
        limit: 100,
      }),
    enabled: isTenantSession && submittedInvoiceID.length > 0,
  });

  const lineItems = explainabilityQuery.data?.line_items ?? [];
  const selectedLineItem = lineItems.find((item) => item.fee_id === selectedLineItemID) ?? null;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs
          items={[
            { href: "/invoices", label: "Invoices" },
            { label: "Explainability" },
          ]}
        />


        <div className="overflow-hidden rounded-lg border border-border bg-surface shadow-sm">
            {/* Header with search */}
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <h1 className="text-sm font-semibold text-text-primary">Invoice explainability</h1>
              <div className="flex items-center gap-2">
                <input
                  value={invoiceID}
                  onChange={(e) => setInvoiceID(e.target.value)}
                  placeholder="Invoice ID..."
                  data-testid="explainability-invoice-id"
                  className="h-8 w-48 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition placeholder:text-text-faint focus:ring-2"
                />
                <button
                  type="button"
                  data-testid="explainability-load"
                  onClick={() => setSubmittedInvoiceID(invoiceID.trim())}
                  disabled={!invoiceID.trim()}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg bg-blue-600 px-3.5 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {explainabilityQuery.isFetching ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5" />}
                  Load
                </button>
                <select
                  value={feeType}
                  onChange={(e) => setFeeType(e.target.value)}
                  data-testid="explainability-fee-types"
                  className="h-8 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All fee types</option>
                  {feeTypeOptions.filter(Boolean).map((ft) => (
                    <option key={ft} value={ft}>{ft}</option>
                  ))}
                </select>
              </div>
            </div>

            {explainabilityQuery.error ? (
              <div data-testid="explainability-error" className="border-b border-border px-5 py-3 text-sm text-rose-700">
                {(explainabilityQuery.error as Error).message}
              </div>
            ) : null}

            {/* Invoice metadata bar */}
            {explainabilityQuery.data ? (
              <div className="flex items-center gap-4 border-b border-border px-5 py-2 text-sm text-text-muted" data-testid="explainability-meta-invoice">
                <span className="font-medium text-text-primary">{explainabilityQuery.data.invoice_number || submittedInvoiceID}</span>
                <span>{explainabilityQuery.data.currency || "USD"}</span>
                <span data-testid="explainability-meta-total">{formatMoney(explainabilityQuery.data.total_amount_cents, explainabilityQuery.data.currency || "USD")}</span>
                <span>{explainabilityQuery.data.line_items_count} line item{explainabilityQuery.data.line_items_count === 1 ? "" : "s"}</span>
                {explainabilityQuery.data.invoice_status ? (
                  <span className="inline-flex rounded-full border border-border bg-surface-secondary px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-text-muted">
                    {explainabilityQuery.data.invoice_status}
                  </span>
                ) : null}
              </div>
            ) : null}

            {/* Line items table */}
            {lineItems.length === 0 && !explainabilityQuery.isFetching ? (
              <div data-testid="explainability-empty" className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
                <p className="text-sm font-medium text-text-secondary">No line items</p>
                <p className="text-xs text-text-muted">Load an invoice to inspect explainability.</p>
              </div>
            ) : lineItems.length > 0 ? (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border-light text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-text-faint">
                    <th className="px-5 py-2.5 font-semibold">Fee</th>
                    <th className="px-4 py-2.5 font-semibold">Item</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Amount</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Tax</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Total</th>
                    <th className="px-4 py-2.5 font-semibold"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-light">
                  {lineItems.map((line) => (
                    <tr
                      key={line.fee_id}
                      data-testid={`explainability-line-item-${line.fee_id}`}
                      className="cursor-pointer transition hover:bg-surface-secondary"
                      onClick={() => setSelectedLineItemID(line.fee_id)}
                    >
                      <td className="px-5 py-3">
                        <span className="inline-flex rounded-full border border-border bg-surface-secondary px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-text-muted">
                          {line.fee_type || "-"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <p className="font-medium text-text-primary">{line.item_name}</p>
                        <p className="mt-0.5 font-mono text-xs text-text-faint">{line.item_code || "-"}</p>
                      </td>
                      <td className="px-4 py-3 text-right text-text-secondary">
                        {formatMoney(line.amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3 text-right text-text-muted">
                        {formatMoney(line.taxes_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3 text-right font-medium text-text-primary">
                        {formatMoney(line.total_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          data-testid={`explainability-view-line-item-${line.fee_id}`}
                          onClick={(e) => { e.stopPropagation(); setSelectedLineItemID(line.fee_id); }}
                          className="text-xs font-medium text-text-muted hover:text-text-primary"
                        >
                          View →
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : null}
          </div>
      </main>

      {/* Line item detail slide-out */}
      {selectedLineItem ? (
        <aside className="fixed inset-y-0 right-0 z-30 flex w-full max-w-[420px] flex-col border-l border-border bg-surface shadow-lg">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <h2 className="text-sm font-semibold text-text-primary">{selectedLineItem.item_name}</h2>
            <button type="button" onClick={() => setSelectedLineItemID("")} className="text-text-faint hover:text-text-secondary">
              <X className="h-4 w-4" />
            </button>
          </div>
          <div className="flex-1 overflow-auto px-5 py-4">
            <dl className="grid gap-y-2 text-sm">
              <DetailRow label="Fee ID" value={selectedLineItem.fee_id} mono />
              <DetailRow label="Fee type" value={selectedLineItem.fee_type || "-"} />
              <DetailRow label="Computation" value={selectedLineItem.computation_mode} />
              <DetailRow label="Rule reference" value={selectedLineItem.rule_reference} mono />
              <DetailRow label="Units" value={selectedLineItem.units !== undefined ? String(selectedLineItem.units) : "-"} />
              <DetailRow label="Events" value={selectedLineItem.events_count !== undefined ? String(selectedLineItem.events_count) : "-"} />
              <DetailRow label="Charge model" value={selectedLineItem.charge_model || "-"} />
              <DetailRow label="Subscription" value={selectedLineItem.subscription_id || "-"} mono />
              <DetailRow label="Charge ID" value={selectedLineItem.charge_id || "-"} mono />
              <DetailRow label="Billable metric" value={selectedLineItem.billable_metric_code || "-"} mono />
              <DetailRow label="Amount" value={formatMoney(selectedLineItem.amount_cents, explainabilityQuery.data?.currency || "USD")} />
              <DetailRow label="Tax" value={formatMoney(selectedLineItem.taxes_amount_cents, explainabilityQuery.data?.currency || "USD")} />
              <DetailRow label="Total" value={formatMoney(selectedLineItem.total_amount_cents, explainabilityQuery.data?.currency || "USD")} />
              {selectedLineItem.from_datetime ? <DetailRow label="From" value={formatExactTimestamp(selectedLineItem.from_datetime)} /> : null}
              {selectedLineItem.to_datetime ? <DetailRow label="To" value={formatExactTimestamp(selectedLineItem.to_datetime)} /> : null}
            </dl>

            <div className="mt-5">
              <h3 className="text-xs font-semibold uppercase tracking-[0.1em] text-text-faint">Properties</h3>
              <pre className="mt-2 max-h-[300px] overflow-auto rounded-lg border border-border bg-surface-secondary p-3 text-[11px] leading-4 text-text-secondary">
                {JSON.stringify(selectedLineItem.properties ?? {}, null, 2)}
              </pre>
            </div>
          </div>
        </aside>
      ) : null}
    </div>
  );
}

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-text-muted">{label}</dt>
      <dd className={`text-right text-text-secondary ${mono ? "break-all font-mono text-xs" : ""}`}>{value}</dd>
    </div>
  );
}
