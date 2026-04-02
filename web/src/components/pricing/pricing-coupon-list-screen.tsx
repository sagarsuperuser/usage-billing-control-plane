"use client";

import Link from "next/link";
import { ChevronRight, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { Skeleton } from "@/components/ui/skeleton";
import { fetchCoupons } from "@/lib/api";
import { type Coupon } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingCouponListScreen() {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const [search, setSearch] = useState("");

  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const filtered = useMemo(() => {
    const items = couponsQuery.data ?? [];
    const term = search.trim().toLowerCase();
    if (!term) return items;
    return items.filter((item) => item.code.toLowerCase().includes(term) || item.name.toLowerCase().includes(term));
  }, [couponsQuery.data, search]);

  const draftCount = (couponsQuery.data ?? []).filter((item) => item.status === "draft").length;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { label: "Coupons" }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice title="Workspace session required" body="Coupons are workspace-scoped. Sign in with a workspace account to manage them." actionHref="/billing-connections" actionLabel="Open platform home" />
        ) : null}

        {isTenantSession ? (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace pricing console</p>
                  <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Coupons</h1>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">
                    Model amount-off and percent-off commercial relief for launches, promotions, and negotiated offers.
                  </p>
                </div>
                <Link href="/pricing/coupons/new" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-900 bg-slate-900 px-4 text-sm font-medium text-white transition hover:bg-slate-800">
                  <Plus className="h-4 w-4" />
                  New coupon
                </Link>
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-3">
              <MetricCard label="Total coupons" value={String(couponsQuery.data?.length ?? 0)} />
              <MetricCard label="Draft coupons" value={String(draftCount)} />
              <MetricCard label="Search results" value={String(filtered.length)} />
            </section>

            <section className="grid gap-3 xl:grid-cols-3">
              <OperatorCard title="Commercial rule" body="Coupons are discount rules, not one-off adjustments. Keep their scope and expiry easy to explain." />
              <OperatorCard title="Inventory rule" body="Review discount type, runtime frequency, and expiration from the list before attaching a rule to active commercial packages." />
              <OperatorCard title="Next action" body="Open coupon detail to confirm commercial posture, then apply it through plans or customer follow-up." />
            </section>

            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Coupon inventory</p>
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
                {couponsQuery.isLoading ? <LoadingState /> : filtered.length === 0 ? <EmptyState /> : filtered.map((coupon) => <CouponRow key={coupon.id} coupon={coupon} />)}
              </div>
            </section>
          </>
        ) : null}
      </main>
    </div>
  );
}

function CouponRow({ coupon }: { coupon: Coupon }) {
  return (
    <Link href={`/pricing/coupons/${encodeURIComponent(coupon.id)}`} className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 transition hover:border-slate-300 hover:bg-slate-100 lg:grid-cols-[minmax(0,1.1fr)_repeat(6,minmax(0,0.55fr))_auto] lg:items-center">
      <div className="min-w-0">
        <h3 className="truncate text-base font-semibold text-slate-950">{coupon.name}</h3>
        <p className="mt-1 break-all font-mono text-xs text-slate-500">{coupon.code}</p>
        <p className="mt-2 text-sm text-slate-600">{coupon.description || "No description provided."}</p>
      </div>
      <StatusCell label="Status" value={coupon.status} />
      <StatusCell label="Type" value={coupon.discount_type} />
      <StatusCell label="Value" value={coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`} />
      <StatusCell label="Currency" value={coupon.currency?.toUpperCase() || "N/A"} />
      <StatusCell label="Frequency" value={renderFrequency(coupon)} />
      <StatusCell label="Expires" value={coupon.expiration_at ? "Timed" : "Ongoing"} />
      <StatusCell label="Scope" value="Plan" />
      <span className="inline-flex items-center gap-2 text-sm font-medium text-slate-700">Open<ChevronRight className="h-4 w-4" /></span>
    </Link>
  );
}

function renderFrequency(coupon: Coupon) {
  switch (coupon.frequency) {
    case "once":
      return "Once";
    case "recurring":
      return `${coupon.frequency_duration} periods`;
    default:
      return "Forever";
  }
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
  return (
    <div className="grid gap-3">
      {Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="grid gap-4 rounded-xl border border-slate-200 bg-slate-50 p-4 lg:grid-cols-[minmax(0,1.1fr)_repeat(6,minmax(0,0.55fr))_auto] lg:items-center">
          <div className="min-w-0">
            <Skeleton className="h-5 w-36" />
            <Skeleton className="mt-2 h-3 w-24" />
            <Skeleton className="mt-2 h-4 w-48" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-10" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-8" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-10" />
            <Skeleton className="mt-2 h-4 w-20" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-12" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-12" />
            <Skeleton className="mt-2 h-4 w-16" />
          </div>
          <div className="rounded-lg border border-slate-200 bg-white px-4 py-3">
            <Skeleton className="h-3 w-8" />
            <Skeleton className="mt-2 h-4 w-12" />
          </div>
          <Skeleton className="h-4 w-10" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 px-5 py-8 text-sm text-slate-600"><p className="font-semibold text-slate-950">No coupons yet.</p><p className="mt-2">Create the first coupon, then attach it to plans.</p><div className="mt-4"><Link href="/pricing/coupons/new" className="inline-flex h-9 items-center rounded-lg border border-slate-900 bg-slate-900 px-4 text-xs font-semibold uppercase tracking-[0.14em] text-white transition hover:bg-slate-800">Create coupon</Link></div></div>;
}
