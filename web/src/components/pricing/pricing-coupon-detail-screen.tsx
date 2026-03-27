"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ScopeNotice } from "@/components/auth/scope-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchCoupon } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingCouponDetailScreen({ couponID }: { couponID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();

  const couponQuery = useQuery({
    queryKey: ["pricing-coupon", apiBaseURL, couponID],
    queryFn: () => fetchCoupon({ runtimeBaseURL: apiBaseURL, couponID }),
    enabled: isAuthenticated && scope === "tenant" && couponID.trim().length > 0,
  });

  const coupon = couponQuery.data ?? null;

  return (
    <div className="min-h-screen bg-[#f5f7fb] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <ControlPlaneNav />
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: coupon?.name || couponID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}
        {isAuthenticated && scope !== "tenant" ? <ScopeNotice title="Workspace session required" body="Coupons are workspace-scoped. Sign in with a workspace account to inspect them." actionHref="/billing-connections" actionLabel="Open platform home" /> : null}

        {couponQuery.isLoading ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 text-sm text-slate-600 shadow-sm"><div className="flex items-center gap-2"><LoaderCircle className="h-4 w-4 animate-spin" />Loading coupon detail</div></section>
        ) : !coupon ? (
          <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Pricing coupon</p>
            <h1 className="mt-2 text-2xl font-semibold text-slate-950">Coupon not available</h1>
            <Link href="/pricing/coupons" className="mt-5 inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to coupons</Link>
          </section>
        ) : (
          <>
            <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
              <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Workspace coupon</p>
                  <h1 className="mt-2 break-words text-3xl font-semibold tracking-tight text-slate-950">{coupon.name}</h1>
                  <p className="mt-3 break-all font-mono text-xs text-slate-500">{coupon.code}</p>
                  <p className="mt-3 max-w-3xl text-sm text-slate-600">{coupon.description || "No description provided."}</p>
                </div>
                <Link href="/pricing/coupons" className="inline-flex h-10 items-center gap-2 rounded-lg border border-slate-200 bg-slate-50 px-4 text-sm text-slate-700 transition hover:bg-slate-100"><ArrowLeft className="h-4 w-4" />Back to coupons</Link>
              </div>
            </section>

            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
              <div className="grid gap-5">
                <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  <Stat label="Status" value={coupon.status} />
                  <Stat label="Type" value={coupon.discount_type} />
                  <Stat label="Value" value={coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`} />
                  <Stat label="Currency" value={coupon.currency || "N/A"} />
                  <Stat label="Frequency" value={renderFrequency(coupon)} />
                  <Stat label="Expires" value={coupon.expiration_at ? new Date(coupon.expiration_at).toLocaleString() : "No expiration"} />
                </section>

                <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm">
                  <p className="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">Commercial rule</p>
                  <h2 className="mt-2 text-xl font-semibold text-slate-950">Discount behavior</h2>
                  <div className="mt-5 grid gap-3 md:grid-cols-2">
                    <InfoCell label="Coupon code" value={coupon.code} mono />
                    <InfoCell label="Discount type" value={coupon.discount_type} />
                    <InfoCell label="Commercial use" value="Applied through plans or customer follow-up" />
                    <InfoCell label="Expiry model" value={coupon.expiration_at ? "Time-bound" : "Open-ended unless frequency stops it"} />
                  </div>
                </section>
              </div>

              <aside className="grid gap-5 self-start">
                <GuidanceCard title="Operator posture" body="Coupons are commercial relief rules. Keep their shape and expiry easy to explain before they are attached to plans or customer subscriptions." />
                <GuidanceCard title="Next action" body="Use plan detail or subscription follow-up to confirm where this coupon is applied and when the relief should end." />
              </aside>
            </div>
          </>
        )}
      </main>
    </div>
  );
}

function renderFrequency(coupon: { frequency: "once" | "recurring" | "forever"; frequency_duration: number }) {
  switch (coupon.frequency) {
    case "once":
      return "Once";
    case "recurring":
      return `${coupon.frequency_duration} billing periods`;
    default:
      return "Forever";
  }
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
