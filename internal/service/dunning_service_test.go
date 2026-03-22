package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

func TestDunningServiceGetPolicyCreatesDefault(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	svc, err := NewDunningService(repo)
	if err != nil {
		t.Fatalf("new dunning service: %v", err)
	}
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	policy, err := svc.GetPolicy("tenant_a")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if policy.TenantID != "tenant_a" {
		t.Fatalf("expected tenant_a, got %q", policy.TenantID)
	}
	if !policy.Enabled {
		t.Fatalf("expected default policy enabled")
	}
	if got := len(policy.RetrySchedule); got != 3 {
		t.Fatalf("expected retry schedule length 3, got %d", got)
	}
}

func TestDunningServiceEnsureRunForInvoiceAwaitingPaymentSetup(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_1"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_1",
		CustomerExternalID: "cust_1",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_1"] = domain.Customer{
		ID:         "cust_row_1",
		TenantID:   "tenant_a",
		ExternalID: "cust_1",
		Status:     domain.CustomerStatusActive,
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.EnsureRunForInvoice("tenant_a", "inv_1")
	if err != nil {
		t.Fatalf("ensure run: %v", err)
	}
	if !result.Created || result.Run == nil {
		t.Fatalf("expected created run")
	}
	if result.Run.State != domain.DunningRunStateAwaitingPaymentSetup {
		t.Fatalf("expected awaiting_payment_setup, got %q", result.Run.State)
	}
	if result.Run.NextActionType != domain.DunningActionTypeCollectPaymentReminder {
		t.Fatalf("expected collect_payment_reminder, got %q", result.Run.NextActionType)
	}
	if result.Event == nil || result.Event.EventType != domain.DunningEventTypeStarted {
		t.Fatalf("expected dunning_started event, got %+v", result.Event)
	}
}

func TestDunningServiceEnsureRunForInvoiceRetryDueWhenReady(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_2"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_2",
		CustomerExternalID: "cust_2",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_2"] = domain.Customer{
		ID:         "cust_row_2",
		TenantID:   "tenant_a",
		ExternalID: "cust_2",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_2"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_2",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.EnsureRunForInvoice("tenant_a", "inv_2")
	if err != nil {
		t.Fatalf("ensure run: %v", err)
	}
	if result.Run == nil {
		t.Fatalf("expected run")
	}
	if result.Run.State != domain.DunningRunStateRetryDue {
		t.Fatalf("expected retry_due, got %q", result.Run.State)
	}
	if result.Run.NextActionType != domain.DunningActionTypeRetryPayment {
		t.Fatalf("expected retry_payment, got %q", result.Run.NextActionType)
	}
}

func TestDunningServiceEnsureRunTransitionsWhenSetupBecomesReady(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_3"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_3",
		CustomerExternalID: "cust_3",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_3"] = domain.Customer{
		ID:         "cust_row_3",
		TenantID:   "tenant_a",
		ExternalID: "cust_3",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_3"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_3",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusMissing,
		DefaultPaymentMethodPresent: false,
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	first, err := svc.EnsureRunForInvoice("tenant_a", "inv_3")
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if first.Run == nil || first.Run.State != domain.DunningRunStateAwaitingPaymentSetup {
		t.Fatalf("expected initial awaiting_payment_setup state")
	}

	repo.paymentSetups["tenant_a|cust_row_3"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_3",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}
	svc.now = func() time.Time { return base.Add(2 * time.Hour) }

	second, err := svc.EnsureRunForInvoice("tenant_a", "inv_3")
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if !second.Updated || second.Run == nil {
		t.Fatalf("expected updated run")
	}
	if second.Run.State != domain.DunningRunStateRetryDue {
		t.Fatalf("expected retry_due after setup ready, got %q", second.Run.State)
	}
	if second.Event == nil || second.Event.EventType != domain.DunningEventTypePaymentSetupReady {
		t.Fatalf("expected payment_setup_ready event, got %+v", second.Event)
	}
}

func TestDunningServiceEnsureRunTransitionsPendingInvoiceIntoReprocessing(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_pending_ready"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_pending_ready",
		CustomerExternalID: "cust_pending_ready",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "pending",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_pending_ready"] = domain.Customer{
		ID:         "cust_row_pending_ready",
		TenantID:   "tenant_a",
		ExternalID: "cust_pending_ready",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_pending_ready"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_pending_ready",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusMissing,
		DefaultPaymentMethodPresent: false,
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	first, err := svc.EnsureRunForInvoice("tenant_a", "inv_pending_ready")
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if first.Run == nil || first.Run.State != domain.DunningRunStateAwaitingPaymentSetup {
		t.Fatalf("expected initial awaiting_payment_setup state")
	}

	repo.paymentSetups["tenant_a|cust_row_pending_ready"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_pending_ready",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}
	svc.now = func() time.Time { return base.Add(2 * time.Hour) }

	second, err := svc.EnsureRunForInvoice("tenant_a", "inv_pending_ready")
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if !second.Updated || second.Run == nil {
		t.Fatalf("expected updated run")
	}
	if second.Run.State != domain.DunningRunStateAwaitingRetryResult {
		t.Fatalf("expected awaiting_retry_result after payment method reprocessing, got %q", second.Run.State)
	}
	if second.Run.NextActionType != "" || second.Run.NextActionAt != nil {
		t.Fatalf("expected no scheduled next action while waiting for payment reprocessing, got %+v", second.Run)
	}
	if second.Event == nil || second.Event.EventType != domain.DunningEventTypePaymentSetupReady {
		t.Fatalf("expected payment_setup_ready event, got %+v", second.Event)
	}
}

func TestDunningServiceEnsureRunResolvesSucceededInvoice(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_4"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_4",
		CustomerExternalID: "cust_4",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_4"] = domain.Customer{
		ID:         "cust_row_4",
		TenantID:   "tenant_a",
		ExternalID: "cust_4",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_4"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_4",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	created, err := svc.EnsureRunForInvoice("tenant_a", "inv_4")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if created.Run == nil {
		t.Fatalf("expected created run")
	}

	repo.invoiceViews["tenant_a|inv_4"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_4",
		CustomerExternalID: "cust_4",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "succeeded",
		LastEventAt:        base.Add(3 * time.Hour),
	}
	svc.now = func() time.Time { return base.Add(3 * time.Hour) }

	resolved, err := svc.EnsureRunForInvoice("tenant_a", "inv_4")
	if err != nil {
		t.Fatalf("resolve run: %v", err)
	}
	if !resolved.Resolved || resolved.Run == nil {
		t.Fatalf("expected resolved run")
	}
	if resolved.Run.State != domain.DunningRunStateResolved {
		t.Fatalf("expected resolved state, got %q", resolved.Run.State)
	}
	if resolved.Run.Resolution != domain.DunningResolutionPaymentSucceeded {
		t.Fatalf("expected payment_succeeded resolution, got %q", resolved.Run.Resolution)
	}
	if resolved.Event == nil || resolved.Event.EventType != domain.DunningEventTypeResolved {
		t.Fatalf("expected resolved event, got %+v", resolved.Event)
	}
}

func TestDunningServiceEnsureRunEmitsRetrySucceededFromAwaitingRetryResult(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.invoiceViews["tenant_a|inv_retry_success"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_retry_success",
		CustomerExternalID: "cust_retry_success",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "pending",
		LastEventAt:        base,
	}
	repo.customers["tenant_a|cust_retry_success"] = domain.Customer{
		ID:         "cust_row_retry_success",
		TenantID:   "tenant_a",
		ExternalID: "cust_retry_success",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_retry_success"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_retry_success",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	created, err := svc.EnsureRunForInvoice("tenant_a", "inv_retry_success")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if created.Run == nil || created.Run.State != domain.DunningRunStateAwaitingRetryResult {
		t.Fatalf("expected awaiting_retry_result run, got %+v", created.Run)
	}

	repo.invoiceViews["tenant_a|inv_retry_success"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_retry_success",
		CustomerExternalID: "cust_retry_success",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "succeeded",
		LastEventAt:        base.Add(time.Hour),
	}
	svc.now = func() time.Time { return base.Add(time.Hour) }

	resolved, err := svc.EnsureRunForInvoice("tenant_a", "inv_retry_success")
	if err != nil {
		t.Fatalf("resolve run: %v", err)
	}
	if !resolved.Resolved || resolved.Run == nil {
		t.Fatalf("expected resolved run")
	}
	if resolved.Run.Resolution != domain.DunningResolutionPaymentSucceeded {
		t.Fatalf("expected payment_succeeded resolution, got %q", resolved.Run.Resolution)
	}
	if resolved.Event == nil || resolved.Event.EventType != domain.DunningEventTypeRetrySucceeded {
		t.Fatalf("expected retry_succeeded event, got %+v", resolved.Event)
	}
}

func TestDunningServiceQueueCollectPaymentReminderCreatesIntentAndReschedules(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_1",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d", "2d", "5d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_5"] = domain.Customer{
		ID:         "cust_row_5",
		TenantID:   "tenant_a",
		ExternalID: "cust_5",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.activeRuns["tenant_a|inv_5"] = domain.InvoiceDunningRun{
		ID:                 "dru_5",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_5",
		CustomerExternalID: "cust_5",
		PolicyID:           "dpo_1",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.runsByID["dru_5"] = repo.activeRuns["tenant_a|inv_5"]

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.QueueCollectPaymentReminder("tenant_a", "dru_5")
	if err != nil {
		t.Fatalf("queue collect payment reminder: %v", err)
	}
	if result.NotificationIntent.IntentType != domain.DunningNotificationIntentTypePaymentMethodRequired {
		t.Fatalf("expected payment_method_required intent, got %q", result.NotificationIntent.IntentType)
	}
	if result.NotificationIntent.Status != domain.DunningNotificationIntentStatusQueued {
		t.Fatalf("expected queued status, got %q", result.NotificationIntent.Status)
	}
	if result.Run.NextActionAt == nil || !result.Run.NextActionAt.Equal(base.Add(48*time.Hour)) {
		t.Fatalf("expected next action at +48h, got %v", result.Run.NextActionAt)
	}
	if result.Event.EventType != domain.DunningEventTypePaymentSetupPending {
		t.Fatalf("expected payment_setup_pending event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceQueueCollectPaymentReminderEscalatesWhenScheduleExhausted(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_2",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_6"] = domain.Customer{
		ID:         "cust_row_6",
		TenantID:   "tenant_a",
		ExternalID: "cust_6",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.activeRuns["tenant_a|inv_6"] = domain.InvoiceDunningRun{
		ID:                 "dru_6",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_6",
		CustomerExternalID: "cust_6",
		PolicyID:           "dpo_2",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.runsByID["dru_6"] = repo.activeRuns["tenant_a|inv_6"]

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.QueueCollectPaymentReminder("tenant_a", "dru_6")
	if err != nil {
		t.Fatalf("queue collect payment reminder: %v", err)
	}
	if !result.Escalated {
		t.Fatalf("expected escalated result")
	}
	if result.Run.State != domain.DunningRunStateEscalated {
		t.Fatalf("expected escalated run state, got %q", result.Run.State)
	}
	if result.Event.EventType != domain.DunningEventTypeEscalated {
		t.Fatalf("expected escalated event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceGetInvoiceSummaryFallsBackToLatestResolvedRun(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	resolvedAt := base.Add(2 * time.Hour)
	run := domain.InvoiceDunningRun{
		ID:                 "dru_resolved_summary",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_resolved_summary",
		CustomerExternalID: "cust_resolved_summary",
		PolicyID:           "dpo_resolved_summary",
		State:              domain.DunningRunStateResolved,
		Reason:             "payment_succeeded",
		AttemptCount:       1,
		ResolvedAt:         &resolvedAt,
		Resolution:         domain.DunningResolutionPaymentSucceeded,
		CreatedAt:          base,
		UpdatedAt:          resolvedAt,
	}
	repo.runsByID[run.ID] = run
	repo.eventsByRunID[run.ID] = []domain.InvoiceDunningEvent{
		{
			ID:        "dne_resolved_1",
			RunID:     run.ID,
			TenantID:  "tenant_a",
			InvoiceID: run.InvoiceID,
			EventType: domain.DunningEventTypeRetrySucceeded,
			State:     run.State,
			CreatedAt: resolvedAt,
		},
	}

	svc, _ := NewDunningService(repo)
	summary, err := svc.GetInvoiceSummary("tenant_a", run.InvoiceID)
	if err != nil {
		t.Fatalf("get invoice summary: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected dunning summary")
	}
	if summary.RunID != run.ID {
		t.Fatalf("expected run %q, got %q", run.ID, summary.RunID)
	}
	if summary.State != domain.DunningRunStateResolved {
		t.Fatalf("expected resolved state, got %q", summary.State)
	}
	if summary.Resolution != domain.DunningResolutionPaymentSucceeded {
		t.Fatalf("expected payment_succeeded resolution, got %q", summary.Resolution)
	}
	if summary.LastEventType != domain.DunningEventTypeRetrySucceeded {
		t.Fatalf("expected retry_succeeded last event, got %q", summary.LastEventType)
	}
}

func TestDunningServiceGetInvoiceSummaryIncludesLatestEventAndIntent(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	run := domain.InvoiceDunningRun{
		ID:                 "dru_7",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_7",
		CustomerExternalID: "cust_7",
		PolicyID:           "dpo_7",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		Reason:             "payment_setup_pending",
		AttemptCount:       1,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(2 * time.Hour)),
		CreatedAt:          base,
		UpdatedAt:          base,
	}
	repo.activeRuns["tenant_a|inv_7"] = run
	repo.runsByID[run.ID] = run
	repo.eventsByRunID[run.ID] = []domain.InvoiceDunningEvent{
		{
			ID:        "dne_1",
			RunID:     run.ID,
			TenantID:  "tenant_a",
			InvoiceID: "inv_7",
			EventType: domain.DunningEventTypeStarted,
			State:     run.State,
			CreatedAt: base,
		},
		{
			ID:        "dne_2",
			RunID:     run.ID,
			TenantID:  "tenant_a",
			InvoiceID: "inv_7",
			EventType: domain.DunningEventTypeNotificationSent,
			State:     run.State,
			CreatedAt: base.Add(30 * time.Minute),
		},
	}
	repo.intentsByRunID[run.ID] = []domain.DunningNotificationIntent{
		{
			ID:           "dni_1",
			RunID:        run.ID,
			TenantID:     "tenant_a",
			InvoiceID:    "inv_7",
			IntentType:   domain.DunningNotificationIntentTypePaymentMethodRequired,
			ActionType:   domain.DunningActionTypeCollectPaymentReminder,
			Status:       domain.DunningNotificationIntentStatusDispatched,
			CreatedAt:    base,
			DispatchedAt: ptrTime(base.Add(20 * time.Minute)),
		},
	}

	svc, _ := NewDunningService(repo)
	summary, err := svc.GetInvoiceSummary("tenant_a", "inv_7")
	if err != nil {
		t.Fatalf("get invoice summary: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected dunning summary")
	}
	if summary.LastEventType != domain.DunningEventTypeNotificationSent {
		t.Fatalf("expected latest event notification_sent, got %q", summary.LastEventType)
	}
	if summary.LastNotificationStatus != domain.DunningNotificationIntentStatusDispatched {
		t.Fatalf("expected dispatched notification status, got %q", summary.LastNotificationStatus)
	}
}

func TestDunningServiceListRunsFiltersActiveStateAndCustomer(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	active := domain.InvoiceDunningRun{
		ID:                 "dru_active",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_active",
		CustomerExternalID: "cust_match",
		PolicyID:           "dpo_1",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		CreatedAt:          base,
		UpdatedAt:          base,
	}
	resolvedAt := base.Add(time.Hour)
	resolved := domain.InvoiceDunningRun{
		ID:                 "dru_resolved",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_resolved",
		CustomerExternalID: "cust_other",
		PolicyID:           "dpo_1",
		State:              domain.DunningRunStateResolved,
		ResolvedAt:         &resolvedAt,
		Resolution:         domain.DunningResolutionInvoiceNotCollectible,
		CreatedAt:          base.Add(2 * time.Hour),
		UpdatedAt:          base.Add(2 * time.Hour),
	}
	repo.activeRuns["tenant_a|inv_active"] = active
	repo.runsByID[active.ID] = active
	repo.runsByID[resolved.ID] = resolved

	svc, _ := NewDunningService(repo)

	items, err := svc.ListRuns("tenant_a", ListDunningRunsRequest{
		CustomerExternalID: "cust_match",
		ActiveOnly:         true,
		Limit:              20,
	})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(items) != 1 || items[0].ID != active.ID {
		t.Fatalf("expected only active matching run, got %+v", items)
	}

	items, err = svc.ListRuns("tenant_a", ListDunningRunsRequest{
		State:      "resolved",
		ActiveOnly: false,
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("list runs by state: %v", err)
	}
	if len(items) != 1 || items[0].ID != resolved.ID {
		t.Fatalf("expected resolved run, got %+v", items)
	}
}

func TestDunningServiceGetRunDetailIncludesEventsAndIntents(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	run := domain.InvoiceDunningRun{
		ID:                 "dru_detail",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_detail",
		CustomerExternalID: "cust_detail",
		PolicyID:           "dpo_detail",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		CreatedAt:          base,
		UpdatedAt:          base,
	}
	repo.activeRuns["tenant_a|inv_detail"] = run
	repo.runsByID[run.ID] = run
	repo.eventsByRunID[run.ID] = []domain.InvoiceDunningEvent{{
		ID:        "dne_detail",
		RunID:     run.ID,
		TenantID:  "tenant_a",
		InvoiceID: run.InvoiceID,
		EventType: domain.DunningEventTypeStarted,
		State:     run.State,
		CreatedAt: base,
	}}
	repo.intentsByRunID[run.ID] = []domain.DunningNotificationIntent{{
		ID:         "dni_detail",
		RunID:      run.ID,
		TenantID:   "tenant_a",
		InvoiceID:  run.InvoiceID,
		IntentType: domain.DunningNotificationIntentTypePaymentMethodRequired,
		ActionType: domain.DunningActionTypeCollectPaymentReminder,
		Status:     domain.DunningNotificationIntentStatusQueued,
		CreatedAt:  base,
	}}

	svc, _ := NewDunningService(repo)
	detail, err := svc.GetRunDetail("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("get run detail: %v", err)
	}
	if detail.Run.ID != run.ID {
		t.Fatalf("expected run %q, got %q", run.ID, detail.Run.ID)
	}
	if len(detail.Events) != 1 || detail.Events[0].ID != "dne_detail" {
		t.Fatalf("expected one event, got %+v", detail.Events)
	}
	if len(detail.NotificationIntents) != 1 || detail.NotificationIntents[0].ID != "dni_detail" {
		t.Fatalf("expected one notification intent, got %+v", detail.NotificationIntents)
	}
}

func TestDunningServiceProcessNextCollectPaymentReminderDispatchesIntent(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_8",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_8"] = domain.Customer{
		ID:         "cust_row_8",
		TenantID:   "tenant_a",
		ExternalID: "cust_8",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_8"] = domain.CustomerPaymentSetup{
		CustomerID:        "cust_row_8",
		TenantID:          "tenant_a",
		PaymentMethodType: "card",
	}
	repo.activeRuns["tenant_a|inv_8"] = domain.InvoiceDunningRun{
		ID:                 "dru_8",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_8",
		CustomerExternalID: "cust_8",
		PolicyID:           "dpo_8",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.runsByID["dru_8"] = repo.activeRuns["tenant_a|inv_8"]

	sender := &fakeDunningPaymentSetupSender{
		result: CustomerPaymentSetupRequestResult{
			Action: "resent",
			PaymentSetup: domain.CustomerPaymentSetup{
				LastRequestToEmail: "customer@example.com",
			},
			Dispatch: NotificationDispatchResult{
				Backend:      "alpha_email",
				DispatchedAt: base.Add(time.Minute),
			},
		},
	}

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithPaymentSetupRequestSender(sender)

	processed, result, err := svc.ProcessNextCollectPaymentReminder("tenant_a")
	if err != nil {
		t.Fatalf("process next collect payment reminder: %v", err)
	}
	if !processed || result == nil {
		t.Fatalf("expected processed result")
	}
	if sender.calls != 1 {
		t.Fatalf("expected one resend call, got %d", sender.calls)
	}
	if result.NotificationIntent.Status != domain.DunningNotificationIntentStatusDispatched {
		t.Fatalf("expected dispatched intent status, got %q", result.NotificationIntent.Status)
	}
	if result.DispatchEvent == nil || result.DispatchEvent.EventType != domain.DunningEventTypeNotificationSent {
		t.Fatalf("expected notification_sent dispatch event, got %+v", result.DispatchEvent)
	}
}

func TestDunningServiceProcessNextCollectPaymentReminderMarksDispatchFailure(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_9",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_9"] = domain.Customer{
		ID:         "cust_row_9",
		TenantID:   "tenant_a",
		ExternalID: "cust_9",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.activeRuns["tenant_a|inv_9"] = domain.InvoiceDunningRun{
		ID:                 "dru_9",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_9",
		CustomerExternalID: "cust_9",
		PolicyID:           "dpo_9",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.runsByID["dru_9"] = repo.activeRuns["tenant_a|inv_9"]

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithPaymentSetupRequestSender(&fakeDunningPaymentSetupSender{err: errors.New("smtp down")})

	processed, result, err := svc.ProcessNextCollectPaymentReminder("tenant_a")
	if !processed || result == nil {
		t.Fatalf("expected processed result")
	}
	if err == nil {
		t.Fatalf("expected dispatch error")
	}
	if result.NotificationIntent.Status != domain.DunningNotificationIntentStatusFailed {
		t.Fatalf("expected failed intent status, got %q", result.NotificationIntent.Status)
	}
	if result.DispatchEvent == nil || result.DispatchEvent.EventType != domain.DunningEventTypeNotificationFailed {
		t.Fatalf("expected notification_failed dispatch event, got %+v", result.DispatchEvent)
	}
}

func TestDunningServiceDispatchCollectPaymentReminderIgnoresDueTime(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_dispatch",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_dispatch"] = domain.Customer{
		ID:         "cust_row_dispatch",
		TenantID:   "tenant_a",
		ExternalID: "cust_dispatch",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_dispatch"] = domain.CustomerPaymentSetup{
		CustomerID:        "cust_row_dispatch",
		TenantID:          "tenant_a",
		PaymentMethodType: "card",
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_dispatch",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_dispatch",
		CustomerExternalID: "cust_dispatch",
		PolicyID:           "dpo_dispatch",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(4 * time.Hour)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_dispatch"] = run
	repo.runsByID[run.ID] = run

	sender := &fakeDunningPaymentSetupSender{
		result: CustomerPaymentSetupRequestResult{
			Action: "resent",
			PaymentSetup: domain.CustomerPaymentSetup{
				LastRequestToEmail: "customer@example.com",
			},
			Dispatch: NotificationDispatchResult{
				Backend:      "alpha_email",
				DispatchedAt: base.Add(time.Minute),
			},
		},
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithPaymentSetupRequestSender(sender)

	result, err := svc.DispatchCollectPaymentReminder("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("dispatch collect payment reminder: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("expected one resend call, got %d", sender.calls)
	}
	if result.NotificationIntent.Status != domain.DunningNotificationIntentStatusDispatched {
		t.Fatalf("expected dispatched intent status, got %q", result.NotificationIntent.Status)
	}
	if result.DispatchEvent == nil || result.DispatchEvent.EventType != domain.DunningEventTypeNotificationSent {
		t.Fatalf("expected notification_sent dispatch event, got %+v", result.DispatchEvent)
	}
}

func TestDunningServiceProcessCollectPaymentReminderBatch(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:                             "dpo_batch",
		TenantID:                       "tenant_a",
		Name:                           "Default dunning policy",
		Enabled:                        true,
		CollectPaymentReminderSchedule: []string{"0d", "2d"},
		RetrySchedule:                  []string{"1d"},
		MaxRetryAttempts:               3,
		FinalAction:                    domain.DunningFinalActionManualReview,
	}
	repo.customers["tenant_a|cust_batch"] = domain.Customer{
		ID:         "cust_row_batch",
		TenantID:   "tenant_a",
		ExternalID: "cust_batch",
		Email:      "customer@example.com",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_batch"] = domain.CustomerPaymentSetup{
		CustomerID:        "cust_row_batch",
		TenantID:          "tenant_a",
		PaymentMethodType: "card",
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_batch",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_batch",
		CustomerExternalID: "cust_batch",
		PolicyID:           "dpo_batch",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_batch"] = run
	repo.runsByID[run.ID] = run

	sender := &fakeDunningPaymentSetupSender{
		result: CustomerPaymentSetupRequestResult{
			Action: "resent",
			PaymentSetup: domain.CustomerPaymentSetup{
				LastRequestToEmail: "customer@example.com",
			},
			Dispatch: NotificationDispatchResult{
				Backend:      "alpha_email",
				DispatchedAt: base.Add(time.Minute),
			},
		},
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithPaymentSetupRequestSender(sender)

	result, err := svc.ProcessCollectPaymentReminderBatch("tenant_a", 5)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("expected processed=1, got %d", result.Processed)
	}
	if result.Dispatched != 1 {
		t.Fatalf("expected dispatched=1, got %d", result.Dispatched)
	}
	if result.Failed != 0 {
		t.Fatalf("expected failed=0, got %d", result.Failed)
	}
	if result.LastRunID != run.ID {
		t.Fatalf("expected last_run_id %q, got %q", run.ID, result.LastRunID)
	}
}

func TestDunningServicePauseRun(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	run := domain.InvoiceDunningRun{
		ID:                 "dru_pause",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_pause",
		CustomerExternalID: "cust_pause",
		PolicyID:           "dpo_pause",
		State:              domain.DunningRunStateRetryDue,
		Reason:             "payment_setup_ready",
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(base.Add(time.Hour)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_pause"] = run
	repo.runsByID[run.ID] = run

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.PauseRun("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("pause run: %v", err)
	}
	if !result.Run.Paused || result.Run.State != domain.DunningRunStatePaused {
		t.Fatalf("expected paused run, got %+v", result.Run)
	}
	if result.Event.EventType != domain.DunningEventTypePaused {
		t.Fatalf("expected paused event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceResumeRun(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:               "dpo_resume",
		TenantID:         "tenant_a",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
	}
	repo.invoiceViews["tenant_a|inv_resume"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_resume",
		CustomerExternalID: "cust_resume",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventType:      "invoice.payment_failure",
		LastEventAt:        base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.customers["tenant_a|cust_resume"] = domain.Customer{
		ID:         "cust_row_resume",
		TenantID:   "tenant_a",
		ExternalID: "cust_resume",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_resume"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_resume",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_resume",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_resume",
		CustomerExternalID: "cust_resume",
		PolicyID:           "dpo_resume",
		State:              domain.DunningRunStatePaused,
		Reason:             "paused_by_operator",
		Paused:             true,
		CreatedAt:          base.Add(-2 * time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_resume"] = run
	repo.runsByID[run.ID] = run

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.ResumeRun("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if result.Run.Paused {
		t.Fatalf("expected run to be unpaused")
	}
	if result.Run.State != domain.DunningRunStateRetryDue {
		t.Fatalf("expected retry_due state, got %q", result.Run.State)
	}
	if result.Event.EventType != domain.DunningEventTypeResumed {
		t.Fatalf("expected resumed event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceResolveRun(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	run := domain.InvoiceDunningRun{
		ID:                 "dru_resolve",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_resolve",
		CustomerExternalID: "cust_resolve",
		PolicyID:           "dpo_resolve",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		Reason:             "payment_setup_pending",
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		CreatedAt:          base.Add(-2 * time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_resolve"] = run
	repo.runsByID[run.ID] = run

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	result, err := svc.ResolveRun("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("resolve run: %v", err)
	}
	if result.Run.State != domain.DunningRunStateResolved {
		t.Fatalf("expected resolved state, got %q", result.Run.State)
	}
	if result.Run.Resolution != domain.DunningResolutionOperatorResolved {
		t.Fatalf("expected operator_resolved, got %q", result.Run.Resolution)
	}
	if result.Event.EventType != domain.DunningEventTypeResolved {
		t.Fatalf("expected resolved event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceRefreshRunsForCustomer(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 12, 30, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:               "dpo_refresh_customer",
		TenantID:         "tenant_a",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
	}
	repo.invoiceViews["tenant_a|inv_refresh_customer"] = domain.InvoicePaymentStatusView{
		TenantID:           "tenant_a",
		InvoiceID:          "inv_refresh_customer",
		CustomerExternalID: "cust_refresh_customer",
		InvoiceStatus:      "finalized",
		PaymentStatus:      "failed",
		LastEventType:      "invoice.payment_failure",
		LastEventAt:        base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.customers["tenant_a|cust_refresh_customer"] = domain.Customer{
		ID:         "cust_row_refresh_customer",
		TenantID:   "tenant_a",
		ExternalID: "cust_refresh_customer",
		Status:     domain.CustomerStatusActive,
	}
	repo.paymentSetups["tenant_a|cust_row_refresh_customer"] = domain.CustomerPaymentSetup{
		CustomerID:                  "cust_row_refresh_customer",
		TenantID:                    "tenant_a",
		SetupStatus:                 domain.PaymentSetupStatusReady,
		DefaultPaymentMethodPresent: true,
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_refresh_customer",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_refresh_customer",
		CustomerExternalID: "cust_refresh_customer",
		PolicyID:           "dpo_refresh_customer",
		State:              domain.DunningRunStateAwaitingPaymentSetup,
		Reason:             "payment_setup_pending",
		NextActionType:     domain.DunningActionTypeCollectPaymentReminder,
		NextActionAt:       ptrTime(base.Add(time.Hour)),
		CreatedAt:          base.Add(-2 * time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_refresh_customer"] = run
	repo.runsByID[run.ID] = run

	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }

	results, err := svc.RefreshRunsForCustomer("tenant_a", "cust_refresh_customer")
	if err != nil {
		t.Fatalf("refresh runs for customer: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one refreshed run, got %d", len(results))
	}
	if results[0].Run == nil || results[0].Run.State != domain.DunningRunStateRetryDue {
		t.Fatalf("expected run to move to retry_due, got %+v", results[0].Run)
	}
}

func TestDunningServiceDispatchRetryPaymentMovesRunToAwaitingResult(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:               "dpo_retry",
		TenantID:         "tenant_a",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_retry",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_retry",
		CustomerExternalID: "cust_retry",
		PolicyID:           "dpo_retry",
		State:              domain.DunningRunStateRetryDue,
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_retry"] = run
	repo.runsByID[run.ID] = run

	retrier := &fakeDunningInvoiceRetryExecutor{
		statusCode: 200,
		body:       []byte(`{"status":"queued"}`),
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithInvoiceRetryExecutor(retrier)

	result, err := svc.DispatchRetryPayment("tenant_a", run.ID)
	if err != nil {
		t.Fatalf("dispatch retry payment: %v", err)
	}
	if retrier.calls != 1 {
		t.Fatalf("expected one retry call, got %d", retrier.calls)
	}
	if result.Run.State != domain.DunningRunStateAwaitingRetryResult {
		t.Fatalf("expected awaiting_retry_result, got %q", result.Run.State)
	}
	if result.Run.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", result.Run.AttemptCount)
	}
	if result.Event.EventType != domain.DunningEventTypeRetryAttempted {
		t.Fatalf("expected retry_attempted event, got %q", result.Event.EventType)
	}
}

func TestDunningServiceDispatchRetryPaymentReschedulesFailure(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:               "dpo_retry_fail",
		TenantID:         "tenant_a",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_retry_fail",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_retry_fail",
		CustomerExternalID: "cust_retry_fail",
		PolicyID:           "dpo_retry_fail",
		State:              domain.DunningRunStateRetryDue,
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_retry_fail"] = run
	repo.runsByID[run.ID] = run

	retrier := &fakeDunningInvoiceRetryExecutor{
		statusCode: 502,
		body:       []byte(`{"error":"gateway"}`),
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithInvoiceRetryExecutor(retrier)

	result, err := svc.DispatchRetryPayment("tenant_a", run.ID)
	if err == nil {
		t.Fatalf("expected retry failure error")
	}
	if result.Run.State != domain.DunningRunStateScheduled {
		t.Fatalf("expected scheduled state, got %q", result.Run.State)
	}
	if result.Run.NextActionType != domain.DunningActionTypeRetryPayment {
		t.Fatalf("expected retry_payment next action, got %q", result.Run.NextActionType)
	}
	if result.Event.EventType != domain.DunningEventTypeRetryFailed {
		t.Fatalf("expected retry_failed event, got %q", result.Event.EventType)
	}
	if result.Run.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", result.Run.AttemptCount)
	}
}

func TestDunningServiceProcessRetryPaymentBatch(t *testing.T) {
	t.Parallel()

	repo := newFakeDunningStore()
	base := time.Date(2026, 3, 22, 11, 0, 0, 0, time.UTC)
	repo.policies["tenant_a"] = domain.DunningPolicy{
		ID:               "dpo_retry_batch",
		TenantID:         "tenant_a",
		Name:             "Default dunning policy",
		Enabled:          true,
		RetrySchedule:    []string{"1d", "3d"},
		MaxRetryAttempts: 3,
		FinalAction:      domain.DunningFinalActionManualReview,
	}
	run := domain.InvoiceDunningRun{
		ID:                 "dru_retry_batch",
		TenantID:           "tenant_a",
		InvoiceID:          "inv_retry_batch",
		CustomerExternalID: "cust_retry_batch",
		PolicyID:           "dpo_retry_batch",
		State:              domain.DunningRunStateRetryDue,
		NextActionType:     domain.DunningActionTypeRetryPayment,
		NextActionAt:       ptrTime(base.Add(-time.Minute)),
		CreatedAt:          base.Add(-time.Hour),
		UpdatedAt:          base.Add(-time.Hour),
	}
	repo.activeRuns["tenant_a|inv_retry_batch"] = run
	repo.runsByID[run.ID] = run

	retrier := &fakeDunningInvoiceRetryExecutor{
		statusCode: 200,
		body:       []byte(`{"status":"queued"}`),
	}
	svc, _ := NewDunningService(repo)
	svc.now = func() time.Time { return base }
	svc.WithInvoiceRetryExecutor(retrier)

	result, err := svc.ProcessRetryPaymentBatch("tenant_a", 5)
	if err != nil {
		t.Fatalf("process retry batch: %v", err)
	}
	if result.Processed != 1 {
		t.Fatalf("expected processed=1, got %d", result.Processed)
	}
	if result.Dispatched != 1 {
		t.Fatalf("expected dispatched=1, got %d", result.Dispatched)
	}
	if result.Failed != 0 {
		t.Fatalf("expected failed=0, got %d", result.Failed)
	}
	if result.LastRunID != run.ID {
		t.Fatalf("expected last_run_id %q, got %q", run.ID, result.LastRunID)
	}
}

type fakeDunningStore struct {
	policies       map[string]domain.DunningPolicy
	invoiceViews   map[string]domain.InvoicePaymentStatusView
	customers      map[string]domain.Customer
	paymentSetups  map[string]domain.CustomerPaymentSetup
	activeRuns     map[string]domain.InvoiceDunningRun
	runsByID       map[string]domain.InvoiceDunningRun
	eventsByRunID  map[string][]domain.InvoiceDunningEvent
	intentsByRunID map[string][]domain.DunningNotificationIntent
}

func newFakeDunningStore() *fakeDunningStore {
	return &fakeDunningStore{
		policies:       map[string]domain.DunningPolicy{},
		invoiceViews:   map[string]domain.InvoicePaymentStatusView{},
		customers:      map[string]domain.Customer{},
		paymentSetups:  map[string]domain.CustomerPaymentSetup{},
		activeRuns:     map[string]domain.InvoiceDunningRun{},
		runsByID:       map[string]domain.InvoiceDunningRun{},
		eventsByRunID:  map[string][]domain.InvoiceDunningEvent{},
		intentsByRunID: map[string][]domain.DunningNotificationIntent{},
	}
}

func (f *fakeDunningStore) GetDunningPolicy(tenantID string) (domain.DunningPolicy, error) {
	item, ok := f.policies[tenantID]
	if !ok {
		return domain.DunningPolicy{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) UpsertDunningPolicy(input domain.DunningPolicy) (domain.DunningPolicy, error) {
	if input.ID == "" {
		input.ID = "dpo_test"
	}
	f.policies[input.TenantID] = input
	return input, nil
}

func (f *fakeDunningStore) GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error) {
	item, ok := f.invoiceViews[tenantID+"|"+invoiceID]
	if !ok {
		return domain.InvoicePaymentStatusView{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error) {
	item, ok := f.customers[tenantID+"|"+externalID]
	if !ok {
		return domain.Customer{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) GetCustomerPaymentSetup(tenantID, customerID string) (domain.CustomerPaymentSetup, error) {
	item, ok := f.paymentSetups[tenantID+"|"+customerID]
	if !ok {
		return domain.CustomerPaymentSetup{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) GetActiveInvoiceDunningRunByInvoiceID(tenantID, invoiceID string) (domain.InvoiceDunningRun, error) {
	item, ok := f.activeRuns[tenantID+"|"+invoiceID]
	if !ok {
		return domain.InvoiceDunningRun{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) CreateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error) {
	key := input.TenantID + "|" + input.InvoiceID
	if _, ok := f.activeRuns[key]; ok {
		return domain.InvoiceDunningRun{}, store.ErrAlreadyExists
	}
	if input.ID == "" {
		input.ID = "dru_test_" + input.InvoiceID
	}
	f.activeRuns[key] = input
	f.runsByID[input.ID] = input
	return input, nil
}

func (f *fakeDunningStore) UpdateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error) {
	key := input.TenantID + "|" + input.InvoiceID
	if _, ok := f.activeRuns[key]; !ok {
		return domain.InvoiceDunningRun{}, store.ErrNotFound
	}
	if input.ResolvedAt != nil {
		delete(f.activeRuns, key)
	} else {
		f.activeRuns[key] = input
	}
	f.runsByID[input.ID] = input
	return input, nil
}

func (f *fakeDunningStore) GetInvoiceDunningRun(tenantID, id string) (domain.InvoiceDunningRun, error) {
	item, ok := f.runsByID[id]
	if !ok || item.TenantID != tenantID {
		return domain.InvoiceDunningRun{}, store.ErrNotFound
	}
	return item, nil
}

func (f *fakeDunningStore) ListInvoiceDunningRuns(filter store.InvoiceDunningRunListFilter) ([]domain.InvoiceDunningRun, error) {
	items := make([]domain.InvoiceDunningRun, 0)
	for _, item := range f.runsByID {
		if item.TenantID != filter.TenantID {
			continue
		}
		if invoiceID := strings.TrimSpace(filter.InvoiceID); invoiceID != "" && item.InvoiceID != invoiceID {
			continue
		}
		if customerExternalID := strings.TrimSpace(filter.CustomerExternalID); customerExternalID != "" && item.CustomerExternalID != customerExternalID {
			continue
		}
		if state := strings.TrimSpace(filter.State); state != "" && strings.ToLower(string(item.State)) != strings.ToLower(state) {
			continue
		}
		if filter.ActiveOnly && item.ResolvedAt != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (f *fakeDunningStore) ListDueInvoiceDunningRuns(filter store.DueInvoiceDunningRunFilter) ([]domain.InvoiceDunningRun, error) {
	items := make([]domain.InvoiceDunningRun, 0)
	for _, item := range f.activeRuns {
		if item.TenantID != filter.TenantID {
			continue
		}
		if filter.ActionType != "" && string(item.NextActionType) != filter.ActionType {
			continue
		}
		if item.NextActionAt == nil || item.NextActionAt.After(filter.DueBefore) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (f *fakeDunningStore) CreateInvoiceDunningEvent(input domain.InvoiceDunningEvent) (domain.InvoiceDunningEvent, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("dne_%d", len(f.eventsByRunID[input.RunID])+1)
	}
	f.eventsByRunID[input.RunID] = append(f.eventsByRunID[input.RunID], input)
	return input, nil
}

func (f *fakeDunningStore) ListInvoiceDunningEvents(tenantID, runID string) ([]domain.InvoiceDunningEvent, error) {
	return append([]domain.InvoiceDunningEvent(nil), f.eventsByRunID[runID]...), nil
}

func (f *fakeDunningStore) CreateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error) {
	if input.ID == "" {
		input.ID = fmt.Sprintf("dni_%d", len(f.intentsByRunID[input.RunID])+1)
	}
	f.intentsByRunID[input.RunID] = append(f.intentsByRunID[input.RunID], input)
	return input, nil
}

func (f *fakeDunningStore) UpdateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error) {
	items := f.intentsByRunID[input.RunID]
	for i := range items {
		if items[i].ID == input.ID {
			items[i] = input
			f.intentsByRunID[input.RunID] = items
			return input, nil
		}
	}
	return domain.DunningNotificationIntent{}, store.ErrNotFound
}

func (f *fakeDunningStore) ListDunningNotificationIntents(filter store.DunningNotificationIntentListFilter) ([]domain.DunningNotificationIntent, error) {
	items := append([]domain.DunningNotificationIntent(nil), f.intentsByRunID[filter.RunID]...)
	return items, nil
}

var _ dunningStore = (*fakeDunningStore)(nil)

type fakeDunningPaymentSetupSender struct {
	result CustomerPaymentSetupRequestResult
	err    error
	calls  int
}

func (f *fakeDunningPaymentSetupSender) Resend(tenantID, externalID string, actor CustomerPaymentSetupRequestActor, paymentMethodType string) (CustomerPaymentSetupRequestResult, error) {
	f.calls++
	return f.result, f.err
}

type fakeDunningInvoiceRetryExecutor struct {
	statusCode int
	body       []byte
	err        error
	calls      int
}

func (f *fakeDunningInvoiceRetryExecutor) RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error) {
	f.calls++
	return f.statusCode, f.body, f.err
}

func TestParseDunningDelayRejectsInvalid(t *testing.T) {
	t.Parallel()
	if _, err := parseDunningDelay("soon"); err == nil {
		t.Fatalf("expected invalid delay error")
	}
	if _, err := parseDunningDelay(""); err == nil {
		t.Fatalf("expected empty delay error")
	}
}
