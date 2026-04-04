
import { Link } from "@tanstack/react-router";
import { useMemo } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { Skeleton } from "@/components/ui/skeleton";
import {
  fetchCustomerReadiness,
  fetchCustomers,
} from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function ControlPlaneOverviewScreen() {
  const { apiBaseURL, isAuthenticated, isLoading } = useUISession();

  const customersQuery = useQuery({
    queryKey: ["overview-customers", apiBaseURL],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isAuthenticated,
  });

  const customerReadinessQueries = useQueries({
    queries: (customersQuery.data ?? []).map((customer) => ({
      queryKey: ["overview-customer-readiness", apiBaseURL, customer.external_id],
      queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID: customer.external_id }),
      enabled: isAuthenticated,
    })),
  });

  const tenantMetrics = useMemo(() => {
    const customers = customersQuery.data ?? [];
    const readiness = customerReadinessQueries.flatMap((query) => (query.data ? [query.data] : []));
    return {
      total: customers.length,
      synced: customers.filter((customer) => Boolean(customer.lago_customer_id)).length,
      pendingPayment: readiness.filter((item) => item.payment_setup_status !== "ready").length,
      syncErrors: readiness.filter((item) => item.billing_profile_status === "sync_error").length,
      billingReady: readiness.filter((item) => item.status === "ready").length,
    };
  }, [customerReadinessQueries, customersQuery.data]);

  const attentionLoading =
    customersQuery.isLoading ||
    customerReadinessQueries.some((query) => query.isLoading);

  const summaryItems = [
    { label: "Customers", value: tenantMetrics.total, tone: "default" as const },
    { label: "Billing-ready", value: tenantMetrics.billingReady, tone: "success" as const },
    { label: "Errors", value: tenantMetrics.syncErrors, tone: "danger" as const },
  ];

  const attentionItems = [
    { title: "Customers waiting on payment setup", value: tenantMetrics.pendingPayment, href: "/subscriptions" },
    { title: "Customers with billing sync errors", value: tenantMetrics.syncErrors, href: "/payments" },
    { title: "Billing-ready customers", value: tenantMetrics.billingReady, href: "/customers" },
  ];

  const hasAttention = attentionItems.some((item) => item.value > 0);

  return (
    <div className="text-slate-900">
      <main className="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <LoginRedirectNotice />

        <div className="overflow-hidden rounded-lg border border-stone-200 bg-white shadow-sm">
          {/* Header */}
          <div className="border-b border-stone-200 px-5 py-3">
            <h1 className="text-sm font-semibold text-slate-900">Overview</h1>
          </div>

          {/* Compact summary bar */}
          <div className="flex items-center gap-6 border-b border-stone-200 px-5 py-3">
            {isLoading || attentionLoading ? (
              Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Skeleton className="h-3 w-16" />
                  <Skeleton className="h-5 w-8" />
                </div>
              ))
            ) : (
              summaryItems.map((item) => (
                <div key={item.label} className="flex items-center gap-2 text-sm">
                  <span className="text-slate-500">{item.label}</span>
                  <span className={`font-semibold ${item.tone === "success" ? "text-emerald-700" : item.tone === "danger" ? "text-rose-700" : "text-slate-900"}`}>
                    {item.value}
                  </span>
                </div>
              ))
            )}
          </div>

          {/* Needs attention section */}
          {!attentionLoading && hasAttention ? (
            <div className="border-b border-stone-200 px-5 py-4">
              <h2 className="text-sm font-semibold text-slate-900">Needs attention</h2>
              <div className="mt-3 divide-y divide-stone-100">
                {attentionItems.filter((item) => item.value > 0).map((item) => (
                  <Link
                    key={item.title}
                    to={item.href}
                    className="flex items-center justify-between py-2 text-sm transition hover:bg-stone-50"
                  >
                    <span className="text-slate-700">
                      <span className="font-medium text-slate-900">{item.value}</span>{" "}
                      {item.title.toLowerCase()}
                    </span>
                    <span className="text-xs text-slate-400">View →</span>
                  </Link>
                ))}
              </div>
            </div>
          ) : null}

          {/* Getting started -- only when 0 customers */}
          {!attentionLoading && tenantMetrics.total === 0 ? (
            <div className="px-5 py-4">
              <h2 className="text-sm font-semibold text-slate-900">Getting started</h2>
              <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <GettingStartedStep step="1" title="Create a metric" description="Define what you'll charge for." href="/pricing/metrics/new" />
                <GettingStartedStep step="2" title="Create a plan" description="Package the metric with a price." href="/pricing/plans/new" />
                <GettingStartedStep step="3" title="Add a customer" description="Create a customer with billing profile." href="/customers/new" />
                <GettingStartedStep step="4" title="Create a subscription" description="Attach the customer to the plan." href="/subscriptions/new" />
              </div>
            </div>
          ) : null}
        </div>
      </main>
    </div>
  );
}

function GettingStartedStep({ step, title, description, href }: { step: string; title: string; description: string; href: string }) {
  return (
    <Link to={href} className="group flex flex-col gap-2 rounded-lg border border-stone-200 bg-stone-50 p-3 transition hover:border-slate-300 hover:bg-white">
      <span className="inline-flex h-6 w-6 items-center justify-center rounded-full bg-slate-900 text-[10px] font-semibold text-white">{step}</span>
      <div>
        <p className="text-sm font-medium text-slate-900">{title}</p>
        <p className="mt-0.5 text-xs text-slate-500">{description}</p>
      </div>
      <span className="mt-auto text-xs text-slate-400 transition group-hover:text-slate-700">Start →</span>
    </Link>
  );
}
