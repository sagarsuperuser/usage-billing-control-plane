"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import {
  Activity,
  ArrowRight,
  Building2,
  CreditCard,
  LoaderCircle,
  ReceiptText,
  ShieldCheck,
  UserRoundPlus,
  Workflow,
} from "lucide-react";

import { LoginRedirectNotice } from "@/components/auth/login-redirect-notice";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  fetchBillingProviderConnections,
  fetchCustomerReadiness,
  fetchCustomers,
  fetchTenantOnboardingStatus,
  fetchTenants,
} from "@/lib/api";
import { useUISession } from "@/hooks/use-ui-session";

export function ControlPlaneOverviewScreen() {
  const { apiBaseURL, isAuthenticated, isLoading, scope, isPlatformAdmin, session, platformRole } = useUISession();
  const scopeKey = scope === "platform" ? "platform" : "tenant";

  const tenantsQuery = useQuery({
    queryKey: ["overview-tenants", apiBaseURL],
    queryFn: () => fetchTenants({ runtimeBaseURL: apiBaseURL }),
    enabled: isAuthenticated && scope === "platform" && isPlatformAdmin,
  });

  const billingConnectionsQuery = useQuery({
    queryKey: ["overview-billing-provider-connections", apiBaseURL],
    queryFn: () => fetchBillingProviderConnections({ runtimeBaseURL: apiBaseURL, limit: 100 }),
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
    const connections = billingConnectionsQuery.data ?? [];
    return {
      total: tenants.length,
      missingBilling: tenants.filter((tenant) => !tenant.workspace_billing.connected).length,
      missingPricing: readiness.filter((item) => !item.billing_integration.pricing_ready).length,
      missingFirstCustomer: readiness.filter((item) => !item.first_customer.customer_exists).length,
      connectedProviders: connections.filter((item) => item.status === "connected").length,
      providerErrors: connections.filter((item) => item.status === "sync_error").length,
    };
  }, [billingConnectionsQuery.data, tenantReadinessQueries, tenantsQuery.data]);

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
    billingConnectionsQuery.isLoading ||
    customersQuery.isLoading ||
    tenantReadinessQueries.some((query) => query.isLoading) ||
    customerReadinessQueries.some((query) => query.isLoading);

  const platformAttention = [
    {
      title: "Billing connection errors",
      value: platformMetrics.providerErrors,
      body: "Fix sync issues before reusing those credentials across more workspaces.",
      href: "/billing-connections",
    },
    {
      title: "Workspaces missing pricing",
      value: platformMetrics.missingPricing,
      body: "Pricing remains the most common blocker before billing can run cleanly.",
      href: "/workspaces",
    },
    {
      title: "Workspaces missing first customer",
      value: platformMetrics.missingFirstCustomer,
      body: "A workspace is still not operational until a billable customer exists inside it.",
      href: "/workspaces",
    },
  ];

  const tenantAttention = [
    {
      title: "Customers waiting on payment setup",
      value: tenantMetrics.pendingPayment,
      body: "Push payer-owned payment setup to completion before marking accounts active.",
      href: "/subscriptions",
    },
    {
      title: "Customers with billing sync errors",
      value: tenantMetrics.syncErrors,
      body: "Use diagnostics and replay paths to repair broken billing state before retrying collection.",
      href: "/payment-operations",
    },
    {
      title: "Billing-ready customers",
      value: tenantMetrics.billingReady,
      body: "These customers are healthy enough for subscriptions, invoices, and downstream operations.",
      href: "/customers",
    },
  ];

  const guidedJourneys = [
    {
      href: "/billing-connections/new",
      title: "Create billing connection",
      description: "Store Stripe secrets in Alpha, sync the provider into Lago, and keep credential ownership centralized.",
      icon: <CreditCard className="h-5 w-5 text-emerald-700" />,
      eyebrow: "Platform setup",
      scope: "platform" as const,
    },
    {
      href: "/workspaces/new",
      title: "Launch workspace",
      description: "Create a workspace, attach one active billing path, and hand off the first workspace admin cleanly.",
      icon: <Building2 className="h-5 w-5 text-emerald-700" />,
      eyebrow: "Platform rollout",
      scope: "platform" as const,
    },
    {
      href: "/customers/new",
      title: "Create first customer",
      description: "Open the workspace, create the first billable customer, and start subscription and payment setup.",
      icon: <UserRoundPlus className="h-5 w-5 text-emerald-700" />,
      eyebrow: "Workspace onboarding",
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  const workspaceModules = [
    {
      href: "/billing-connections",
      title: "Billing Connections",
      description: "Provider credentials, sync health, and workspace reuse live here.",
      icon: <CreditCard className="h-5 w-5 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/workspaces",
      title: "Workspaces",
      description: "See which workspaces are launched, blocked, or missing operational setup.",
      icon: <Building2 className="h-5 w-5 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/customers",
      title: "Customers",
      description: "Track readiness, sync health, and payment setup progression inside the workspace.",
      icon: <UserRoundPlus className="h-5 w-5 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/subscriptions",
      title: "Subscriptions",
      description: "Own activation state and payer-owned payment setup as one operating flow.",
      icon: <ShieldCheck className="h-5 w-5 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/payment-operations",
      title: "Payments",
      description: "Investigate failed collections, retries, and operational follow-up.",
      icon: <Activity className="h-5 w-5 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/invoice-explainability",
      title: "Explainability",
      description: "Show how invoice outcomes were computed when finance or support needs evidence.",
      icon: <ReceiptText className="h-5 w-5 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/replay-operations",
      title: "Recovery",
      description: "Repair failed processing runs without dropping to backend-only operational tooling.",
      icon: <Workflow className="h-5 w-5 text-emerald-700" />,
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  const sessionHeadline =
    scope === "platform"
      ? "Platform operating surface"
      : `Workspace operating surface${session?.tenant_id ? ` · ${session.tenant_id}` : ""}`;
  const sessionSubcopy =
    scope === "platform"
      ? "Use Alpha to manage reusable billing credentials, launch workspaces, and hand ownership off without exposing Lago internals."
      : "Use Alpha to run pricing, customer onboarding, subscriptions, and payment operations inside one workspace boundary.";

  const heroMetrics =
    scope === "platform"
      ? [
          { label: "Workspaces", value: platformMetrics.total, detail: "Active workspace directory" },
          { label: "Connected credentials", value: platformMetrics.connectedProviders, detail: "Healthy billing connections" },
          { label: "Missing billing", value: platformMetrics.missingBilling, detail: "Workspaces not yet attached" },
        ]
      : [
          { label: "Customers", value: tenantMetrics.total, detail: "Accounts visible in this workspace" },
          { label: "Billing-ready", value: tenantMetrics.billingReady, detail: "Customers ready for operations" },
          { label: "Pending payment setup", value: tenantMetrics.pendingPayment, detail: "Action still required" },
        ];

  const primaryAction = guidedJourneys[0] ?? workspaceModules[0] ?? null;
  const attentionCards = scope === "platform" ? platformAttention : tenantAttention;

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#dfeee3_0%,#f5efe4_18%,#f7f3eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700/70">Alpha operating system</p>
            <h1 className="mt-3 max-w-3xl text-4xl font-semibold tracking-tight text-slate-950 md:text-5xl">
              Billing operations should read like a product, not like engine wiring.
            </h1>
            <p className="mt-4 max-w-3xl text-base leading-7 text-slate-600">
              Alpha owns the control plane. Billing connections, workspaces, customers, subscriptions, recovery, and access all stay visible as product modules while Lago remains internal execution infrastructure.
            </p>

            <div className="mt-6 grid gap-3 md:grid-cols-3">
              {heroMetrics.map((metric) => (
                <div key={metric.label} className="rounded-[24px] border border-stone-200 bg-stone-50/85 p-4">
                  <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">{metric.label}</p>
                  <p className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">{metric.value}</p>
                  <p className="mt-2 text-sm text-slate-600">{metric.detail}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-[32px] border border-emerald-900/10 bg-[linear-gradient(160deg,#0f3b2f_0%,#1b5a45_100%)] p-6 text-emerald-50 shadow-[0_25px_70px_rgba(15,23,42,0.12)]">
            <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-100/70">Current session</p>
            <h2 className="mt-3 text-3xl font-semibold tracking-tight">{sessionHeadline}</h2>
            <p className="mt-4 text-sm leading-7 text-emerald-50/78">{sessionSubcopy}</p>
            <div className="mt-5 flex flex-wrap gap-2">
              <span className="rounded-full border border-white/12 bg-white/8 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-50">
                {scope === "platform" ? platformRole ?? "platform" : session?.role ?? "reader"}
              </span>
              {session?.tenant_id ? (
                <span className="rounded-full border border-white/12 bg-white/8 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-50">
                  {session.tenant_id}
                </span>
              ) : null}
            </div>
            {primaryAction ? (
              <Link
                href={primaryAction.href}
                className="mt-6 inline-flex items-center gap-2 rounded-2xl bg-white px-4 py-3 text-sm font-semibold text-emerald-900 transition hover:bg-emerald-50"
              >
                Open {primaryAction.title}
                <ArrowRight className="h-4 w-4" />
              </Link>
            ) : null}
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        <section className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Immediate focus</p>
            <h2 className="mt-3 text-2xl font-semibold tracking-tight text-slate-950">Signals that need action</h2>
            {isLoading || attentionLoading ? (
              <div className="mt-5 flex items-center gap-2 text-sm text-slate-600">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading live operating signals
              </div>
            ) : (
              <div className="mt-5 grid gap-3">
                {attentionCards.map((item) => (
                  <Link
                    key={item.title}
                    href={item.href}
                    className="rounded-[24px] border border-stone-200 bg-stone-50/85 p-4 transition hover:border-emerald-200 hover:bg-white"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">{item.title}</p>
                        <p className="mt-2 text-4xl font-semibold tracking-tight text-slate-950">{item.value}</p>
                      </div>
                      <span className="rounded-full bg-emerald-700 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-white">
                        Open
                      </span>
                    </div>
                    <p className="mt-3 text-sm leading-6 text-slate-600">{item.body}</p>
                  </Link>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
            <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Guided work</p>
                <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Start with the right operational loop</h2>
              </div>
              <p className="text-sm text-slate-500">Alpha should show ownership boundaries clearly, not force users through engine terminology.</p>
            </div>
            <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {guidedJourneys.map((item) => (
                <GuidedCard key={item.href} href={item.href} title={item.title} description={item.description} eyebrow={item.eyebrow} icon={item.icon} />
              ))}
            </div>
          </div>
        </section>

        <section className="rounded-[32px] border border-emerald-900/10 bg-white/88 p-6 shadow-[0_25px_70px_rgba(15,23,42,0.08)] backdrop-blur">
          <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-emerald-700/70">Modules</p>
              <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Browse the product by operational surface</h2>
            </div>
            <p className="text-sm text-slate-500">The shell should feel closer to a product catalogue than a flat list of routes.</p>
          </div>
          <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {workspaceModules.map((item) => (
              <ModuleCard key={item.href} href={item.href} title={item.title} description={item.description} icon={item.icon} />
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}

function GuidedCard({
  href,
  title,
  description,
  eyebrow,
  icon,
}: {
  href: string;
  title: string;
  description: string;
  eyebrow: string;
  icon: ReactNode;
}) {
  return (
    <Link href={href} className="rounded-[24px] border border-stone-200 bg-stone-50/85 p-4 transition hover:border-emerald-200 hover:bg-white">
      <span className="inline-flex rounded-2xl border border-emerald-200 bg-emerald-50 p-3">{icon}</span>
      <p className="mt-4 text-[11px] font-semibold uppercase tracking-[0.16em] text-emerald-700/70">{eyebrow}</p>
      <h3 className="mt-2 text-lg font-semibold tracking-tight text-slate-950">{title}</h3>
      <p className="mt-2 text-sm leading-6 text-slate-600">{description}</p>
    </Link>
  );
}

function ModuleCard({
  href,
  title,
  description,
  icon,
}: {
  href: string;
  title: string;
  description: string;
  icon: ReactNode;
}) {
  return (
    <Link href={href} className="rounded-[24px] border border-stone-200 bg-[#fbfaf6] p-5 transition hover:-translate-y-0.5 hover:border-emerald-200 hover:bg-white">
      <div className="flex items-start justify-between gap-4">
        <span className="inline-flex rounded-2xl border border-emerald-200 bg-emerald-50 p-3">{icon}</span>
        <span className="rounded-full border border-stone-200 bg-white px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-600">
          Open
        </span>
      </div>
      <h3 className="mt-5 text-xl font-semibold tracking-tight text-slate-950">{title}</h3>
      <p className="mt-2 text-sm leading-6 text-slate-600">{description}</p>
    </Link>
  );
}
