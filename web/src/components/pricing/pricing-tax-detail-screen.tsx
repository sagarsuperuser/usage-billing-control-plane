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
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Tenant session required" body="Taxes are tenant-scoped. Sign in with a tenant account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

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
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing tax</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{tax.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{tax.code}</p>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">{tax.description || "No description provided."}</p>
                </div>
                <Link href="/pricing/taxes" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to taxes</Link>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Stat label="Status" value={tax.status} />
              <Stat label="Rate" value={tax.rate.toFixed(2) + "%"} />
              <Stat
                label="Availability"
                value={tax.status === "active" ? "Ready to assign" : tax.status === "draft" ? "Draft" : "Archived"}
              />
              <Stat label="Scope" value="Customers / entity" />
            </section>
          </>
        )}
      </main>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm"><p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p><p className="mt-2 text-base font-semibold text-slate-950">{value}</p></div>;
}
