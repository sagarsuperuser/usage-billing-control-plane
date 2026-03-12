import Link from "next/link";
import { Activity, ReceiptText } from "lucide-react";
import { type ReactNode } from "react";

import { ControlPlaneNav } from "@/components/layout/control-plane-nav";

export default function ControlPlanePage() {
  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute -left-20 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-orange-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Lago Alpha Control Plane</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">
            Usage Billing Operations
          </h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
            Keep Lago as billing engine, and run product-specific operations from this control plane:
            payment failure triage, invoice explainability, replay, and reconciliation workflows.
          </p>
        </section>

        <section className="grid gap-4 md:grid-cols-2">
          <Card
            href="/payment-operations"
            title="Payment Operations"
            description="Monitor invoice payment failures, inspect webhook timeline, and trigger retry-payment."
            icon={<Activity className="h-5 w-5 text-cyan-200" />}
            accent="border-cyan-400/40 bg-cyan-500/10"
          />
          <Card
            href="/invoice-explainability"
            title="Invoice Explainability"
            description="Show deterministic line-item computation trace and digest for financial correctness workflows."
            icon={<ReceiptText className="h-5 w-5 text-emerald-200" />}
            accent="border-emerald-400/40 bg-emerald-500/10"
          />
        </section>
      </main>
    </div>
  );
}

function Card({
  href,
  title,
  description,
  icon,
  accent,
}: {
  href: string;
  title: string;
  description: string;
  icon: ReactNode;
  accent: string;
}) {
  return (
    <Link
      href={href}
      className={`group rounded-2xl border p-5 transition hover:-translate-y-0.5 hover:bg-slate-900/90 ${accent}`}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white">{title}</h2>
        <span className="inline-flex rounded-lg border border-white/20 bg-white/5 p-2">{icon}</span>
      </div>
      <p className="mt-3 text-sm leading-6 text-slate-200">{description}</p>
      <p className="mt-4 text-xs font-semibold uppercase tracking-[0.16em] text-slate-300 group-hover:text-white">
        Open
      </p>
    </Link>
  );
}
