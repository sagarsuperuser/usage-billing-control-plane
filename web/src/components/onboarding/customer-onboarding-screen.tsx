"use client";

import { useMemo, useState } from "react";
import { CreditCard, LoaderCircle, RefreshCw, RotateCcw, UserRoundPlus } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { SessionLoginCard } from "@/components/auth/session-login-card";
import { ControlPlaneNav } from "@/components/layout/control-plane-nav";
import {
  fetchCustomerReadiness,
  fetchCustomers,
  onboardCustomer,
  refreshCustomerPaymentSetup,
  retryCustomerBillingSync,
} from "@/lib/api";
import { formatExactTimestamp } from "@/lib/format";
import { type Customer, type CustomerOnboardingResult } from "@/lib/types";
import { useUISession } from "@/hooks/use-ui-session";

const EMPTY_CUSTOMERS: Customer[] = [];

function readinessTone(status?: string): string {
  return status === "ready"
    ? "border-emerald-400/40 bg-emerald-500/10 text-emerald-100"
    : "border-amber-400/40 bg-amber-500/10 text-amber-100";
}

function profileTone(status?: string): string {
  switch ((status || "").toLowerCase()) {
    case "ready":
      return "border-emerald-400/40 bg-emerald-500/10 text-emerald-100";
    case "sync_error":
      return "border-rose-400/40 bg-rose-500/10 text-rose-100";
    case "incomplete":
      return "border-amber-400/40 bg-amber-500/10 text-amber-100";
    default:
      return "border-slate-500/40 bg-slate-700/30 text-slate-100";
  }
}

export function CustomerOnboardingScreen() {
  const queryClient = useQueryClient();
  const { apiBaseURL, csrfToken, canWrite, isAuthenticated, role, scope } = useUISession();

  const [statusFilter, setStatusFilter] = useState("");
  const [selectedExternalID, setSelectedExternalID] = useState("");
  const [externalID, setExternalID] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [legalName, setLegalName] = useState("");
  const [addressLine1, setAddressLine1] = useState("");
  const [city, setCity] = useState("");
  const [postalCode, setPostalCode] = useState("");
  const [country, setCountry] = useState("");
  const [currency, setCurrency] = useState("USD");
  const [providerCode, setProviderCode] = useState("");
  const [startPaymentSetup, setStartPaymentSetup] = useState(true);
  const [paymentMethodType, setPaymentMethodType] = useState("card");
  const [flash, setFlash] = useState<string | null>(null);
  const [result, setResult] = useState<CustomerOnboardingResult | null>(null);

  const customersQuery = useQuery({
    queryKey: ["customers", apiBaseURL, statusFilter],
    queryFn: () => fetchCustomers({ runtimeBaseURL: apiBaseURL, status: statusFilter || undefined, limit: 100 }),
    enabled: isAuthenticated && scope === "tenant",
  });

  const readinessQuery = useQuery({
    queryKey: ["customer-readiness", apiBaseURL, selectedExternalID],
    queryFn: () => fetchCustomerReadiness({ runtimeBaseURL: apiBaseURL, externalID: selectedExternalID }),
    enabled: isAuthenticated && scope === "tenant" && selectedExternalID.trim().length > 0,
  });

  const onboardingMutation = useMutation({
    mutationFn: () =>
      onboardCustomer({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        body: {
          external_id: externalID.trim(),
          display_name: displayName.trim(),
          email: email.trim(),
          start_payment_setup: startPaymentSetup,
          payment_method_type: paymentMethodType.trim() || undefined,
          billing_profile: {
            legal_name: legalName.trim(),
            email: email.trim(),
            billing_address_line1: addressLine1.trim(),
            billing_city: city.trim(),
            billing_postal_code: postalCode.trim(),
            billing_country: country.trim(),
            currency: currency.trim(),
            provider_code: providerCode.trim(),
          },
        },
      }),
    onSuccess: async (payload) => {
      setResult(payload);
      setSelectedExternalID(payload.customer.external_id);
      setExternalID(payload.customer.external_id);
      setDisplayName(payload.customer.display_name);
      setEmail(payload.customer.email ?? "");
      setFlash(
        payload.payment_setup_started
          ? `Customer ${payload.customer.external_id} is onboarded and checkout is ready.`
          : `Customer ${payload.customer.external_id} is onboarded and readiness has been refreshed.`
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.customer.external_id] }),
      ]);
    },
  });

  const retryMutation = useMutation({
    mutationFn: (customerID: string) => retryCustomerBillingSync({ runtimeBaseURL: apiBaseURL, csrfToken, externalID: customerID }),
    onSuccess: async (payload) => {
      setSelectedExternalID(payload.external_id);
      setFlash(`Billing sync retried for ${payload.external_id}.`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.external_id] }),
      ]);
    },
  });

  const refreshMutation = useMutation({
    mutationFn: (customerID: string) => refreshCustomerPaymentSetup({ runtimeBaseURL: apiBaseURL, csrfToken, externalID: customerID }),
    onSuccess: async (payload) => {
      setSelectedExternalID(payload.external_id);
      setFlash(`Payment setup refreshed for ${payload.external_id}.`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["customers"] }),
        queryClient.invalidateQueries({ queryKey: ["customer-readiness", apiBaseURL, payload.external_id] }),
      ]);
    },
  });

  const customers = customersQuery.data ?? EMPTY_CUSTOMERS;
  const selectedCustomer = customers.find((customer) => customer.external_id === selectedExternalID) ?? result?.customer ?? null;
  const selectedReadiness = readinessQuery.data ?? result?.readiness ?? null;

  const topMetrics = useMemo(() => {
    const active = customers.filter((customer) => customer.status === "active").length;
    const withLago = customers.filter((customer) => Boolean(customer.lago_customer_id)).length;
    return { total: customers.length, active, withLago };
  }, [customers]);

  return (
    <div className="relative min-h-screen overflow-hidden bg-[radial-gradient(circle_at_top_right,_#164e63_0%,_#0f172a_34%,_#070b13_78%)] text-slate-100">
      <div className="pointer-events-none absolute inset-0 opacity-55">
        <div className="absolute -left-24 top-4 h-72 w-72 rounded-full bg-emerald-500/15 blur-3xl" />
        <div className="absolute right-0 top-1/3 h-96 w-96 rounded-full bg-cyan-500/10 blur-3xl" />
      </div>

      <main className="relative mx-auto flex max-w-[1440px] flex-col gap-6 px-4 py-6 md:px-8 lg:px-10">
        <ControlPlaneNav />

        <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p className="text-xs uppercase tracking-[0.24em] text-cyan-300/80">Tenant Admin Console</p>
              <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white md:text-4xl">Customer Onboarding</h1>
              <p className="mt-3 max-w-3xl text-sm text-slate-300 md:text-base">
                Create or reconcile a customer, apply the billing profile, start payment setup, and verify readiness from a single Alpha workflow.
              </p>
            </div>
            <div className="grid grid-cols-3 gap-3 text-sm">
              <MetricCard label="Customers" value={topMetrics.total} />
              <MetricCard label="Active" value={topMetrics.active} tone="success" />
              <MetricCard label="Synced" value={topMetrics.withLago} tone="info" />
            </div>
          </div>
        </section>

        {!isAuthenticated ? <SessionLoginCard /> : null}
        {isAuthenticated && scope !== "tenant" ? (
          <ScopeNotice
            title="Tenant session required"
            body="This screen drives tenant-scoped customer and payment APIs. Sign in with a tenant reader, writer, or admin API key."
          />
        ) : null}
        {isAuthenticated && scope === "tenant" && !canWrite ? (
          <ScopeNotice
            title="Read-only session"
            body={`Current session role ${role ?? "reader"} can inspect customer readiness, but a writer or admin key is required to run the onboarding workflow.`}
          />
        ) : null}

        {flash ? (
          <section className="rounded-2xl border border-emerald-400/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-100">
            {flash}
          </section>
        ) : null}

        <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Workflow</p>
                <h2 className="mt-2 text-xl font-semibold text-white">First-customer happy path</h2>
              </div>
              <span className="inline-flex rounded-xl border border-cyan-400/40 bg-cyan-500/10 p-3 text-cyan-100">
                <UserRoundPlus className="h-5 w-5" />
              </span>
            </div>

            <div className="mt-6 grid gap-3 md:grid-cols-2">
              <InputField label="Customer external ID" value={externalID} onChange={setExternalID} placeholder="cust_acme_primary" />
              <InputField label="Display name" value={displayName} onChange={setDisplayName} placeholder="Acme Primary Customer" />
              <InputField label="Billing email" value={email} onChange={setEmail} placeholder="billing@acme.test" />
              <InputField label="Legal name" value={legalName} onChange={setLegalName} placeholder="Acme Primary Customer LLC" />
              <InputField label="Billing address line 1" value={addressLine1} onChange={setAddressLine1} placeholder="1 Billing Street" />
              <InputField label="Billing city" value={city} onChange={setCity} placeholder="Bengaluru" />
              <InputField label="Billing postal code" value={postalCode} onChange={setPostalCode} placeholder="560001" />
              <InputField label="Billing country" value={country} onChange={setCountry} placeholder="IN" />
              <InputField label="Currency" value={currency} onChange={setCurrency} placeholder="USD" />
              <InputField label="Provider code" value={providerCode} onChange={setProviderCode} placeholder="stripe_default" />
            </div>

            <div className="mt-4 rounded-2xl border border-white/10 bg-slate-950/55 p-4">
              <p className="text-xs font-medium uppercase tracking-[0.16em] text-slate-400">Payment setup</p>
              <div className="mt-3 grid gap-3 md:grid-cols-[1fr_auto] md:items-end">
                <InputField
                  label="Payment method type"
                  value={paymentMethodType}
                  onChange={setPaymentMethodType}
                  placeholder="card"
                />
                <label className="flex items-center gap-2 text-sm text-slate-200 md:mb-2">
                  <input
                    type="checkbox"
                    checked={startPaymentSetup}
                    onChange={(event) => setStartPaymentSetup(event.target.checked)}
                    className="h-4 w-4 rounded border-white/20 bg-slate-950/70"
                  />
                  Start payment setup checkout
                </label>
              </div>
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFlash(null);
                  onboardingMutation.mutate();
                }}
                disabled={!canWrite || !csrfToken || onboardingMutation.isPending || !externalID.trim() || !displayName.trim()}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-4 text-sm font-medium text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {onboardingMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
                Run customer onboarding
              </button>
              <button
                type="button"
                onClick={() => {
                  setExternalID("");
                  setDisplayName("");
                  setEmail("");
                  setLegalName("");
                  setAddressLine1("");
                  setCity("");
                  setPostalCode("");
                  setCountry("");
                  setCurrency("USD");
                  setProviderCode("");
                  setPaymentMethodType("card");
                  setStartPaymentSetup(true);
                  setResult(null);
                  setFlash(null);
                }}
                className="inline-flex h-11 items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-slate-200 transition hover:bg-white/10"
              >
                Reset form
              </button>
            </div>

            {result?.checkout_url ? (
              <div className="mt-6 rounded-2xl border border-emerald-400/40 bg-emerald-500/10 p-4 text-sm text-emerald-100">
                <p className="font-semibold text-emerald-50">Checkout URL</p>
                <a
                  href={result.checkout_url}
                  target="_blank"
                  rel="noreferrer"
                  className="mt-2 block break-all rounded-xl border border-white/10 bg-slate-950/60 px-3 py-3 font-mono text-xs text-emerald-50 hover:bg-slate-950/80"
                >
                  {result.checkout_url}
                </a>
              </div>
            ) : null}
          </section>

          <section className="rounded-3xl border border-white/10 bg-slate-900/70 p-6 backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-cyan-300/80">Inventory + Recovery</p>
                <h2 className="mt-2 text-xl font-semibold text-white">Existing customers</h2>
              </div>
              <button
                type="button"
                onClick={() => {
                  void Promise.all([
                    customersQuery.refetch(),
                    selectedExternalID ? readinessQuery.refetch() : Promise.resolve(),
                  ]);
                }}
                disabled={!isAuthenticated || customersQuery.isFetching || readinessQuery.isFetching}
                className="inline-flex h-10 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <RefreshCw className={`h-4 w-4 ${(customersQuery.isFetching || readinessQuery.isFetching) ? "animate-spin" : ""}`} />
                Refresh
              </button>
            </div>

            <div className="mt-4 grid gap-3 md:grid-cols-[0.9fr_1.1fr]">
              <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="text-sm font-semibold text-white">Customers</p>
                  <select
                    value={statusFilter}
                    onChange={(event) => setStatusFilter(event.target.value)}
                    className="h-9 rounded-lg border border-white/15 bg-slate-950/70 px-3 text-xs text-slate-100 outline-none ring-cyan-400 transition focus:ring-2"
                  >
                    <option value="">All</option>
                    <option value="active">Active</option>
                    <option value="suspended">Suspended</option>
                    <option value="archived">Archived</option>
                  </select>
                </div>
                <div className="mt-3 max-h-[420px] space-y-2 overflow-y-auto pr-1">
                  {customers.map((customer) => {
                    const active = customer.external_id === selectedExternalID;
                    return (
                      <button
                        key={customer.external_id}
                        type="button"
                        onClick={() => setSelectedExternalID(customer.external_id)}
                        className={`w-full rounded-2xl border p-3 text-left transition ${
                          active
                            ? "border-cyan-400/50 bg-cyan-500/10"
                            : "border-white/10 bg-white/5 hover:bg-white/10"
                        }`}
                      >
                        <div className="flex items-center justify-between gap-3">
                          <p className="font-semibold text-white">{customer.display_name}</p>
                          <span className={`rounded-full px-2 py-1 text-[11px] uppercase tracking-[0.14em] ${profileTone(customer.status === "active" ? "ready" : "sync_error")}`}>
                            {customer.status}
                          </span>
                        </div>
                        <p className="mt-1 font-mono text-xs text-slate-400">{customer.external_id}</p>
                      </button>
                    );
                  })}
                  {!customersQuery.isFetching && customers.length === 0 ? (
                    <p className="rounded-2xl border border-dashed border-white/10 px-4 py-6 text-sm text-slate-400">
                      No customers loaded for the selected filter.
                    </p>
                  ) : null}
                </div>
              </div>

              <div className="rounded-2xl border border-white/10 bg-slate-950/55 p-4">
                {!selectedCustomer || !selectedReadiness ? (
                  <div className="rounded-2xl border border-dashed border-white/10 px-4 py-8 text-sm text-slate-400">
                    Select a customer to inspect readiness and run recovery actions.
                  </div>
                ) : (
                  <>
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <p className="text-xs uppercase tracking-[0.16em] text-slate-400">Selected customer</p>
                        <h3 className="mt-1 text-lg font-semibold text-white">{selectedCustomer.display_name}</h3>
                        <p className="font-mono text-xs text-slate-400">{selectedCustomer.external_id}</p>
                      </div>
                      <span className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em] ${readinessTone(selectedReadiness.status)}`}>
                        {selectedReadiness.status}
                      </span>
                    </div>

                    <div className="mt-4 grid gap-3 md:grid-cols-2">
                      <StatusCard title="Billing profile" value={selectedReadiness.billing_profile_status} />
                      <StatusCard title="Payment setup" value={selectedReadiness.payment_setup_status} />
                    </div>

                    <div className="mt-4 rounded-2xl border border-white/10 bg-slate-900/60 p-4 text-sm text-slate-200">
                      <p className="font-semibold text-white">Missing steps</p>
                      <div className="mt-3 flex flex-wrap gap-2">
                        {selectedReadiness.missing_steps.length > 0 ? (
                          selectedReadiness.missing_steps.map((step) => (
                            <span key={step} className="rounded-full border border-white/10 bg-white/5 px-3 py-1 text-xs text-slate-300">
                              {step}
                            </span>
                          ))
                        ) : (
                          <span className="rounded-full border border-emerald-400/30 bg-emerald-500/10 px-3 py-1 text-xs text-emerald-100">
                            No missing steps
                          </span>
                        )}
                      </div>
                    </div>

                    <dl className="mt-4 grid gap-3 md:grid-cols-2">
                      <MetaItem label="Lago customer ID" value={selectedCustomer.lago_customer_id || "-"} mono />
                      <MetaItem label="Billing sync error" value={selectedReadiness.billing_profile.last_sync_error || "-"} />
                      <MetaItem label="Last synced" value={formatExactTimestamp(selectedReadiness.billing_profile.last_synced_at)} />
                      <MetaItem label="Last verified" value={formatExactTimestamp(selectedReadiness.payment_setup.last_verified_at)} />
                    </dl>

                    <div className="mt-4 flex flex-wrap gap-3">
                      <button
                        type="button"
                        onClick={() => retryMutation.mutate(selectedCustomer.external_id)}
                        disabled={!canWrite || !csrfToken || retryMutation.isPending}
                        className="inline-flex h-10 items-center gap-2 rounded-xl border border-amber-400/40 bg-amber-500/10 px-3 text-sm text-amber-100 transition hover:bg-amber-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {retryMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RotateCcw className="h-4 w-4" />}
                        Retry billing sync
                      </button>
                      <button
                        type="button"
                        onClick={() => refreshMutation.mutate(selectedCustomer.external_id)}
                        disabled={!canWrite || !csrfToken || refreshMutation.isPending}
                        className="inline-flex h-10 items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-500/20 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {refreshMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                        Refresh payment setup
                      </button>
                    </div>
                  </>
                )}
              </div>
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}

function ScopeNotice({ title, body }: { title: string; body: string }) {
  return (
    <section className="rounded-2xl border border-amber-400/40 bg-amber-500/10 p-4 text-sm text-amber-100">
      <p className="font-semibold text-amber-50">{title}</p>
      <p className="mt-1 text-amber-100/90">{body}</p>
    </section>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "info" }) {
  const toneClass = tone === "success" ? "text-emerald-100" : tone === "info" ? "text-cyan-100" : "text-white";
  return (
    <div className="rounded-2xl border border-white/10 bg-slate-950/50 px-4 py-3">
      <p className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</p>
      <p className={`mt-2 text-2xl font-semibold ${toneClass}`}>{value}</p>
    </div>
  );
}

function InputField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <label className="grid gap-2">
      <span className="text-xs font-medium uppercase tracking-[0.16em] text-slate-300">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-11 rounded-xl border border-white/15 bg-slate-950/70 px-3 text-sm text-slate-100 outline-none ring-cyan-400 transition placeholder:text-slate-500 focus:ring-2"
      />
    </label>
  );
}

function StatusCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
      <p className="text-sm font-semibold text-white">{title}</p>
      <span className={`mt-3 inline-flex rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.14em] ${profileTone(value)}`}>
        {value}
      </span>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <dt className="text-xs uppercase tracking-[0.15em] text-slate-400">{label}</dt>
      <dd className={`mt-2 text-sm text-slate-100 ${mono ? "font-mono" : ""}`}>{value || "-"}</dd>
    </div>
  );
}
