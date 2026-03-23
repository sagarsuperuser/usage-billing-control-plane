"use client";

import type { BillingEvidenceItem } from "@/lib/billing-lifecycle";

export function BillingFailureEvidence({
  items,
}: {
  items: BillingEvidenceItem[];
}) {
  if (items.length === 0) {
    return null;
  }

  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Why Alpha thinks this failed</p>
      <p className="mt-2 text-sm text-slate-600">
        These are the concrete billing signals behind the current diagnosis. If they look wrong, inspect the correlated events before escalating to logs.
      </p>
      <div className="mt-5 grid gap-3 md:grid-cols-2">
        {items.map((item) => (
          <div key={`${item.label}:${item.value}`} className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{item.label}</p>
            <p className="mt-2 break-words text-sm font-medium text-slate-900">{item.value}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
