
import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import { useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { StatusChip } from "@/components/ui/status-chip";
import { fetchTax } from "@/lib/api";
import { statusTone } from "@/lib/badge";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingTaxDetailScreen({ taxID }: { taxID: string }) {
  const { apiBaseURL, isAuthenticated, scope } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";

  const taxQuery = useQuery({
    queryKey: ["pricing-tax", apiBaseURL, taxID],
    queryFn: () => fetchTax({ runtimeBaseURL: apiBaseURL, taxID }),
    enabled: isTenantSession && taxID.trim().length > 0,
  });

  const tax = taxQuery.data ?? null;

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/taxes", label: "Taxes" }, { label: tax?.name || taxID }]} />

        {!isAuthenticated ? <LoginRedirectNotice /> : null}


        {isTenantSession ? taxQuery.isLoading ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <div className="animate-pulse space-y-3">
              <div className="h-6 w-48 rounded bg-surface-secondary" />
              <div className="h-4 w-72 rounded bg-surface-secondary" />
              <div className="h-32 w-full rounded bg-surface-secondary" />
            </div>
          </section>
        ) : !tax ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <p className="text-sm font-semibold text-text-primary">Tax not available</p>
            <p className="mt-1 text-sm text-text-muted">The requested tax could not be loaded.</p>
            <Link to="/pricing/taxes" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to taxes
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-border bg-surface shadow-sm divide-y divide-slate-200">
              {/* Header */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-text-primary truncate">{tax.name}</h1>
                    <StatusChip tone={statusTone(tax.status)}>{tax.status}</StatusChip>
                  </div>
                  <Link to="/pricing/taxes" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                    <ArrowLeft className="h-3.5 w-3.5" />
                    Back to taxes
                  </Link>
                </div>
                {tax.description ? <p className="mt-1.5 text-xs text-text-muted">{tax.description}</p> : null}
              </div>

              {/* Details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-text-faint">Code</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary font-mono">{tax.code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Rate</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{tax.rate.toFixed(2)}%</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Status</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{tax.status}</dd>
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
