"use client";

import Link from "next/link";
import { ArrowLeft, LoaderCircle } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchCoupon } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

function statusBadgeClass(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "draft":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "archived":
      return "border-slate-200 bg-slate-100 text-slate-500";
    default:
      return "border-slate-200 bg-slate-50 text-slate-600";
  }
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

export function PricingCouponDetailScreen({ couponID }: { couponID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const couponQuery = useQuery({
    queryKey: ["pricing-coupon", apiBaseURL, couponID],
    queryFn: () => fetchCoupon({ runtimeBaseURL: apiBaseURL, couponID }),
    enabled: isTenantSession && couponID.trim().length > 0,
  });

  const coupon = couponQuery.data ?? null;

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: coupon?.name || couponID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}


        {isTenantSession ? couponQuery.isLoading ? (
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <div className="flex items-center gap-2 text-sm text-slate-500">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              Loading coupon detail
            </div>
          </section>
        ) : !coupon ? (
          <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
            <p className="text-sm font-semibold text-slate-900">Coupon not available</p>
            <p className="mt-1 text-sm text-slate-500">The requested coupon could not be loaded.</p>
            <Link href="/pricing/coupons" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to coupons
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-slate-200 bg-white shadow-sm divide-y divide-slate-200">
              {/* Header */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-slate-900 truncate">{coupon.name}</h1>
                    <span className={`shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide ${statusBadgeClass(coupon.status)}`}>
                      {coupon.status}
                    </span>
                  </div>
                  <Link href="/pricing/coupons" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-slate-200 bg-white px-3 text-xs font-medium text-slate-700 transition hover:bg-slate-50">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to coupons
                  </Link>
                </div>
                {coupon.description ? <p className="mt-1.5 text-xs text-slate-500">{coupon.description}</p> : null}
              </div>

              {/* Details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-slate-400">Code</dt>
                    <dd className="mt-0.5 text-sm text-slate-700 font-mono">{coupon.code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Discount type</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{coupon.discount_type}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Value</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">
                      {coupon.discount_type === "percent_off"
                        ? `${coupon.percent_off}% off`
                        : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Currency</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{coupon.currency || "N/A"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Frequency</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{renderFrequency(coupon)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-slate-400">Expiration</dt>
                    <dd className="mt-0.5 text-sm text-slate-700">{coupon.expiration_at ? new Date(coupon.expiration_at).toLocaleString() : "No expiration"}</dd>
                  </div>
                </dl>
              </div>
            </div>
          </SectionErrorBoundary>
        ) : null}
      </main>
    </div>
  );
}
