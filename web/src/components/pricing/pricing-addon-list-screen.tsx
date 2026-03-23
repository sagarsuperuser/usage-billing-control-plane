"use client";

import Link from "next/link";
import { ChevronRight, LoaderCircle, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchAddOns } from "@/lib/api";
import { type AddOn } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingAddOnListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const [search, setSearch] = useState("");

  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const filtered = useMemo(() => {
    const items = addOnsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term) || item.currency.toLowerCase().includes(term));
  }, [addOnsQuery.data, search]);

  const draftCount = (addOnsQuery.data ?? []).filter((item) => item.status === "draft").length;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Add-ons" }]} />

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Add-ons</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-600">
                Create reusable recurring extras like premium support, onboarding, or compliance bundles and attach them to plans.
              </p>
            </div>
            <Link href="/pricing/add-ons/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
              <Plus className="h-4 w-4" />
              New add-on
            </Link>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Tenant session required" body="Add-ons are tenant-scoped. Sign in with a tenant account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        <section className="grid gap-4 md:grid-cols-3">
          <MetricCard label="Total add-ons" value={String(addOnsQuery.data?.length ?? 0)} />
          <MetricCard label="Draft add-ons" value={String(draftCount)} />
          <MetricCard label="Search results" value={String(filtered.length)} />
        </section>

        <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Add-on inventory</p>
              <h2 className="mt-2 text-xl font-semibold text-slate-950">Browse and inspect</h2>
            </div>
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search by name, code, or currency"
              className="h-10 min-w-[260px] rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-900 outline-none ring-slate-400 transition placeholder:text-slate-400 focus:ring-2"
            />
          </div>
          <div className="mt-5 grid gap-3">
            {addOnsQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((addOn) => <AddOnRow key={addOn.id} addOn={addOn} />)}
          </div>
        </section>
      </main>
    </div>
  );
}

function AddOnRow({ addOn }: { addOn: AddOn }) {
  return (
    <Link href={`/pricing/add-ons/${encodeURIComponent(addOn.id)}`} className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(5,minmax(0,0.55fr))_auto] lg:items-center">
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{addOn.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{addOn.code}</p>
        <p className="mt-2 text-sm text-slate-600">{addOn.description || "No description provided."}</p>
      </div>
      <StatusCell label="Status" value={addOn.status} />
      <StatusCell label="Interval" value={addOn.billing_interval} />
      <StatusCell label="Amount" value={`${(addOn.amount_cents / 100).toFixed(2)} ${addOn.currency}`} />
      <StatusCell label="Currency" value={addOn.currency.toUpperCase()} />
      <StatusCell label="Type" value="Recurring" />
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

function LoadingState() {
  return <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-6 text-sm text-slate-600"><LoaderCircle className="h-4 w-4 animate-spin" />Loading add-on inventory</div>;
}

function EmptyState() {
  return <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600"><p className="font-semibold text-slate-950">No add-ons yet.</p><p className="mt-2">Create the first add-on, then attach it to plans.</p><div className="mt-4"><Link href="/pricing/add-ons/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">Create add-on</Link></div></div>;
}
