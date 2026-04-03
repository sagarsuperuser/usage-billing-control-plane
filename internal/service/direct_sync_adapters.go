package service

import (
	"context"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

// ---------------------------------------------------------------------------
// No-op adapters for concepts that are now local-only (no external sync needed).
// These replace the Lago sync adapters that previously pushed data to Lago.
// ---------------------------------------------------------------------------

// DirectMeterSyncAdapter is a no-op: meters are stored in Postgres only.
type DirectMeterSyncAdapter struct{}

func (a *DirectMeterSyncAdapter) SyncMeter(_ context.Context, _ domain.Meter) error { return nil }

// DirectPlanSyncAdapter is a no-op: plans are stored in Postgres only.
type DirectPlanSyncAdapter struct{}

func (a *DirectPlanSyncAdapter) SyncPlan(_ context.Context, _ domain.Plan, _ []PlanSyncComponent) error {
	return nil
}

// DirectUsageSyncAdapter is a no-op: usage events are stored in Postgres only.
// The billing cycle Temporal workflow aggregates them at invoice generation time.
type DirectUsageSyncAdapter struct{}

func (a *DirectUsageSyncAdapter) SyncUsageEvent(_ context.Context, _ domain.UsageEvent, _ domain.Meter, _ domain.Subscription) error {
	return nil
}

// DirectTaxSyncAdapter is a no-op: taxes are stored in Postgres only.
type DirectTaxSyncAdapter struct{}

func (a *DirectTaxSyncAdapter) SyncTax(_ context.Context, _ domain.Tax) error { return nil }

// DirectBillingEntitySettingsSyncAdapter is a no-op: settings are stored in Postgres only.
type DirectBillingEntitySettingsSyncAdapter struct{}

func (a *DirectBillingEntitySettingsSyncAdapter) SyncBillingEntitySettings(_ context.Context, _ domain.WorkspaceBillingSettings) error {
	return nil
}

// ---------------------------------------------------------------------------
// Subscription sync adapter: initializes billing cycle tracking on new subs.
// ---------------------------------------------------------------------------

type DirectSubscriptionSyncAdapter struct {
	store store.Repository
}

func NewDirectSubscriptionSyncAdapter(repo store.Repository) *DirectSubscriptionSyncAdapter {
	return &DirectSubscriptionSyncAdapter{store: repo}
}

func (a *DirectSubscriptionSyncAdapter) SyncSubscription(_ context.Context, subscription domain.Subscription, _ domain.Customer, plan domain.Plan) error {
	if subscription.Status != domain.SubscriptionStatusActive {
		return nil
	}

	now := time.Now().UTC()
	startAt := now
	if subscription.StartedAt != nil {
		startAt = *subscription.StartedAt
	} else if subscription.ActivatedAt != nil {
		startAt = *subscription.ActivatedAt
	}

	var periodEnd time.Time
	switch plan.BillingInterval {
	case domain.BillingIntervalYearly:
		periodEnd = startAt.AddDate(1, 0, 0)
	default:
		periodEnd = startAt.AddDate(0, 1, 0)
	}

	return a.store.UpdateSubscriptionBillingCycle(
		subscription.TenantID,
		subscription.ID,
		startAt,
		periodEnd,
		periodEnd, // next_billing_at = end of first period
	)
}
