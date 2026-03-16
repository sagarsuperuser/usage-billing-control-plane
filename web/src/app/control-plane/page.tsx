import Link from "next/link";
import { Activity, Building2, ReceiptText, UserRoundPlus, Workflow } from "lucide-react";
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
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Alpha Admin</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">
            Billing operations that feel product-first
          </h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
            Run workspace setup, customer onboarding, payment recovery, and billing diagnostics from Alpha.
            Lago stays behind the scenes as the billing engine.
          </p>
        </section>

        <section className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
          <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Get Started</p>
            <h2 className="mt-2 text-2xl font-semibold text-white">Primary onboarding journeys</h2>
            <p className="mt-3 max-w-2xl text-sm text-slate-300">
              Use guided setup first. Advanced recovery and diagnostics stay available, but they should not be
              the starting point for normal onboarding.
            </p>
            <div className="mt-5 grid gap-4 md:grid-cols-2">
              <Card
                href="/tenant-onboarding"
                title="Create workspace"
                description="Create a tenant workspace, connect billing, and hand off the first admin credential."
                icon={<Building2 className="h-5 w-5 text-sky-200" />}
                accent="border-sky-400/40 bg-sky-500/10"
              />
              <Card
                href="/customer-onboarding"
                title="Onboard customer"
                description="Create the first billable customer, sync the billing profile, and start payment setup."
                icon={<UserRoundPlus className="h-5 w-5 text-teal-200" />}
                accent="border-teal-400/40 bg-teal-500/10"
              />
            </div>
          </div>

          <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Needs Attention</p>
            <h2 className="mt-2 text-2xl font-semibold text-white">Operational focus</h2>
            <div className="mt-5 grid gap-3">
              <FocusLine title="Workspaces missing pricing" body="Use workspace setup to finish pricing before billing starts." />
              <FocusLine title="Customers waiting on payment setup" body="Use customer onboarding and payment refresh to reach verified readiness." />
              <FocusLine title="Billing recovery" body="Use Payments and Recovery when onboarding is complete but runtime issues need action." />
            </div>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <Card
            href="/tenant-onboarding"
            title="Workspace Setup"
            description="Guided platform flow for workspace creation, billing connection, admin access, and readiness review."
            icon={<Building2 className="h-5 w-5 text-sky-200" />}
            accent="border-sky-400/40 bg-sky-500/10"
          />
          <Card
            href="/customer-onboarding"
            title="Customers"
            description="Guided customer onboarding plus advanced billing sync and payment setup recovery when needed."
            icon={<UserRoundPlus className="h-5 w-5 text-teal-200" />}
            accent="border-teal-400/40 bg-teal-500/10"
          />
          <Card
            href="/payment-operations"
            title="Payments"
            description="Monitor invoice payment failures, inspect webhook history, and trigger payment retries."
            icon={<Activity className="h-5 w-5 text-cyan-200" />}
            accent="border-cyan-400/40 bg-cyan-500/10"
          />
          <Card
            href="/replay-operations"
            title="Recovery"
            description="Queue replay jobs, inspect diagnostics, and recover failed reprocessing runs."
            icon={<Workflow className="h-5 w-5 text-amber-200" />}
            accent="border-amber-400/40 bg-amber-500/10"
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

function FocusLine({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/50 px-4 py-4">
      <p className="text-sm font-semibold text-white">{title}</p>
      <p className="mt-1 text-sm text-slate-300">{body}</p>
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
