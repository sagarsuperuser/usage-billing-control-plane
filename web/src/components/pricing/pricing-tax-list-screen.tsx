"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchTaxes } from "@/lib/api";
import { type Tax } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingTaxListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [search, setSearch] = useState("");

  const taxesQuery = useQuery({
    queryKey: ["pricing-taxes", apiBaseURL],
    queryFn: () => fetchTaxes({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filtered = useMemo(() => {
    const items = taxesQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term));
  }, [taxesQuery.data, search]);

  const activeCount = (taxesQuery.data ?? []).filter((item) => item.status === "active").length;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Taxes" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Taxes</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Maintain reusable tax codes and rates, then assign them to customer billing profiles and workspace billing settings.
              </p>
            </div>
            <Link href="/pricing/taxes/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
              <Plus className="h-4 w-4" />
              New tax
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Workspace session required" body="Taxes are workspace-scoped. Sign in with a workspace account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        <section className="grid gap-4 md:grid-cols-3">
          <MetricCard label="Total taxes" value={String(taxesQuery.data?.length ?? 0)} />
          <MetricCard label="Active taxes" value={String(activeCount)} />
          <MetricCard label="Search results" value={String(filtered.length)} />
        </section>

        <section className="grid gap-3 xl:grid-cols-3">
          <OperatorCard title="Assignment rule" body="Keep tax codes stable and rates deliberate so invoice behavior stays explainable." />
          <OperatorCard title="Inventory rule" body="This list is for reusable tax rules. Use it to verify readiness before applying a rule to customer or workspace billing settings." />
          <OperatorCard title="Next action" body="Open tax detail to confirm availability, then assign active rules through billing settings." />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Tax inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by name or code"
              className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
          <div className="mt-5 grid gap-3">
            {taxesQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((tax) => <TaxRow key={tax.id} tax={tax} />)}
          </div>
        </section>
      </main>
    </div>
  );
}

function TaxRow({ tax }: { tax: Tax }) {
  return (
    <Link href={"/pricing/taxes/" + encodeURIComponent(tax.id)} className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(4,minmax(0,0.6fr))_auto] lg:items-center">
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{tax.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{tax.code}</p>
        <p className="mt-2 text-sm text-slate-600">{tax.description || "No description provided."}</p>
      </div>
      <StatusCell label="Status" value={tax.status} />
      <StatusCell label="Rate" value={tax.rate.toFixed(2) + "%"} />
      <StatusCell label="Applies to" value="Customers / entity" />
      <StatusCell
        label="Availability"
        value={
          tax.status === "active" ? "Ready to assign" : tax.status === "draft" ? "Draft" : "Archived"
        }
      />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-slate-700">Open<ChevronRight className="h-4 w-4" /></span>
    </Link>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return <div className="rounded-2xl border border-slate-200 bg-white px-4 py-4 shadow-sm"><p className="text-[11px] font-semibold uppercase tracking-[0.15em] text-slate-500">{label}</p><p className="mt-2 text-base font-semibold text-slate-950">{value}</p></div>;
}

function StatusCell({ label, value }: { label: string; value: string }) {
  return <div className="rounded-lg border border-slate-200 bg-white px-4 py-3"><p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p><p className="mt-2 text-sm font-semibold text-slate-950">{value}</p></div>;
}

function OperatorCard({ title, body }: { title: string; body: string }) {
  return <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><p className="text-sm font-semibold text-slate-950">{title}</p><p className="mt-2 text-sm leading-relaxed text-slate-600">{body}</p></section>;
}

function LoadingState() {
  return <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600"><LoaderCircle className="h-4 w-4 animate-spin" />Loading tax inventory</div>;
}

function EmptyState() {
  return <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600"><p className="font-semibold text-slate-950">No taxes yet.</p><p className="mt-2">Create the first tax, then assign it to customer profiles or workspace billing settings.</p><div className="mt-4"><Link href="/pricing/taxes/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">Create tax</Link></div></div>;
}
