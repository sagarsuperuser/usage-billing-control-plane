"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchAddOn } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingAddOnDetailScreen({ addOnID }: { addOnID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const addOnQuery = useQuery({
    queryKey: ["pricing-add-on", apiBaseURL, addOnID],
    queryFn: () => fetchAddOn({ runtimeBaseURL: apiBaseURL, addOnID }),
    enabled: isTenantSession && addOnID.trim().length > 0,
  });

  const addOn = addOnQuery.data ?? null;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/add-ons", label: "Add-ons" }, { label: addOn?.name || addOnID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Add-ons are workspace-scoped. Sign in with a workspace account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {isTenantSession ? addOnQuery.isLoading ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm"><div className="flex items-center gap-2"><LoaderCircle className="h-4 w-4 animate-spin" />Loading add-on detail</div></section>
        ) : !addOn ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing add-on</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Add-on not available</h1>
            <Link href="/pricing/add-ons" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to add-ons</Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace add-on</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{addOn.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{addOn.code}</p>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">{addOn.description || "No description provided."}</p>
                </div>
                <Link href="/pricing/add-ons" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to add-ons</Link>
              </div>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <div className="grid gap-5">
                <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <Stat label="Status" value={addOn.status} />
                  <Stat label="Interval" value={addOn.billing_interval} />
                  <Stat label="Recurring amount" value={`${(addOn.amount_cents / 100).toFixed(2)} ${addOn.currency}`} />
                  <Stat label="Currency" value={addOn.currency} />
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial rule</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Recurring add-on terms</h2>
                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    <InfoCell label="Name" value={addOn.name} />
                    <InfoCell label="Code" value={addOn.code} mono />
                    <InfoCell label="Billing interval" value={addOn.billing_interval} />
                    <InfoCell label="Commercial use" value="Reusable recurring extra" />
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <GuidanceCard title="When to use add-ons" body="Use add-ons for fixed recurring extras you can clearly explain to customers on top of the base plan." />
                <GuidanceCard title="Next action" body="Attach this record to plans that need the extra charge. Use plan detail to confirm where it is currently used." />
              </aside>
            </div>
          </SectionErrorBoundary>
        ) : null}
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
