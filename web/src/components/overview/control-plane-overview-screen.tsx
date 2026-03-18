"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import {
  Activity,
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

  const sessionTitle =
    scope === "platform"
      ? "Platform overview"
      : `Workspace overview${session?.tenant_id ? ` · ${session.tenant_id}` : ""}`;

  const summaryCards =
    scope === "platform"
      ? [
          { label: "Workspaces", value: platformMetrics.total, tone: "default" as const },
          { label: "Connected credentials", value: platformMetrics.connectedProviders, tone: "success" as const },
          { label: "Missing billing", value: platformMetrics.missingBilling, tone: "warn" as const },
          { label: "Provider errors", value: platformMetrics.providerErrors, tone: "danger" as const },
        ]
      : [
          { label: "Customers", value: tenantMetrics.total, tone: "default" as const },
          { label: "Billing-ready", value: tenantMetrics.billingReady, tone: "success" as const },
          { label: "Pending payment setup", value: tenantMetrics.pendingPayment, tone: "warn" as const },
          { label: "Sync errors", value: tenantMetrics.syncErrors, tone: "danger" as const },
        ];

  const attentionItems =
    scope === "platform"
      ? [
          {
            title: "Billing connection errors",
            value: platformMetrics.providerErrors,
            body: "Resolve sync issues before attaching those credentials to additional workspaces.",
            href: "/billing-connections",
          },
          {
            title: "Workspaces missing pricing",
            value: platformMetrics.missingPricing,
            body: "Pricing remains a common blocker before a workspace can operate cleanly.",
            href: "/workspaces",
          },
          {
            title: "Workspaces missing first customer",
            value: platformMetrics.missingFirstCustomer,
            body: "A workspace is not operational until its first billable customer exists.",
            href: "/workspaces",
          },
        ]
      : [
          {
            title: "Customers waiting on payment setup",
            value: tenantMetrics.pendingPayment,
            body: "Finish payer-owned payment setup before marking those customers as live.",
            href: "/subscriptions",
          },
          {
            title: "Customers with billing sync errors",
            value: tenantMetrics.syncErrors,
            body: "Use diagnostics and recovery paths before retrying collection or invoice actions.",
            href: "/payment-operations",
          },
          {
            title: "Billing-ready customers",
            value: tenantMetrics.billingReady,
            body: "These customers are clear for subscription and payment operations.",
            href: "/customers",
          },
        ];

  const actionItems = [
    {
      href: "/billing-connections/new",
      title: "Create billing connection",
      body: "Create a reusable provider credential and sync it into Lago under Alpha ownership.",
      icon: <CreditCard className="h-4 w-4 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/workspaces/new",
      title: "Launch workspace",
      body: "Create a workspace, attach one active billing path, and hand off access cleanly.",
      icon: <Building2 className="h-4 w-4 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/customers/new",
      title: "Create first customer",
      body: "Create the first billable customer and move directly into subscription and payment setup.",
      icon: <UserRoundPlus className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  const moduleItems = [
    {
      href: "/billing-connections",
      title: "Billing Connections",
      body: "Provider credentials, sync health, and workspace reuse.",
      icon: <CreditCard className="h-4 w-4 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/workspaces",
      title: "Workspaces",
      body: "Workspace readiness, billing attachment, and handoff.",
      icon: <Building2 className="h-4 w-4 text-emerald-700" />,
      scope: "platform" as const,
    },
    {
      href: "/customers",
      title: "Customers",
      body: "Readiness, billing state, and payment setup progression.",
      icon: <UserRoundPlus className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/subscriptions",
      title: "Subscriptions",
      body: "Activation state and payer-owned payment setup.",
      icon: <ShieldCheck className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/payment-operations",
      title: "Payments",
      body: "Operational follow-up for payment failures and retries.",
      icon: <Activity className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/replay-operations",
      title: "Recovery",
      body: "Repair failed processing runs and inspect replay state.",
      icon: <Workflow className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
    {
      href: "/invoice-explainability",
      title: "Explainability",
      body: "Trace invoice outcomes when support or finance needs evidence.",
      icon: <ReceiptText className="h-4 w-4 text-emerald-700" />,
      scope: "tenant" as const,
    },
  ].filter((item) => item.scope === scopeKey);

  const primaryAction = actionItems[0] ?? moduleItems[0] ?? null;

  return (
    <div className="min-h-screen bg-[linear-gradient(180deg,#eef4ef_0%,#f6f2eb_100%)] text-slate-900">
      <main className="mx-auto flex max-w-[1360px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-stone-200 bg-white/92 shadow-[0_18px_50px_rgba(15,23,42,0.06)]">
          <div className="grid gap-6 p-5 lg:grid-cols-[minmax(0,1.5fr)_320px] lg:p-6">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-slate-500">Overview</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">{sessionTitle}</h1>
              <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
                Alpha stays product-first while Lago remains the execution backend. Use this surface to see operational status, next actions, and module-level health without dropping into engine terminology.
              </p>
              <div className="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                {summaryCards.map((item) => (
                  <SummaryCard key={item.label} label={item.label} value={item.value} tone={item.tone} />
                ))}
              </div>
            </div>

            <div className="rounded-2xl border border-stone-200 bg-stone-50 p-4">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Current context</p>
              <p className="mt-2 text-lg font-semibold tracking-tight text-slate-950">
                {scope === "platform" ? platformRole ?? "platform" : session?.role ?? "reader"}
              </p>
              <p className="mt-2 text-sm leading-6 text-slate-600">
                {scope === "platform"
                  ? "Manage reusable billing assets, launch workspaces, and control handoff."
                  : "Run pricing, customers, subscriptions, and workspace operations inside one workspace boundary."}
              </p>
              {session?.tenant_id ? (
                <p className="mt-3 rounded-xl border border-stone-200 bg-white px-3 py-2 text-sm font-medium text-slate-700">
                  Workspace: {session.tenant_id}
                </p>
              ) : null}
              {primaryAction ? (
                <Link
                  href={primaryAction.href}
                  className="mt-4 inline-flex w-full items-center justify-center rounded-xl bg-emerald-700 px-4 py-3 text-sm font-semibold text-white transition hover:bg-emerald-800"
                >
                  Open {primaryAction.title}
                </Link>
              ) : null}
            </div>
          </div>
        </section>

        {!isAuthenticated ? <LoginRedirectNotice /> : null}

        <section className="grid gap-6 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
          <div className="rounded-3xl border border-stone-200 bg-white/92 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.06)] lg:p-6">
            <div className="flex items-end justify-between gap-4">
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Needs attention</p>
                <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Operational focus</h2>
              </div>
            </div>
            {isLoading || attentionLoading ? (
              <div className="mt-5 flex items-center gap-2 text-sm text-slate-600">
                <LoaderCircle className="h-4 w-4 animate-spin" />
                Loading live attention data
              </div>
            ) : (
              <div className="mt-5 divide-y divide-stone-200">
                {attentionItems.map((item) => (
                  <AttentionRow key={item.title} href={item.href} title={item.title} value={item.value} body={item.body} />
                ))}
              </div>
            )}
          </div>

          <div className="grid gap-6">
            <section className="rounded-3xl border border-stone-200 bg-white/92 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.06)] lg:p-6">
              <div className="flex items-end justify-between gap-4">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Primary actions</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Start with the right loop</h2>
                </div>
              </div>
              <div className="mt-5 divide-y divide-stone-200">
                {actionItems.map((item) => (
                  <ActionRow key={item.href} href={item.href} title={item.title} body={item.body} icon={item.icon} />
                ))}
              </div>
            </section>

            <section className="rounded-3xl border border-stone-200 bg-white/92 p-5 shadow-[0_18px_50px_rgba(15,23,42,0.06)] lg:p-6">
              <div className="flex items-end justify-between gap-4">
                <div>
                  <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-500">Modules</p>
                  <h2 className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">Open an operating surface</h2>
                </div>
              </div>
              <div className="mt-5 divide-y divide-stone-200">
                {moduleItems.map((item) => (
                  <ActionRow key={item.href} href={item.href} title={item.title} body={item.body} icon={item.icon} />
                ))}
              </div>
            </section>
          </div>
        </section>
      </main>
    </div>
  );
}

function SummaryCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "default" | "success" | "warn" | "danger";
}) {
  const toneClass =
    tone === "success"
      ? "text-emerald-700"
      : tone === "warn"
        ? "text-amber-700"
        : tone === "danger"
          ? "text-rose-700"
          : "text-slate-950";

  return (
    <div className="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500">{label}</p>
      <p className={`mt-2 text-3xl font-semibold tracking-tight ${toneClass}`}>{value}</p>
    </div>
  );
}

function AttentionRow({
  href,
  title,
  value,
  body,
}: {
  href: string;
  title: string;
  value: number;
  body: string;
}) {
  return (
    <Link href={href} className="flex items-start justify-between gap-4 py-4 first:pt-0 last:pb-0">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-slate-950">{title}</p>
        <p className="mt-1 text-sm leading-6 text-slate-600">{body}</p>
      </div>
      <div className="shrink-0 text-right">
        <p className="text-2xl font-semibold tracking-tight text-slate-950">{value}</p>
        <p className="mt-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">Open</p>
      </div>
    </Link>
  );
}

function ActionRow({
  href,
  title,
  body,
  icon,
}: {
  href: string;
  title: string;
  body: string;
  icon: ReactNode;
}) {
  return (
    <Link href={href} className="flex items-start gap-4 py-4 first:pt-0 last:pb-0">
      <span className="mt-0.5 inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-emerald-200 bg-emerald-50">
        {icon}
      </span>
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-semibold text-slate-950">{title}</p>
          <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-emerald-700">Open</span>
        </div>
        <p className="mt-1 text-sm leading-6 text-slate-600">{body}</p>
      </div>
    </Link>
  );
}
