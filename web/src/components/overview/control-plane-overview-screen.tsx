"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Activity, Building2, LoaderCircle, ReceiptText, UserRoundPlus, Workflow } from "lucide-react";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import { fetchCustomerReadiness, fetchCustomers, fetchTenantOnboardingStatus, fetchTenants } from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function ControlPlaneOverviewScreen() {
  const { apiBaseURL, isAuthenticated, isLoading, scope, isPlatformAdmin } = useUISession();
  const scopeKey = scope === "platform" ? "platform" : "tenant";

  const tenantsQuery = useQuery({
    queryKey: ["overview-tenants", apiBaseURL],
    queryFn: () => fetchTenants({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "platform" && isPlatformAdmin,
  });

  const customersQuery = useQuery({
    queryKey: ["overview-customers", apiBaseURL],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, limit: 100 }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const tenantReadinessQueries = useQueries({
    queries: (tenantsQuery.data ?? []).map((tenant) => ({
      queryKey: ["overview-tenant-readiness", apiBaseURL, tenant.id],
      queryFn: () => fetchTenantOnboardingStatus({ runtimeBaseURL: apiBaseURL, tenantID: tenant.id }),
      enabled: isAuthenticated && scope === "platform" && isPlatformAdmin,
    })),
  });

  const customerReadinessQueries = useQueries({
    queries: (customersQuery.data ?? []).map((customer) => ({
      queryKey: ["overview-customer-readiness", apiBaseURL, customer.external_id],
      queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID: customer.external_id }),
      enabled: isAuthenticated && scope === "tenant",
    })),
  });

  const platformMetrics = useMemo(() => {
    const tenants = tenantsQuery.data ?? [];
    const readiness = tenantReadinessQueries.flatMap((query) => (query.data ? [query.data.readiness] : []));
    return {
      total: tenants.length,
      nonActive: tenants.filter((tenant) => tenant.status !== "active").length,
      missingBilling: tenants.filter(
        (tenant) => !tenant.lago_organization_id || !tenant.lago_billing_provider_code
      ).length,
      missingPricing: readiness.filter((item) => !item.billing_integration.pricing_ready).length,
      missingFirstCustomer: readiness.filter((item) => !item.first_customer.customer_exists).length,
    };
  }, [tenantReadinessQueries, tenantsQuery.data]);

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
    tenantsQuery.isLoading ||
    customersQuery.isLoading ||
    tenantReadinessQueries.some((query) => query.isLoading) ||
    customerReadinessQueries.some((query) => query.isLoading);

  const platformAttention = [
    {
      title: "Workspaces missing pricing",
      value: platformMetrics.missingPricing,
      body: "Finish pricing in workspace setup before billing starts.",
    },
    {
      title: "Workspaces missing first customer",
      value: platformMetrics.missingFirstCustomer,
      body: "Create the first billing-ready customer to complete onboarding.",
    },
    {
      title: "Workspaces missing billing connection",
      value: platformMetrics.missingBilling,
      body: "Add the billing organization and connection code before payment flows can work.",
    },
  ];

  const tenantAttention = [
    {
      title: "Customers waiting on payment setup",
      value: tenantMetrics.pendingPayment,
      body: "Finish payment setup or refresh payment state to reach verified readiness.",
    },
    {
      title: "Customers with billing sync errors",
      value: tenantMetrics.syncErrors,
      body: "Use customer diagnostics to retry billing sync and inspect the last sync error.",
    },
    {
      title: "Billing-ready customers",
      value: tenantMetrics.billingReady,
      body: "Customers that are ready for billing and payment operations.",
    },
  ];

  const guidedJourneys = [
    {
      href: "/tenant-onboarding",
      title: "Create workspace",
      description: "Create a tenant workspace, connect billing, and hand off the first admin credential.",
      icon: <Building2 className="h-5 w-5 text-sky-200" />,
      accent: "border-sky-400/40 bg-sky-500/10",
      scope: "platform" as const,
    },
    {
      href: "/customer-onboarding",
      title: "Onboard customer",
      description: "Create the first billable customer, sync the billing profile, and start payment setup.",
      icon: <UserRoundPlus className="h-5 w-5 text-teal-200" />,
      accent: "border-teal-400/40 bg-teal-500/10",
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  const workspaceModules = [
    {
      href: "/tenant-onboarding",
      title: "Workspace Setup",
      description: "Guided platform flow for workspace creation, billing connection, admin access, and readiness review.",
      icon: <Building2 className="h-5 w-5 text-sky-200" />,
      accent: "border-sky-400/40 bg-sky-500/10",
      scope: "platform" as const,
    },
    {
      href: "/customer-onboarding",
      title: "Customers",
      description: "Guided customer onboarding plus advanced billing sync and payment setup recovery when needed.",
      icon: <UserRoundPlus className="h-5 w-5 text-teal-200" />,
      accent: "border-teal-400/40 bg-teal-500/10",
      scope: "tenant" as const,
    },
    {
      href: "/payment-operations",
      title: "Payments",
      description: "Monitor invoice payment failures, inspect webhook history, and trigger payment retries.",
      icon: <Activity className="h-5 w-5 text-cyan-200" />,
      accent: "border-cyan-400/40 bg-cyan-500/10",
      scope: "tenant" as const,
    },
    {
      href: "/replay-operations",
      title: "Recovery",
      description: "Queue replay jobs, inspect diagnostics, and recover failed reprocessing runs.",
      icon: <Workflow className="h-5 w-5 text-amber-200" />,
      accent: "border-amber-400/40 bg-amber-500/10",
      scope: "tenant" as const,
    },
    {
      href: "/invoice-explainability",
      title: "Invoice Explainability",
      description: "Show deterministic line-item computation trace and digest for financial correctness workflows.",
      icon: <ReceiptText className="h-5 w-5 text-emerald-200" />,
      accent: "border-emerald-400/40 bg-emerald-500/10",
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#172554_0%,_#0f172a_38%,_#090d16_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute -left-20 top-0 h-72 w-72 rounded-full bg-cyan-500/20 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-orange-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1280px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Alpha Admin</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">
            Billing operations that feel product-first
          </h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
            Run workspace setup, customer onboarding, payment recovery, and billing diagnostics from Alpha.
            Lago stays behind the scenes as the billing engine.
          </p>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}

        <section className="grid gap-4 lg:grid-cols-[1.15fr_0.85fr]">
          <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Get Started</p>
            <h2 className="mt-2 text-2xl font-semibold text-white">Primary onboarding journeys</h2>
            <p className="mt-3 max-w-2xl text-sm text-slate-300">
              Use guided setup first. Advanced recovery and diagnostics stay available, but they should not be
              the starting point for normal onboarding.
            </p>
            <div className="mt-5 grid gap-4 md:grid-cols-2">
              {guidedJourneys.map((item) => (
                <Card
                  key={item.href}
                  href={item.href}
                  title={item.title}
                  description={item.description}
                  icon={item.icon}
                  accent={item.accent}
                />
              ))}
            </div>
          </div>

          <div className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Needs Attention</p>
            <h2 className="mt-2 text-2xl font-semibold text-white">Operational focus</h2>
            {isLoading || attentionLoading ? (
              <div className="mt-5 flex items-center gap-2 text-sm text-slate-300">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading live attention data
              </div>
            ) : !isAuthenticated ? (
              <div className="mt-5 rounded-2xl border border-dashed border-white/10 px-4 py-6 text-sm text-slate-400">
                Sign in with a tenant or platform key to load live onboarding and readiness counts.
              </div>
            ) : (
              <div className="mt-5 grid gap-3">
                {(scope === "platform" ? platformAttention : tenantAttention).map((item) => (
                  <FocusLine key={item.title} title={item.title} value={item.value} body={item.body} />
                ))}
              </div>
            )}
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {workspaceModules.map((item) => (
            <Card
              key={item.href}
              href={item.href}
              title={item.title}
              description={item.description}
              icon={item.icon}
              accent={item.accent}
            />
          ))}
        </section>
      </main>
    </div>
  );
}

function FocusLine({ title, value, body }: { title: string; value: number; body: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/50 px-4 py-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-semibold text-white">{title}</p>
          <p className="mt-1 text-sm text-slate-300">{body}</p>
        </div>
        <div className="rounded-xl border border-cyan-400/30 bg-cyan-500/10 px-3 py-2 text-lg font-semibold text-cyan-100">
          {value}
        </div>
      </div>
    </div>
  );
}

function Card({
  href,
  title,
  description,
  icon,
  accent,
}: {
  href: string;
  title: string;
  description: string;
  icon: ReactNode;
  accent: string;
}) {
  return (
    <Link
      href={href}
      className={`group rounded-2xl border p-5 transition hover:-translate-y-0.5 hover:bg-slate-900/90 ${accent}`}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-white">{title}</h2>
        <span className="inline-flex rounded-lg border border-white/20 bg-white/5 p-2">{icon}</span>
      </div>
      <p className="mt-3 text-sm leading-6 text-slate-200">{description}</p>
      <p className="mt-4 text-xs font-semibold uppercase tracking-[0.16em] text-slate-300 group-hover:text-white">
        Open
      </p>
    </Link>
  );
}
