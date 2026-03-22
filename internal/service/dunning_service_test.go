package service

import (
	"fmt"
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
	if resolved.Run.Resolution != domain.DunningResolutionInvoiceNotCollectible {
		t.Fatalf("expected invoice_not_collectible resolution, got %q", resolved.Run.Resolution)
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
	panic("not used in tests")
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

func (f *fakeDunningStore) ListDunningNotificationIntents(filter store.DunningNotificationIntentListFilter) ([]domain.DunningNotificationIntent, error) {
	items := append([]domain.DunningNotificationIntent(nil), f.intentsByRunID[filter.RunID]...)
	return items, nil
}

var _ dunningStore = (*fakeDunningStore)(nil)

func TestParseDunningDelayRejectsInvalid(t *testing.T) {
	t.Parallel()
	if _, err := parseDunningDelay("soon"); err == nil {
		t.Fatalf("expected invalid delay error")
	}
	if _, err := parseDunningDelay(""); err == nil {
		t.Fatalf("expected empty delay error")
	}
}
