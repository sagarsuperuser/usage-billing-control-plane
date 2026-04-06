
import { Link } from "@tanstack/react-router";
import { ArrowLeft, CheckCircle2, LoaderCircle, Archive, Zap } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { AppBreadcrumbs } from "@/components/layout/app-breadcrumbs";
import { SectionErrorBoundary } from "@/components/ui/error-boundary";
import { StatusChip } from "@/components/ui/status-chip";
import { activatePlan, archivePlan, updatePlan, fetchAddOns, fetchCoupons, fetchPlan, fetchPricingMetrics } from "@/lib/api";
import { statusTone } from "@/lib/badge";
import { showError } from "@/lib/toast";
import { useUISession } from "@/hooks/use-ui-session";

export function PricingPlanDetailScreen({ planID }: { planID: string }) {
  const { apiBaseURL, csrfToken, isAuthenticated, scope, canWrite } = useUISession();
  const isTenantSession = isAuthenticated && scope === "tenant";
  const queryClient = useQueryClient();

  const planQuery = useQuery({
    queryKey: ["pricing-plan", apiBaseURL, planID],
    queryFn: () => fetchPlan({ runtimeBaseURL: apiBaseURL, planID }),
    enabled: isTenantSession && planID.trim().length > 0,
  });

  const metricsQuery = useQuery({
    queryKey: ["pricing-metrics", apiBaseURL],
    queryFn: () => fetchPricingMetrics({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const addOnsQuery = useQuery({
    queryKey: ["pricing-add-ons", apiBaseURL],
    queryFn: () => fetchAddOns({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });
  const couponsQuery = useQuery({
    queryKey: ["pricing-coupons", apiBaseURL],
    queryFn: () => fetchCoupons({ runtimeBaseURL: apiBaseURL }),
    enabled: isTenantSession,
  });

  const plan = planQuery.data ?? null;
  const isDraft = plan?.status === "draft";
  const isActive = plan?.status === "active";
  const isArchived = plan?.status === "archived";

  // Inline edit state (draft plans only).
  const [editName, setEditName] = useState("");
  const [editDesc, setEditDesc] = useState("");
  const [editBase, setEditBase] = useState("");

  useEffect(() => {
    if (plan) {
      setEditName(plan.name);
      setEditDesc(plan.description ?? "");
      setEditBase(String(plan.base_amount_cents / 100));
    }
  }, [plan]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["pricing-plan", apiBaseURL, planID] });
    queryClient.invalidateQueries({ queryKey: ["pricing-plans"] });
  };

  const saveMutation = useMutation({
    mutationFn: () =>
      updatePlan({
        runtimeBaseURL: apiBaseURL,
        csrfToken,
        planID,
        body: {
          name: editName.trim(),
          description: editDesc.trim() || undefined,
          base_amount_cents: Math.round(parseFloat(editBase || "0") * 100),
        },
      }),
    onSuccess: invalidate,
    onError: (err: Error) => showError(err.message),
  });

  const activateMutation = useMutation({
    mutationFn: () => activatePlan({ runtimeBaseURL: apiBaseURL, csrfToken, planID }),
    onSuccess: invalidate,
    onError: (err: Error) => showError(err.message),
  });

  const archiveMutation = useMutation({
    mutationFn: () => archivePlan({ runtimeBaseURL: apiBaseURL, csrfToken, planID }),
    onSuccess: invalidate,
    onError: (err: Error) => showError(err.message),
  });

  const linkedMetrics = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((metricsQuery.data ?? []).map((metric) => [metric.id, metric]));
    return plan.meter_ids.map((id) => byID.get(id)).filter(Boolean);
  }, [metricsQuery.data, plan]);
  const linkedAddOns = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((addOnsQuery.data ?? []).map((item) => [item.id, item]));
    return (plan.add_on_ids ?? []).map((id) => byID.get(id)).filter(Boolean);
  }, [addOnsQuery.data, plan]);
  const linkedCoupons = useMemo(() => {
    if (!plan) return [];
    const byID = new Map((couponsQuery.data ?? []).map((item) => [item.id, item]));
    return (plan.coupon_ids ?? []).map((id) => byID.get(id)).filter(Boolean);
  }, [couponsQuery.data, plan]);

  return (
    <div className="text-text-primary">
      <main className="mx-auto flex max-w-4xl flex-col gap-5 px-4 py-6 md:px-6 lg:px-8">
        <AppBreadcrumbs items={[{ href: "/pricing", label: "Pricing" }, { href: "/pricing/plans", label: "Plans" }, { label: plan?.name || planID }]} />

        {isTenantSession ? planQuery.isLoading ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <div className="animate-pulse space-y-3">
              <div className="h-6 w-48 rounded bg-surface-secondary" />
              <div className="h-4 w-72 rounded bg-surface-secondary" />
              <div className="h-32 w-full rounded bg-surface-secondary" />
            </div>
          </section>
        ) : !plan ? (
          <section className="rounded-lg border border-border bg-surface p-5 shadow-sm">
            <p className="text-sm font-semibold text-text-primary">Plan not available</p>
            <p className="mt-1 text-sm text-text-muted">The requested plan could not be loaded.</p>
            <Link to="/pricing/plans" className="mt-4 inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to plans
            </Link>
          </section>
        ) : (
          <SectionErrorBoundary>
            <div className="rounded-lg border border-border bg-surface shadow-sm divide-y divide-border">
              {/* Header with actions */}
              <div className="px-5 py-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <h1 className="text-base font-semibold text-text-primary truncate">{plan.name}</h1>
                    <StatusChip tone={statusTone(plan.status)}>{plan.status}</StatusChip>
                  </div>
                  <div className="flex items-center gap-2">
                    {canWrite && isDraft ? (
                      <button
                        type="button"
                        onClick={() => activateMutation.mutate()}
                        disabled={activateMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md bg-emerald-600 px-3 text-xs font-medium text-white transition hover:bg-emerald-700 disabled:opacity-50"
                      >
                        {activateMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <Zap className="h-3 w-3" />}
                        Activate
                      </button>
                    ) : null}
                    {canWrite && isActive ? (
                      <button
                        type="button"
                        onClick={() => archiveMutation.mutate()}
                        disabled={archiveMutation.isPending}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-muted transition hover:bg-surface-secondary disabled:opacity-50"
                      >
                        {archiveMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : <Archive className="h-3 w-3" />}
                        Archive
                      </button>
                    ) : null}
                    <Link to="/pricing/plans" className="inline-flex h-8 items-center gap-1.5 rounded-md border border-border bg-surface px-3 text-xs font-medium text-text-secondary transition hover:bg-surface-secondary">
                      <ArrowLeft className="h-3.5 w-3.5" />
                      Back
                    </Link>
                  </div>
                </div>
                {plan.description ? <p className="mt-1.5 text-xs text-text-muted">{plan.description}</p> : null}

                {activateMutation.isSuccess ? (
                  <p className="mt-2 flex items-center gap-1.5 text-xs text-emerald-600">
                    <CheckCircle2 className="h-3.5 w-3.5" /> Plan activated — ready for subscriptions
                  </p>
                ) : null}
                {archiveMutation.isSuccess ? (
                  <p className="mt-2 flex items-center gap-1.5 text-xs text-text-muted">
                    <Archive className="h-3.5 w-3.5" /> Plan archived — no new subscriptions
                  </p>
                ) : null}
              </div>

              {/* Editable fields (draft only) */}
              {canWrite && isDraft ? (
                <div className="px-5 py-4">
                  <p className="text-xs font-medium text-text-faint mb-3">Edit plan (available while draft)</p>
                  <div className="grid gap-3 max-w-lg">
                    <label className="grid gap-1">
                      <span className="text-xs text-text-muted">Name</span>
                      <input
                        value={editName}
                        onChange={(e) => setEditName(e.target.value)}
                        className="h-9 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                      />
                    </label>
                    <label className="grid gap-1">
                      <span className="text-xs text-text-muted">Description</span>
                      <input
                        value={editDesc}
                        onChange={(e) => setEditDesc(e.target.value)}
                        className="h-9 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                      />
                    </label>
                    <label className="grid gap-1">
                      <span className="text-xs text-text-muted">Base price ({plan.currency})</span>
                      <input
                        type="number"
                        min="0"
                        step="0.01"
                        value={editBase}
                        onChange={(e) => setEditBase(e.target.value)}
                        className="h-9 rounded-lg border border-border bg-surface px-3 text-sm text-text-primary outline-none ring-slate-400 transition focus:ring-2"
                      />
                    </label>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => saveMutation.mutate()}
                        disabled={saveMutation.isPending || !editName.trim()}
                        className="inline-flex h-8 items-center gap-1.5 rounded-md bg-slate-900 px-3 text-xs font-medium text-white transition hover:bg-slate-800 disabled:opacity-50 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-100"
                      >
                        {saveMutation.isPending ? <LoaderCircle className="h-3 w-3 animate-spin" /> : null}
                        Save
                      </button>
                      {saveMutation.isSuccess ? (
                        <span className="flex items-center gap-1 text-xs text-emerald-600">
                          <CheckCircle2 className="h-3 w-3" /> Saved
                        </span>
                      ) : null}
                    </div>
                  </div>
                </div>
              ) : null}

              {/* Read-only details */}
              <div className="px-5 py-4">
                <dl className="grid grid-cols-2 gap-x-8 gap-y-3 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs text-text-faint">Code</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary font-mono">{plan.code}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Currency</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{plan.currency}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Interval</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{plan.billing_interval}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-text-faint">Base price</dt>
                    <dd className="mt-0.5 text-sm text-text-secondary">{(plan.base_amount_cents / 100).toFixed(2)} {plan.currency}</dd>
                  </div>
                  {!isDraft ? (
                    <div>
                      <dt className="text-xs text-text-faint">Status</dt>
                      <dd className="mt-0.5 text-sm text-text-secondary">{plan.status}</dd>
                    </div>
                  ) : null}
                </dl>
              </div>

              {/* Linked metrics */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-text-faint mb-2">Linked metrics ({linkedMetrics.length})</p>
                {linkedMetrics.length === 0 ? (
                  <p className="text-sm text-text-muted">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedMetrics.map((metric) =>
                      metric ? (
                        <li key={metric.id} className="text-sm text-text-secondary">
                          {metric.name} <span className="font-mono text-xs text-text-faint">{metric.key}</span> &middot; {metric.aggregation} &middot; {metric.unit}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>

              {/* Linked add-ons */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-text-faint mb-2">Linked add-ons ({linkedAddOns.length})</p>
                {linkedAddOns.length === 0 ? (
                  <p className="text-sm text-text-muted">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedAddOns.map((addOn) =>
                      addOn ? (
                        <li key={addOn.id} className="text-sm text-text-secondary">
                          {addOn.name} <span className="font-mono text-xs text-text-faint">{addOn.code}</span> &middot; {(addOn.amount_cents / 100).toFixed(2)} {addOn.currency} &middot; {addOn.billing_interval}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>

              {/* Linked coupons */}
              <div className="px-5 py-4">
                <p className="text-xs font-medium text-text-faint mb-2">Linked coupons ({linkedCoupons.length})</p>
                {linkedCoupons.length === 0 ? (
                  <p className="text-sm text-text-muted">None</p>
                ) : (
                  <ul className="space-y-1">
                    {linkedCoupons.map((coupon) =>
                      coupon ? (
                        <li key={coupon.id} className="text-sm text-text-secondary">
                          {coupon.name} <span className="font-mono text-xs text-text-faint">{coupon.code}</span> &middot; {coupon.discount_type === "percent_off" ? `${coupon.percent_off}% off` : `${(coupon.amount_off_cents / 100).toFixed(2)} ${coupon.currency} off`}
                        </li>
                      ) : null,
                    )}
                  </ul>
                )}
              </div>

              {/* Lifecycle info */}
              {isArchived ? (
                <div className="px-5 py-3 bg-surface-secondary">
                  <p className="text-xs text-text-muted">This plan is archived. Existing subscriptions continue, but no new subscriptions can use it.</p>
                </div>
              ) : null}
            </div>
          </SectionErrorBoundary>
        ) : null}
      </main>
    </div>
  );
}
