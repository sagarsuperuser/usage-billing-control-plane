
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
    <section className="rounded-2xl border border-stone-200 bg-white p-6 shadow-sm">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Why Alpha thinks this failed</p>
      <p className="mt-2 text-sm text-slate-600">
        These are the concrete billing signals behind the current diagnosis. If they look wrong, inspect the correlated events before escalating to logs.
      </p>
      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        <OperatorHint title="Evidence rule" body="Read these fields as the minimum proof set for the current diagnosis. Missing evidence is itself a signal." />
        <OperatorHint title="Triage rule" body="If the evidence contradicts the diagnosis, use the correlated events and raw lifecycle record before retrying or escalating." />
      </div>
      <div className="mt-5 grid gap-3 md:grid-cols-2">
        {items.map((item) => (
          <div key={`${item.label}:${item.value}`} className="rounded-xl border border-stone-200 bg-slate-50 px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{item.label}</p>
            <p className="mt-2 break-words text-sm font-medium text-slate-900">{item.value}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function OperatorHint({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-xl border border-stone-200 bg-slate-50 px-4 py-3">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{title}</p>
      <p className="mt-2 text-sm leading-6 text-slate-700">{body}</p>
    </div>
  );
}
