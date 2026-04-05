
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { fetchPricingMetric } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingMetricDetailScreen({ metricID }: { metricID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const query = useQuery({
    queryKey: ["pricing-metric", apiBaseURL, metricID],
    queryFn: () => fetchPricingMetric({ runtimeBaseURL: apiBaseURL, metricID }),
    enabled: isTenantSession && metricID.trim().length > 0,
  });

  const metric = query.data ?? null;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/metrics", label: "Metrics" }, { label: metric?.name || metricID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        {isTenantSession ? query.isLoading ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <div className="animate-pulse space-y-3">
              <div className="h-6 w-48 rounded bg-surface-secondary" />
              <div className="h-4 w-72 rounded bg-surface-secondary" />
              <div className="h-32 w-full rounded bg-surface-secondary" />
            </div>
          </section>
        ) : !metric ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <p className="text-sm font-semibold text-text-primary">Metric not available</p>
            <p className="mt-1 text-sm text-text-muted">The requested pricing metric could not be loaded.</p>
            <Link to="/pricing/metrics" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to metrics
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-border bg-surface shadow-sm divide-y divide-slate-200">
              {/* Header */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-text-primary truncate">{metric.name}</h1>
                    <span className="shrink-0 rounded-full border border-border bg-surface-secondary px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-text-muted">
                      {metric.key}
                    </span>
                  </div>
                  <Link to="/pricing/metrics" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to metrics
                  </Link>
                </div>
              </div>

              {/* Details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-text-faint">Unit</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{metric.unit}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Aggregation</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{metric.aggregation}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Rating rule</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{metric.key}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Created</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{new Date(metric.created_at).toLocaleString()}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Updated</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{new Date(metric.updated_at).toLocaleString()}</dd>
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
