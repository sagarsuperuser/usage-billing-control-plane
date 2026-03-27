"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchTax } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingTaxDetailScreen({ taxID }: { taxID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();

  const taxQuery = useQuery({
    queryKey: ["pricing-tax", apiBaseURL, taxID],
    queryFn: () => fetchTax({ runtimeBaseURL: apiBaseURL, taxID }),
    enabled: isAuthenticated && scope === "tenant" && taxID.trim().length > 0,
  });

  const tax = taxQuery.data ?? null;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/taxes", label: "Taxes" }, { label: tax?.name || taxID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Taxes are workspace-scoped. Sign in with a workspace account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {taxQuery.isLoading ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm"><div className="flex items-center gap-2"><LoaderCircle className="h-4 w-4 animate-spin" />Loading tax detail</div></section>
        ) : !tax ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing tax</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Tax not available</h1>
            <Link href="/pricing/taxes" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to taxes</Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace tax rule</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{tax.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{tax.code}</p>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">{tax.description || "No description provided."}</p>
                </div>
                <Link href="/pricing/taxes" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to taxes</Link>
              </div>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <div className="grid gap-5">
                <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <Stat label="Status" value={tax.status} />
                  <Stat label="Rate" value={tax.rate.toFixed(2) + "%"} />
                  <Stat
                    label="Availability"
                    value={tax.status === "active" ? "Ready to assign" : tax.status === "draft" ? "Draft" : "Archived"}
                  />
                  <Stat label="Scope" value="Customers / entity" />
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Tax rule</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Assignment posture</h2>
                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    <InfoCell label="Tax code" value={tax.code} mono />
                    <InfoCell label="Status" value={tax.status} />
                    <InfoCell label="Availability" value={tax.status === "active" ? "Ready to assign" : tax.status === "draft" ? "Draft" : "Archived"} />
                    <InfoCell label="Commercial use" value="Reusable customer and workspace tax rule" />
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <GuidanceCard title="Operator posture" body="Use tax detail to confirm the reusable rule before it is assigned to customer billing profiles or workspace billing settings." />
                <GuidanceCard title="Next action" body="Apply active taxes through billing settings and keep code changes deliberate so invoice behavior stays explainable." />
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm"><p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p><p className="mt-2 text-base font-semibold text-slate-950">{value}</p></div>;
}

function InfoCell({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <div className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-4"><p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p><p className={`mt-2 text-sm font-semibold text-slate-950 ${mono ? "break-all font-mono" : ""}`}>{value}</p></div>;
}

function GuidanceCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}
