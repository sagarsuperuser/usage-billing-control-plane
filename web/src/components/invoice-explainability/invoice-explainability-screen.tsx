"use client";

import { useMemo, useState } from "react";
import { LoaderCircle, RefreshCw, Search } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { useUISession } from "@/hooks/use-ui-session";
import { fetchInvoiceExplainability } from "@/lib/api";
import { formatExactTimestamp, formatMoney } from "@/lib/format";

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

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_40%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-12 h-80 w-80 rounded-full bg-cyan-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-amber-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Invoice Explainability</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">
                Line Item Computation Trace
              </h1>
              <p className="mt-2 max-w-3xl text-sm text-slate-300 md:text-base">
                Explain how each invoice line was computed from Lago fees with deterministic digest output.
              </p>
            </div>
            <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
              <MetricCard label="Line items" value={explainabilityQuery.data?.line_items_count ?? 0} />
              <MetricCard
                label="Displayed"
                value={lineItems.length}
                tone={lineItems.length > 0 ? "normal" : "muted"}
              />
              <MetricCard
                label="Status"
                value={explainabilityQuery.data?.invoice_status || "idle"}
                tone={explainabilityQuery.data?.invoice_status ? "normal" : "muted"}
              />
            </div>
          </div>

          <div className="mt-6 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
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
              <label className="text-xs font-medium uppercase tracking-wider text-slate-300">Sort</label>
              <select
                data-testid="explainability-sort"
                className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
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
          </div>

          <div className="mt-3 flex flex-wrap items-end gap-3">
            <InputField
              label="Page"
              value={page}
              onChange={setPage}
              placeholder="1"
              className="w-[120px]"
              dataTestID="explainability-page"
            />
            <InputField
              label="Limit"
              value={limit}
              onChange={setLimit}
              placeholder="50"
              className="w-[120px]"
              dataTestID="explainability-limit"
            />
            <button
              type="button"
              data-testid="explainability-load"
              onClick={() => setSubmittedInvoiceID(invoiceID.trim())}
              disabled={!isTenantSession || !invoiceID.trim()}
              className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <Search className="h-4 w-4" />
              Load Explainability
            </button>
            <button
              type="button"
              data-testid="explainability-refresh"
              onClick={() => explainabilityQuery.refetch()}
              disabled={explainabilityQuery.isFetching || !submittedInvoiceID || !isTenantSession}
              className="inline-flex h-11 items-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/10 px-4 text-sm text-emerald-100 transition hover:bg-emerald-500/20 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {explainabilityQuery.isFetching ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Refresh
            </button>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="Explainability is tenant-scoped. Sign in with a tenant reader, writer, or admin API key to inspect invoice computation traces."
            actionHref="/billing-connections"
            actionLabel="Open platform home"
          />
        ) : null}

        {isTenantSession && explainabilityQuery.error ? (
          <section data-testid="explainability-error" className="rounded-2xl border border-rose-400/40 bg-rose-500/10 p-4 text-sm text-rose-100">
            {(explainabilityQuery.error as Error).message}
          </section>
        ) : null}

        <section className="rounded-2xl border border-white/10 bg-slate-900/75 p-4 backdrop-blur">
          <div className="grid gap-2 md:grid-cols-2">
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

        <section className="rounded-2xl border border-white/10 bg-slate-900/75 p-3 backdrop-blur">
          <div className="overflow-auto">
            <table className="w-full min-w-[1180px] border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-xs uppercase tracking-wider text-slate-400">
                  <th className="px-3 py-1">Item</th>
                  <th className="px-3 py-1">Computation</th>
                  <th className="px-3 py-1">Rule Ref</th>
                  <th className="px-3 py-1">Units/Events</th>
                  <th className="px-3 py-1">Amount</th>
                  <th className="px-3 py-1">Period</th>
                  <th className="px-3 py-1">Properties</th>
                </tr>
              </thead>
              <tbody>
                {lineItems.map((line) => (
                  <tr key={line.fee_id} data-testid={`explainability-line-item-${line.fee_id}`} className="bg-slate-950/75">
                    <td className="rounded-l-xl px-3 py-3 align-top">
                      <p className="font-medium text-cyan-100">{line.item_name}</p>
                      <p className="text-xs text-slate-400">{line.item_code || "-"}</p>
                      <p className="text-xs text-slate-500">{line.fee_id}</p>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <p>{line.computation_mode}</p>
                      <p className="text-xs text-slate-400">{line.fee_type || "-"}</p>
                    </td>
                    <td className="px-3 py-3 align-top text-xs text-slate-300">{line.rule_reference}</td>
                    <td className="px-3 py-3 align-top">
                      <p>Units: {line.units ?? "-"}</p>
                      <p className="text-xs text-slate-400">Events: {line.events_count ?? "-"}</p>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <p>{formatMoney(line.amount_cents, explainabilityQuery.data?.currency || "USD")}</p>
                      <p className="text-xs text-slate-400">
                        Tax {formatMoney(line.taxes_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </p>
                      <p className="text-xs text-emerald-200">
                        Total {formatMoney(line.total_amount_cents, explainabilityQuery.data?.currency || "USD")}
                      </p>
                    </td>
                    <td className="px-3 py-3 align-top text-xs text-slate-300">
                      <p>{line.from_datetime ? formatExactTimestamp(line.from_datetime) : "-"}</p>
                      <p>{line.to_datetime ? formatExactTimestamp(line.to_datetime) : "-"}</p>
                    </td>
                    <td className="rounded-r-xl px-3 py-3 align-top text-xs text-slate-300">
                      <pre className="max-w-[360px] overflow-x-auto whitespace-pre-wrap break-words rounded-lg bg-slate-900/90 p-2 text-[11px] leading-4">
                        {JSON.stringify(line.properties ?? {}, null, 2)}
                      </pre>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {lineItems.length === 0 && !explainabilityQuery.isFetching ? (
            <div data-testid="explainability-empty" className="px-4 py-8 text-center text-sm text-slate-300">
              No line items yet. Load an invoice to inspect explainability.
            </div>
          ) : null}
        </section>
      </main>
    </div>
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
        tone === "normal" ? "border-cyan-400/20 bg-cyan-400/10" : "border-slate-600/40 bg-slate-700/20"
      }`}
    >
      <p className="text-[10px] uppercase tracking-[0.16em] text-slate-300">{label}</p>
      <p className="mt-1 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}

function MetaRow({ label, value, mono, dataTestID }: { label: string; value: string; mono?: boolean; dataTestID?: string }) {
  return (
    <div data-testid={dataTestID} className="rounded-xl border border-white/10 bg-slate-950/70 px-3 py-2">
      <p className="text-[10px] uppercase tracking-[0.16em] text-slate-400">{label}</p>
      <p className={`mt-1 text-sm text-slate-100 ${mono ? "break-all font-mono" : ""}`}>{value}</p>
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
      <label className="text-xs font-medium uppercase tracking-wider text-slate-300">{label}</label>
      <input
        type={sensitive ? "password" : "text"}
        data-testid={dataTestID}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
