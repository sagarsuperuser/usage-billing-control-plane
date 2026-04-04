"use client";

import { useMemo, useState } from "react";
import { LoaderCircle, Search, X } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchInvoiceExplainability } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";
import { type InvoiceExplainabilityLineItem } from "@/lib/types";

const feeTypeOptions = ["", "charge", "subscription", "add_on", "credit", "minimum_commitment"] as const;

export function InvoiceExplainabilityScreen() {
  const searchParams = useSearchParams();
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
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
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs
          items={[
            { href: "/control-plane", label: "Workspace" },
            { href: "/invoices", label: "Invoices" },
            { label: "Explainability" },
          ]}
        />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Workspace session required"
            body="Explainability is workspace-scoped. Sign in with a workspace account to inspect invoice computation traces."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession ? (
          <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
            {/* Header with search */}
            <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
              <h1 className="text-sm font-semibold text-slate-900">Invoice explainability</h1>
              <div className="flex items-center gap-2">
                <input
                  value={invoiceID}
                  onChange={(e) => setInvoiceID(e.target.value)}
                  placeholder="Invoice ID..."
                  data-testid="explainability-invoice-id"
                  className="h-8 w-48 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
                />
                <button
                  type="button"
                  data-testid="explainability-load"
                  onClick={() => setSubmittedInvoiceID(invoiceID.trim())}
                  disabled={!invoiceID.trim()}
                  className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-slate-900 bg-slate-900 px-3 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {explainabilityQuery.isFetching ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Search className="h-3.5 w-3.5" />}
                  Load
                </button>
                <select
                  value={feeType}
                  onChange={(e) => setFeeType(e.target.value)}
                  data-testid="explainability-fee-types"
                  className="h-8 rounded-lg border border-stone-200 bg-stone-50 px-3 text-sm text-slate-900 outline-none ring-slate-400 transition focus:ring-2"
                >
                  <option value="">All fee types</option>
                  {feeTypeOptions.filter(Boolean).map((ft) => (
                    <option key={ft} value={ft}>{ft}</option>
                  ))}
                </select>
              </div>
            </div>

            {explainabilityQuery.error ? (
              <div data-testid="explainability-error" className="border-b border-stone-200 px-5 py-3 text-sm text-rose-700">
                {(explainabilityQuery.error as Error).message}
              </div>
            ) : null}

            {/* Invoice metadata bar */}
            {explainabilityQuery.data ? (
              <div className="flex items-center gap-4 border-b border-stone-200 px-5 py-2 text-sm text-slate-600" data-testid="explainability-meta-invoice">
                <span className="font-medium text-slate-900">{explainabilityQuery.data.invoice_number || submittedInvoiceID}</span>
                <span>{explainabilityQuery.data.currency || "USD"}</span>
                <span data-testid="explainability-meta-total">{formatMoney(explainabilityQuery.data.total_amount_cents, explainabilityQuery.data.currency || "USD")}</span>
                <span>{explainabilityQuery.data.line_items_count} line item{explainabilityQuery.data.line_items_count === 1 ? "" : "s"}</span>
                {explainabilityQuery.data.invoice_status ? (
                  <span className="inline-flex rounded-full border border-stone-200 bg-stone-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-slate-600">
                    {explainabilityQuery.data.invoice_status}
                  </span>
                ) : null}
              </div>
            ) : null}

            {/* Line items table */}
            {lineItems.length === 0 && !explainabilityQuery.isFetching ? (
              <div data-testid="explainability-empty" className="flex flex-col items-center justify-center gap-3 px-5 py-16 text-center">
                <p className="text-sm font-medium text-slate-700">No line items</p>
                <p className="text-xs text-slate-500">Load an invoice to inspect explainability.</p>
              </div>
            ) : lineItems.length > 0 ? (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-stone-100 text-left text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">
                    <th className="px-5 py-2.5 font-semibold">Fee</th>
                    <th className="px-4 py-2.5 font-semibold">Item</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Amount</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Tax</th>
                    <th className="px-4 py-2.5 font-semibold text-right">Total</th>
                    <th className="px-4 py-2.5 font-semibold"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {lineItems.map((line) => (
                    <tr
                      key={line.fee_id}
                      data-testid={`explainability-line-item-${line.fee_id}`}
                      className="cursor-pointer transition hover:bg-stone-50"
                      onClick={() => setSelectedLineItemID(line.fee_id)}
                    >
                      <td className="px-5 py-3">
                        <span className="inline-flex rounded-full border border-stone-200 bg-stone-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-slate-600">
                          {line.fee_type || "-"}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        <p className="font-medium text-slate-900">{line.item_name}</p>
                        <p className="mt-0.5 font-mono text-xs text-slate-400">{line.item_code || "-"}</p>
                      </td>
                      <td className="px-4 py-3 text-right text-slate-700">
                        {formatMoney(line.amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3 text-right text-slate-500">
                        {formatMoney(line.taxes_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3 text-right font-medium text-slate-900">
                        {formatMoney(line.total_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          type="button"
                          data-testid={`explainability-view-line-item-${line.fee_id}`}
                          onClick={(e) => { e.stopPropagation(); setSelectedLineItemID(line.fee_id); }}
                          className="text-xs font-medium text-slate-600 hover:text-slate-900"
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
        ) : null}
      </main>

      {/* Line item detail slide-out */}
      {selectedLineItem ? (
        <aside className="fixed inset-y-0 right-0 z-30 flex w-full max-w-[420px] flex-col border-l border-stone-200 bg-white shadow-lg">
          <div className="flex items-center justify-between border-b border-stone-200 px-5 py-3">
            <h2 className="text-sm font-semibold text-slate-900">{selectedLineItem.item_name}</h2>
            <button type="button" onClick={() => setSelectedLineItemID("")} className="text-slate-400 hover:text-slate-700">
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
              <h3 className="text-xs font-semibold uppercase tracking-[0.1em] text-slate-400">Properties</h3>
              <pre className="mt-2 max-h-[300px] overflow-auto rounded-lg border border-stone-200 bg-stone-50 p-3 text-[11px] leading-4 text-slate-700">
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
      <dt className="text-slate-500">{label}</dt>
      <dd className={`text-right text-slate-700 ${mono ? "break-all font-mono text-xs" : ""}`}>{value}</dd>
    </div>
  );
}
