
import { AlertCircle, CheckCircle2, TriangleAlert } from "lucide-react";

import { type BillingDiagnosis } from "@/lib/billing-lifecycle";

const toneClasses: Record<BillingDiagnosis["tone"], string> = {
  healthy: "border-emerald-200 bg-emerald-50 text-emerald-900",
  warning: "border-amber-200 bg-amber-50 text-amber-900",
  danger: "border-rose-200 bg-rose-50 text-rose-900",
};

function ToneIcon({ tone }: { tone: BillingDiagnosis["tone"] }) {
  switch (tone) {
    case "healthy":
      return <CheckCircle2 className="h-4 w-4" />;
    case "warning":
      return <TriangleAlert className="h-4 w-4" />;
    default:
      return <AlertCircle className="h-4 w-4" />;
  }
}

export function BillingFailureDiagnosisCard({
  diagnosis,
  label = "Failure diagnosis",
}: {
  diagnosis: BillingDiagnosis;
  label?: string;
}) {
  return (
    <section className={`rounded-2xl border p-6 shadow-sm ${toneClasses[diagnosis.tone]}`}>
      <div className="flex items-start gap-3">
        <div className="mt-0.5 shrink-0">
          <ToneIcon tone={diagnosis.tone} />
        </div>
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-[0.16em] opacity-80">{label}</p>
          <h2 className="mt-2 text-lg font-semibold">{diagnosis.title}</h2>
          <p className="mt-2 text-sm opacity-90">{diagnosis.summary}</p>
          <div className="mt-4 grid gap-3 lg:grid-cols-2">
            <div className="rounded-xl border border-current/15 bg-surface/50 px-4 py-3 text-sm">
              <p className="font-semibold">Operator rule</p>
              <p className="mt-2">Use this diagnosis as the current operational stance, then verify it against evidence and timeline before taking recovery action.</p>
            </div>
            <div className="rounded-xl border border-current/15 bg-surface/50 px-4 py-3 text-sm">
              <p className="font-semibold">Before acting</p>
              <p className="mt-2">Do not act on the failure message alone. Verify the issue against the full billing evidence and timeline first.</p>
            </div>
          </div>
          <div className="mt-4 rounded-xl border border-current/15 bg-surface/50 px-4 py-3 text-sm">
            <p className="font-semibold">Next step</p>
            <p className="mt-2">{diagnosis.nextStep}</p>
          </div>
        </div>
      </div>
    </section>
  );
}
