
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { StatusChip } from "@/components/ui/status-chip";
import { fetchCoupon } from "@/lib/api";
import { statusTone } from "@/lib/badge";
import { useUISession } from "@/hooks/use-ui-session";

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
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/coupons", label: "Coupons" }, { label: coupon?.name || couponID }]} />



        {isTenantSession ? couponQuery.isLoading ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <div className="animate-pulse space-y-3">
              <div className="h-6 w-48 rounded bg-surface-secondary" />
              <div className="h-4 w-72 rounded bg-surface-secondary" />
              <div className="h-32 w-full rounded bg-surface-secondary" />
            </div>
          </section>
        ) : !coupon ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <p className="text-sm font-semibold text-text-primary">Coupon not available</p>
            <p className="mt-1 text-sm text-text-muted">The requested coupon could not be loaded.</p>
            <Link to="/pricing/coupons" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to coupons
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-border bg-surface shadow-sm divide-y divide-slate-200">
              {/* Header */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-text-primary truncate">{coupon.name}</h1>
                    <StatusChip tone={statusTone(coupon.status)}>{coupon.status}</StatusChip>
                  </div>
                  <Link to="/pricing/coupons" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to coupons
                  </Link>
                </div>
                {coupon.description ? <p className="mt-1.5 text-xs text-text-muted">{coupon.description}</p> : null}
              </div>

              {/* Details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-text-faint">Code</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary font-mono">{coupon.code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Discount type</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{coupon.discount_type}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Value</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">
                      {coupon.discount_type === "percent_off"
                        ? `${coupon.percent_off}% off`
                        : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Currency</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{coupon.currency || "N/A"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Frequency</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{renderFrequency(coupon)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Expiration</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{coupon.expiration_at ? new Date(coupon.expiration_at).toLocaleString() : "No expiration"}</dd>
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
