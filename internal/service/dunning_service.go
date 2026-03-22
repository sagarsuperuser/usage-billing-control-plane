package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type dunningStore interface {
	GetDunningPolicy(tenantID string) (domain.DunningPolicy, error)
	UpsertDunningPolicy(input domain.DunningPolicy) (domain.DunningPolicy, error)
	GetInvoicePaymentStatusView(tenantID, invoiceID string) (domain.InvoicePaymentStatusView, error)
	GetCustomerByExternalID(tenantID, externalID string) (domain.Customer, error)
	GetCustomerPaymentSetup(tenantID, customerID string) (domain.CustomerPaymentSetup, error)
	GetActiveInvoiceDunningRunByInvoiceID(tenantID, invoiceID string) (domain.InvoiceDunningRun, error)
	CreateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error)
	UpdateInvoiceDunningRun(input domain.InvoiceDunningRun) (domain.InvoiceDunningRun, error)
	GetInvoiceDunningRun(tenantID, id string) (domain.InvoiceDunningRun, error)
	ListDueInvoiceDunningRuns(filter store.DueInvoiceDunningRunFilter) ([]domain.InvoiceDunningRun, error)
	CreateInvoiceDunningEvent(input domain.InvoiceDunningEvent) (domain.InvoiceDunningEvent, error)
	ListInvoiceDunningEvents(tenantID, runID string) ([]domain.InvoiceDunningEvent, error)
	CreateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error)
	ListDunningNotificationIntents(filter store.DunningNotificationIntentListFilter) ([]domain.DunningNotificationIntent, error)
}

type DunningService struct {
	store dunningStore
	now   func() time.Time
}

type PutDunningPolicyRequest struct {
	Name                           string                    `json:"name"`
	Enabled                        *bool                     `json:"enabled,omitempty"`
	RetrySchedule                  []string                  `json:"retry_schedule"`
	MaxRetryAttempts               int                       `json:"max_retry_attempts"`
	CollectPaymentReminderSchedule []string                  `json:"collect_payment_reminder_schedule"`
	FinalAction                    domain.DunningFinalAction `json:"final_action"`
	GracePeriodDays                int                       `json:"grace_period_days"`
}

type EnsureInvoiceDunningRunResult struct {
	Policy   domain.DunningPolicy        `json:"policy"`
	Run      *domain.InvoiceDunningRun   `json:"run,omitempty"`
	Event    *domain.InvoiceDunningEvent `json:"event,omitempty"`
	Eligible bool                        `json:"eligible"`
	Created  bool                        `json:"created"`
	Updated  bool                        `json:"updated"`
	Resolved bool                        `json:"resolved"`
	Reason   string                      `json:"reason,omitempty"`
}

type ListDueDunningRunsRequest struct {
	ActionType domain.DunningActionType `json:"action_type,omitempty"`
	DueBefore  *time.Time               `json:"due_before,omitempty"`
	Limit      int                      `json:"limit,omitempty"`
}

type QueueCollectPaymentReminderResult struct {
	Policy             domain.DunningPolicy             `json:"policy"`
	Run                domain.InvoiceDunningRun         `json:"run"`
	Event              domain.InvoiceDunningEvent       `json:"event"`
	NotificationIntent domain.DunningNotificationIntent `json:"notification_intent"`
	Escalated          bool                             `json:"escalated"`
}

func NewDunningService(s dunningStore) (*DunningService, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	return &DunningService{
		store: s,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}, nil
}

func (s *DunningService) GetPolicy(tenantID string) (domain.DunningPolicy, error) {
	if s == nil || s.store == nil {
		return domain.DunningPolicy{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	policy, err := s.store.GetDunningPolicy(tenantID)
	if err == nil {
		return policy, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return domain.DunningPolicy{}, err
	}
	return s.store.UpsertDunningPolicy(defaultDunningPolicy(tenantID, s.now()))
}

func (s *DunningService) PutPolicy(tenantID string, req PutDunningPolicyRequest) (domain.DunningPolicy, error) {
	policy, err := s.GetPolicy(tenantID)
	if err != nil {
		return domain.DunningPolicy{}, err
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		policy.Name = name
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	if req.RetrySchedule != nil {
		if err := validateDunningSchedule(req.RetrySchedule); err != nil {
			return domain.DunningPolicy{}, err
		}
		policy.RetrySchedule = normalizeSchedule(req.RetrySchedule)
	}
	if req.MaxRetryAttempts < 0 {
		return domain.DunningPolicy{}, fmt.Errorf("%w: max_retry_attempts must be >= 0", ErrValidation)
	}
	if req.MaxRetryAttempts > 0 || req.MaxRetryAttempts == 0 {
		policy.MaxRetryAttempts = req.MaxRetryAttempts
	}
	if req.CollectPaymentReminderSchedule != nil {
		if err := validateDunningSchedule(req.CollectPaymentReminderSchedule); err != nil {
			return domain.DunningPolicy{}, err
		}
		policy.CollectPaymentReminderSchedule = normalizeSchedule(req.CollectPaymentReminderSchedule)
	}
	if req.FinalAction != "" {
		policy.FinalAction = normalizeDunningFinalAction(req.FinalAction)
	}
	if req.GracePeriodDays < 0 {
		return domain.DunningPolicy{}, fmt.Errorf("%w: grace_period_days must be >= 0", ErrValidation)
	}
	policy.GracePeriodDays = req.GracePeriodDays
	policy.UpdatedAt = s.now()
	return s.store.UpsertDunningPolicy(policy)
}

func (s *DunningService) EnsureRunForInvoice(tenantID, invoiceID string) (EnsureInvoiceDunningRunResult, error) {
	if s == nil || s.store == nil {
		return EnsureInvoiceDunningRunResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return EnsureInvoiceDunningRunResult{}, fmt.Errorf("%w: invoice_id is required", ErrValidation)
	}

	policy, err := s.GetPolicy(tenantID)
	if err != nil {
		return EnsureInvoiceDunningRunResult{}, err
	}
	result := EnsureInvoiceDunningRunResult{Policy: policy}
	if !policy.Enabled {
		result.Reason = "dunning_policy_disabled"
		return result, nil
	}

	view, err := s.store.GetInvoicePaymentStatusView(tenantID, invoiceID)
	if err != nil {
		return EnsureInvoiceDunningRunResult{}, err
	}
	activeRun, err := s.store.GetActiveInvoiceDunningRunByInvoiceID(tenantID, invoiceID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return EnsureInvoiceDunningRunResult{}, err
	}
	hasActiveRun := err == nil

	eval, err := s.evaluate(policy, view)
	if err != nil {
		return EnsureInvoiceDunningRunResult{}, err
	}
	result.Eligible = eval.eligible
	result.Reason = eval.reason

	if !eval.eligible {
		if !hasActiveRun {
			return result, nil
		}
		updated := activeRun
		updated.State = domain.DunningRunStateResolved
		updated.NextActionAt = nil
		updated.NextActionType = ""
		updated.ResolvedAt = ptrTime(s.now())
		updated.Resolution = eval.resolution
		updated.Reason = eval.reason
		updated.UpdatedAt = s.now()
		updatedRun, err := s.store.UpdateInvoiceDunningRun(updated)
		if err != nil {
			return EnsureInvoiceDunningRunResult{}, err
		}
		event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              updatedRun.ID,
			TenantID:           tenantID,
			InvoiceID:          invoiceID,
			CustomerExternalID: updatedRun.CustomerExternalID,
			EventType:          domain.DunningEventTypeResolved,
			State:              updatedRun.State,
			Reason:             updatedRun.Reason,
			AttemptCount:       updatedRun.AttemptCount,
			Metadata: map[string]any{
				"resolution": string(updatedRun.Resolution),
			},
			CreatedAt: s.now(),
		})
		if err != nil {
			return EnsureInvoiceDunningRunResult{}, err
		}
		result.Run = &updatedRun
		result.Event = &event
		result.Updated = true
		result.Resolved = true
		return result, nil
	}

	if !hasActiveRun {
		run := domain.InvoiceDunningRun{
			TenantID:           tenantID,
			InvoiceID:          invoiceID,
			CustomerExternalID: view.CustomerExternalID,
			PolicyID:           policy.ID,
			State:              eval.state,
			Reason:             eval.reason,
			AttemptCount:       0,
			NextActionAt:       eval.nextActionAt,
			NextActionType:     eval.nextActionType,
			Paused:             false,
			CreatedAt:          s.now(),
			UpdatedAt:          s.now(),
		}
		createdRun, err := s.store.CreateInvoiceDunningRun(run)
		if err != nil {
			return EnsureInvoiceDunningRunResult{}, err
		}
		event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              createdRun.ID,
			TenantID:           tenantID,
			InvoiceID:          createdRun.InvoiceID,
			CustomerExternalID: createdRun.CustomerExternalID,
			EventType:          domain.DunningEventTypeStarted,
			State:              createdRun.State,
			ActionType:         createdRun.NextActionType,
			Reason:             createdRun.Reason,
			AttemptCount:       createdRun.AttemptCount,
			Metadata:           eval.metadata,
			CreatedAt:          s.now(),
		})
		if err != nil {
			return EnsureInvoiceDunningRunResult{}, err
		}
		result.Run = &createdRun
		result.Event = &event
		result.Created = true
		return result, nil
	}

	if activeRun.Paused {
		result.Run = &activeRun
		return result, nil
	}

	if runMatchesEvaluation(activeRun, eval) {
		result.Run = &activeRun
		return result, nil
	}

	updated := activeRun
	previousState := updated.State
	updated.CustomerExternalID = view.CustomerExternalID
	updated.PolicyID = policy.ID
	updated.State = eval.state
	updated.Reason = eval.reason
	updated.NextActionAt = eval.nextActionAt
	updated.NextActionType = eval.nextActionType
	updated.ResolvedAt = nil
	updated.Resolution = ""
	updated.UpdatedAt = s.now()
	updatedRun, err := s.store.UpdateInvoiceDunningRun(updated)
	if err != nil {
		return EnsureInvoiceDunningRunResult{}, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              updatedRun.ID,
		TenantID:           tenantID,
		InvoiceID:          updatedRun.InvoiceID,
		CustomerExternalID: updatedRun.CustomerExternalID,
		EventType:          transitionEventType(previousState, updatedRun.State),
		State:              updatedRun.State,
		ActionType:         updatedRun.NextActionType,
		Reason:             updatedRun.Reason,
		AttemptCount:       updatedRun.AttemptCount,
		Metadata:           eval.metadata,
		CreatedAt:          s.now(),
	})
	if err != nil {
		return EnsureInvoiceDunningRunResult{}, err
	}
	result.Run = &updatedRun
	result.Event = &event
	result.Updated = true
	return result, nil
}

func (s *DunningService) ListRunEvents(tenantID, runID string) ([]domain.InvoiceDunningEvent, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("%w: run_id is required", ErrValidation)
	}
	return s.store.ListInvoiceDunningEvents(normalizeTenantID(tenantID), runID)
}

func (s *DunningService) ListDueRuns(tenantID string, req ListDueDunningRunsRequest) ([]domain.InvoiceDunningRun, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	limit := req.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrValidation)
	}
	dueBefore := s.now()
	if req.DueBefore != nil {
		dueBefore = req.DueBefore.UTC()
	}
	return s.store.ListDueInvoiceDunningRuns(store.DueInvoiceDunningRunFilter{
		TenantID:   normalizeTenantID(tenantID),
		ActionType: string(req.ActionType),
		DueBefore:  dueBefore,
		Limit:      limit,
	})
}

func (s *DunningService) QueueCollectPaymentReminder(tenantID, runID string) (QueueCollectPaymentReminderResult, error) {
	if s == nil || s.store == nil {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	if run.ResolvedAt != nil {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: dunning run is already resolved", store.ErrInvalidState)
	}
	if run.Paused {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: dunning run is paused", store.ErrInvalidState)
	}
	if run.NextActionType != domain.DunningActionTypeCollectPaymentReminder {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: next action is not collect_payment_reminder", store.ErrInvalidState)
	}
	if run.NextActionAt != nil && run.NextActionAt.After(s.now()) {
		return QueueCollectPaymentReminderResult{}, fmt.Errorf("%w: dunning run is not due yet", store.ErrInvalidState)
	}

	policy, err := s.GetPolicy(tenantID)
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	intents, err := s.store.ListDunningNotificationIntents(store.DunningNotificationIntentListFilter{
		TenantID: tenantID,
		RunID:    run.ID,
		Limit:    100,
	})
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	customer, err := s.store.GetCustomerByExternalID(tenantID, run.CustomerExternalID)
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}

	intentType := domain.DunningNotificationIntentTypePaymentMethodRequired
	if len(intents)+1 >= len(policy.CollectPaymentReminderSchedule) && len(policy.CollectPaymentReminderSchedule) > 0 {
		intentType = domain.DunningNotificationIntentTypeFinalAttempt
	}
	intent, err := s.store.CreateDunningNotificationIntent(domain.DunningNotificationIntent{
		RunID:              run.ID,
		TenantID:           tenantID,
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		IntentType:         intentType,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		Status:             domain.DunningNotificationIntentStatusQueued,
		DeliveryBackend:    "alpha_email",
		RecipientEmail:     customer.Email,
		Payload: map[string]any{
			"invoice_id":            run.InvoiceID,
			"customer_external_id":  run.CustomerExternalID,
			"notification_sequence": len(intents) + 1,
		},
		CreatedAt: s.now(),
	})
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}

	updatedRun := run
	updatedRun.UpdatedAt = s.now()
	eventType := domain.DunningEventTypePaymentSetupPending
	escalated := false

	nextActionAt, exhausted := nextReminderActionAt(s.now(), policy.CollectPaymentReminderSchedule, len(intents)+1)
	if exhausted {
		switch policy.FinalAction {
		case domain.DunningFinalActionPause:
			updatedRun.Paused = true
			updatedRun.State = domain.DunningRunStatePaused
			updatedRun.NextActionAt = nil
			updatedRun.NextActionType = ""
			updatedRun.Reason = "collect_payment_reminders_exhausted"
			eventType = domain.DunningEventTypePaused
		default:
			updatedRun.State = domain.DunningRunStateEscalated
			updatedRun.NextActionAt = nil
			updatedRun.NextActionType = ""
			updatedRun.Reason = "collect_payment_reminders_exhausted"
			eventType = domain.DunningEventTypeEscalated
			escalated = true
		}
	} else {
		updatedRun.State = domain.DunningRunStateAwaitingPaymentSetup
		updatedRun.NextActionAt = nextActionAt
		updatedRun.NextActionType = domain.DunningActionTypeCollectPaymentReminder
		updatedRun.Reason = "payment_setup_pending"
	}

	updatedRun, err = s.store.UpdateInvoiceDunningRun(updatedRun)
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              updatedRun.ID,
		TenantID:           tenantID,
		InvoiceID:          updatedRun.InvoiceID,
		CustomerExternalID: updatedRun.CustomerExternalID,
		EventType:          eventType,
		State:              updatedRun.State,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		Reason:             updatedRun.Reason,
		AttemptCount:       updatedRun.AttemptCount,
		Metadata: map[string]any{
			"notification_intent_id": intent.ID,
			"notification_sequence":  len(intents) + 1,
			"intent_type":            string(intent.IntentType),
		},
		CreatedAt: s.now(),
	})
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	return QueueCollectPaymentReminderResult{
		Policy:             policy,
		Run:                updatedRun,
		Event:              event,
		NotificationIntent: intent,
		Escalated:          escalated,
	}, nil
}

func (s *DunningService) ProcessNextCollectPaymentReminder(tenantID string) (bool, *QueueCollectPaymentReminderResult, error) {
	runs, err := s.ListDueRuns(tenantID, ListDueDunningRunsRequest{
		ActionType: domain.DunningActionTypeCollectPaymentReminder,
		Limit:      1,
	})
	if err != nil {
		return false, nil, err
	}
	if len(runs) == 0 {
		return false, nil, nil
	}
	result, err := s.QueueCollectPaymentReminder(runs[0].TenantID, runs[0].ID)
	if err != nil {
		return true, nil, err
	}
	return true, &result, nil
}

type DunningWorker struct {
	service         *DunningService
	tenantID        string
	pollInterval    time.Duration
	errorBackoff    time.Duration
	maxErrorBackoff time.Duration
}

func NewDunningWorker(service *DunningService, tenantID string, pollInterval time.Duration) *DunningWorker {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	return &DunningWorker{
		service:         service,
		tenantID:        normalizeTenantID(tenantID),
		pollInterval:    pollInterval,
		errorBackoff:    250 * time.Millisecond,
		maxErrorBackoff: 5 * time.Second,
	}
}

func (w *DunningWorker) RunCollectPaymentReminders(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	backoff := w.errorBackoff
	for {
		processed, _, err := w.service.ProcessNextCollectPaymentReminder(w.tenantID)
		if err != nil {
			if !sleepWithContext(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > w.maxErrorBackoff {
				backoff = w.maxErrorBackoff
			}
			continue
		}
		backoff = w.errorBackoff
		if processed {
			select {
			case <-ctx.Done():
				return
			default:
			}
			continue
		}
		if !sleepWithContext(ctx, w.pollInterval) {
			return
		}
	}
}

type dunningEvaluation struct {
	eligible       bool
	state          domain.DunningRunState
	reason         string
	nextActionType domain.DunningActionType
	nextActionAt   *time.Time
	resolution     domain.DunningResolution
	metadata       map[string]any
}

func (s *DunningService) evaluate(policy domain.DunningPolicy, view domain.InvoicePaymentStatusView) (dunningEvaluation, error) {
	if !invoiceRequiresDunning(view) {
		return dunningEvaluation{
			eligible:   false,
			reason:     "invoice_not_collectible",
			resolution: domain.DunningResolutionInvoiceNotCollectible,
			metadata: map[string]any{
				"payment_status": strings.ToLower(strings.TrimSpace(view.PaymentStatus)),
				"invoice_status": strings.ToLower(strings.TrimSpace(view.InvoiceStatus)),
			},
		}, nil
	}
	paymentReady, readinessReason, err := s.defaultPaymentMethodReady(view.TenantID, view.CustomerExternalID)
	if err != nil {
		return dunningEvaluation{}, err
	}
	eval := dunningEvaluation{
		eligible: true,
		reason:   readinessReason,
		metadata: map[string]any{
			"payment_status":       strings.ToLower(strings.TrimSpace(view.PaymentStatus)),
			"invoice_status":       strings.ToLower(strings.TrimSpace(view.InvoiceStatus)),
			"payment_overdue":      view.PaymentOverdue != nil && *view.PaymentOverdue,
			"customer_external_id": strings.TrimSpace(view.CustomerExternalID),
		},
	}
	if paymentReady {
		eval.state = domain.DunningRunStateRetryDue
		eval.nextActionType = domain.DunningActionTypeRetryPayment
		nextActionAt, err := scheduleFromPolicy(s.now(), policy.GracePeriodDays, policy.RetrySchedule)
		if err != nil {
			return dunningEvaluation{}, err
		}
		eval.nextActionAt = nextActionAt
		return eval, nil
	}
	eval.state = domain.DunningRunStateAwaitingPaymentSetup
	eval.nextActionType = domain.DunningActionTypeCollectPaymentReminder
	nextActionAt, err := scheduleFromPolicy(s.now(), policy.GracePeriodDays, policy.CollectPaymentReminderSchedule)
	if err != nil {
		return dunningEvaluation{}, err
	}
	eval.nextActionAt = nextActionAt
	return eval, nil
}

func (s *DunningService) defaultPaymentMethodReady(tenantID, customerExternalID string) (bool, string, error) {
	customerExternalID = strings.TrimSpace(customerExternalID)
	if customerExternalID == "" {
		return false, "customer_external_id_missing", nil
	}
	customer, err := s.store.GetCustomerByExternalID(normalizeTenantID(tenantID), customerExternalID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, "customer_not_found", nil
		}
		return false, "", err
	}
	if customer.Status != domain.CustomerStatusActive {
		return false, "customer_inactive", nil
	}
	setup, err := s.store.GetCustomerPaymentSetup(customer.TenantID, customer.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, "payment_setup_missing", nil
		}
		return false, "", err
	}
	if setup.SetupStatus != domain.PaymentSetupStatusReady || !setup.DefaultPaymentMethodPresent {
		return false, "payment_setup_pending", nil
	}
	return true, "payment_setup_ready", nil
}

func defaultDunningPolicy(tenantID string, now time.Time) domain.DunningPolicy {
	return domain.DunningPolicy{
		TenantID:                       normalizeTenantID(tenantID),
		Name:                           "Default dunning policy",
		Enabled:                        true,
		RetrySchedule:                  []string{"1d", "3d", "5d"},
		MaxRetryAttempts:               3,
		CollectPaymentReminderSchedule: []string{"0d", "2d", "5d"},
		FinalAction:                    domain.DunningFinalActionManualReview,
		GracePeriodDays:                0,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	}
}

func invoiceRequiresDunning(view domain.InvoicePaymentStatusView) bool {
	if strings.ToLower(strings.TrimSpace(view.InvoiceStatus)) != "finalized" {
		return false
	}
	paymentStatus := strings.ToLower(strings.TrimSpace(view.PaymentStatus))
	if paymentStatus == "succeeded" || paymentStatus == "paid" {
		return false
	}
	if view.PaymentOverdue != nil && *view.PaymentOverdue {
		return true
	}
	switch paymentStatus {
	case "failed", "pending":
		return true
	default:
		return false
	}
}

func transitionEventType(previousState, nextState domain.DunningRunState) domain.DunningEventType {
	if nextState == domain.DunningRunStateResolved {
		return domain.DunningEventTypeResolved
	}
	if previousState == domain.DunningRunStateAwaitingPaymentSetup && nextState == domain.DunningRunStateRetryDue {
		return domain.DunningEventTypePaymentSetupReady
	}
	if nextState == domain.DunningRunStateAwaitingPaymentSetup {
		return domain.DunningEventTypePaymentSetupPending
	}
	if nextState == domain.DunningRunStateRetryDue || nextState == domain.DunningRunStateScheduled {
		return domain.DunningEventTypeRetryScheduled
	}
	if nextState == domain.DunningRunStatePaused {
		return domain.DunningEventTypePaused
	}
	return domain.DunningEventTypeStarted
}

func runMatchesEvaluation(run domain.InvoiceDunningRun, eval dunningEvaluation) bool {
	if run.State != eval.state || run.Reason != eval.reason || run.NextActionType != eval.nextActionType {
		return false
	}
	return timesEqual(run.NextActionAt, eval.nextActionAt)
}

func timesEqual(left, right *time.Time) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil || right == nil {
		return false
	}
	return left.UTC().Equal(right.UTC())
}

func scheduleFromPolicy(now time.Time, gracePeriodDays int, schedule []string) (*time.Time, error) {
	base := now.UTC()
	if gracePeriodDays > 0 {
		base = base.Add(time.Duration(gracePeriodDays) * 24 * time.Hour)
	}
	if len(schedule) == 0 {
		return &base, nil
	}
	delay, err := parseDunningDelay(schedule[0])
	if err != nil {
		return nil, err
	}
	at := base.Add(delay)
	return &at, nil
}

func nextReminderActionAt(now time.Time, schedule []string, nextIndex int) (*time.Time, bool) {
	if nextIndex < 0 {
		nextIndex = 0
	}
	if nextIndex >= len(schedule) {
		return nil, true
	}
	delay, err := parseDunningDelay(schedule[nextIndex])
	if err != nil {
		return nil, true
	}
	at := now.UTC().Add(delay)
	return &at, false
}

func validateDunningSchedule(values []string) error {
	for _, value := range values {
		if _, err := parseDunningDelay(value); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSchedule(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func parseDunningDelay(value string) (time.Duration, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return 0, fmt.Errorf("%w: dunning schedule entries must be non-empty", ErrValidation)
	}
	if len(trimmed) < 2 {
		return 0, fmt.Errorf("%w: invalid dunning delay %q", ErrValidation, value)
	}
	unit := trimmed[len(trimmed)-1]
	raw := trimmed[:len(trimmed)-1]
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: invalid dunning delay %q", ErrValidation, value)
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("%w: invalid dunning delay unit in %q", ErrValidation, value)
	}
}

func normalizeDunningFinalAction(v domain.DunningFinalAction) domain.DunningFinalAction {
	switch strings.ToLower(strings.TrimSpace(string(v))) {
	case string(domain.DunningFinalActionPause):
		return domain.DunningFinalActionPause
	case string(domain.DunningFinalActionWriteOff):
		return domain.DunningFinalActionWriteOff
	default:
		return domain.DunningFinalActionManualReview
	}
}

func ptrTime(v time.Time) *time.Time {
	value := v.UTC()
	return &value
}
