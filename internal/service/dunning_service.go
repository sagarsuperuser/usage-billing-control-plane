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
	ListInvoiceDunningRuns(filter store.InvoiceDunningRunListFilter) ([]domain.InvoiceDunningRun, error)
	ListDueInvoiceDunningRuns(filter store.DueInvoiceDunningRunFilter) ([]domain.InvoiceDunningRun, error)
	CreateInvoiceDunningEvent(input domain.InvoiceDunningEvent) (domain.InvoiceDunningEvent, error)
	ListInvoiceDunningEvents(tenantID, runID string) ([]domain.InvoiceDunningEvent, error)
	CreateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error)
	UpdateDunningNotificationIntent(input domain.DunningNotificationIntent) (domain.DunningNotificationIntent, error)
	ListDunningNotificationIntents(filter store.DunningNotificationIntentListFilter) ([]domain.DunningNotificationIntent, error)
}

type dunningPaymentSetupRequestSender interface {
	Resend(tenantID, externalID string, actor CustomerPaymentSetupRequestActor, paymentMethodType string) (CustomerPaymentSetupRequestResult, error)
}

type dunningInvoiceRetryExecutor interface {
	RetryInvoicePayment(ctx context.Context, invoiceID string, payload []byte) (int, []byte, error)
}

type DunningService struct {
	store                dunningStore
	paymentSetupRequests dunningPaymentSetupRequestSender
	invoiceRetries       dunningInvoiceRetryExecutor
	now                  func() time.Time
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
	DispatchEvent      *domain.InvoiceDunningEvent      `json:"dispatch_event,omitempty"`
	Escalated          bool                             `json:"escalated"`
}

type DunningCollectPaymentReminderBatchResult struct {
	TenantID   string `json:"tenant_id"`
	Limit      int    `json:"limit"`
	Processed  int    `json:"processed"`
	Dispatched int    `json:"dispatched"`
	Failed     int    `json:"failed"`
	LastRunID  string `json:"last_run_id,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type DunningRunControlResult struct {
	Run   domain.InvoiceDunningRun   `json:"run"`
	Event domain.InvoiceDunningEvent `json:"event"`
}

type RetryPaymentResult struct {
	Policy       domain.DunningPolicy       `json:"policy"`
	Run          domain.InvoiceDunningRun   `json:"run"`
	Event        domain.InvoiceDunningEvent `json:"event"`
	StatusCode   int                        `json:"status_code,omitempty"`
	Escalated    bool                       `json:"escalated"`
	ResponseBody string                     `json:"response_body,omitempty"`
}

type DunningRetryPaymentBatchResult struct {
	TenantID   string `json:"tenant_id"`
	Limit      int    `json:"limit"`
	Processed  int    `json:"processed"`
	Dispatched int    `json:"dispatched"`
	Failed     int    `json:"failed"`
	LastRunID  string `json:"last_run_id,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type ListDunningRunsRequest struct {
	InvoiceID          string `json:"invoice_id,omitempty"`
	CustomerExternalID string `json:"customer_external_id,omitempty"`
	State              string `json:"state,omitempty"`
	ActiveOnly         bool   `json:"active_only"`
	Limit              int    `json:"limit,omitempty"`
	Offset             int    `json:"offset,omitempty"`
}

type DunningRunDetail struct {
	Run                 domain.InvoiceDunningRun           `json:"run"`
	Events              []domain.InvoiceDunningEvent       `json:"events"`
	NotificationIntents []domain.DunningNotificationIntent `json:"notification_intents"`
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

func (s *DunningService) WithPaymentSetupRequestSender(sender dunningPaymentSetupRequestSender) *DunningService {
	if s == nil {
		return nil
	}
	s.paymentSetupRequests = sender
	return s
}

func (s *DunningService) WithInvoiceRetryExecutor(executor dunningInvoiceRetryExecutor) *DunningService {
	if s == nil {
		return nil
	}
	s.invoiceRetries = executor
	return s
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
		eventType := domain.DunningEventTypeResolved
		if previousState := activeRun.State; previousState == domain.DunningRunStateAwaitingRetryResult && updatedRun.Resolution == domain.DunningResolutionPaymentSucceeded {
			eventType = domain.DunningEventTypeRetrySucceeded
		}
		event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              updatedRun.ID,
			TenantID:           tenantID,
			InvoiceID:          invoiceID,
			CustomerExternalID: updatedRun.CustomerExternalID,
			EventType:          eventType,
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

func (s *DunningService) GetInvoiceSummary(tenantID, invoiceID string) (*domain.DunningSummary, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	invoiceID = strings.TrimSpace(invoiceID)
	if invoiceID == "" {
		return nil, fmt.Errorf("%w: invoice_id is required", ErrValidation)
	}

	run, err := s.store.GetActiveInvoiceDunningRunByInvoiceID(tenantID, invoiceID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		runs, listErr := s.store.ListInvoiceDunningRuns(store.InvoiceDunningRunListFilter{
			TenantID:  tenantID,
			InvoiceID: invoiceID,
			Limit:     1,
			Offset:    0,
		})
		if listErr != nil {
			return nil, listErr
		}
		if len(runs) == 0 {
			return nil, nil
		}
		run = runs[0]
	}
	events, err := s.store.ListInvoiceDunningEvents(tenantID, run.ID)
	if err != nil {
		return nil, err
	}
	intents, err := s.store.ListDunningNotificationIntents(store.DunningNotificationIntentListFilter{
		TenantID: tenantID,
		RunID:    run.ID,
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}

	summary := &domain.DunningSummary{
		RunID:          run.ID,
		State:          run.State,
		Reason:         run.Reason,
		AttemptCount:   run.AttemptCount,
		NextActionType: run.NextActionType,
		NextActionAt:   run.NextActionAt,
		Paused:         run.Paused,
		Resolution:     run.Resolution,
	}
	if latest := latestDunningEvent(events); latest != nil {
		summary.LastEventType = latest.EventType
		summary.LastEventAt = ptrTime(latest.CreatedAt)
	}
	if latest := latestDunningNotificationIntent(intents); latest != nil {
		summary.LastNotificationIntentType = latest.IntentType
		summary.LastNotificationStatus = latest.Status
		summary.LastNotificationError = strings.TrimSpace(latest.LastError)
		if latest.DispatchedAt != nil {
			summary.LastNotificationAt = ptrTime(latest.DispatchedAt.UTC())
		} else {
			summary.LastNotificationAt = ptrTime(latest.CreatedAt)
		}
	}
	return summary, nil
}

func (s *DunningService) ListRuns(tenantID string, req ListDunningRunsRequest) ([]domain.InvoiceDunningRun, error) {
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
	if req.Offset < 0 {
		return nil, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	return s.store.ListInvoiceDunningRuns(store.InvoiceDunningRunListFilter{
		TenantID:           normalizeTenantID(tenantID),
		InvoiceID:          strings.TrimSpace(req.InvoiceID),
		CustomerExternalID: strings.TrimSpace(req.CustomerExternalID),
		State:              strings.ToLower(strings.TrimSpace(req.State)),
		ActiveOnly:         req.ActiveOnly,
		Limit:              limit,
		Offset:             req.Offset,
	})
}

func (s *DunningService) RefreshRunsForCustomer(tenantID, customerExternalID string) ([]EnsureInvoiceDunningRunResult, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	customerExternalID = strings.TrimSpace(customerExternalID)
	if customerExternalID == "" {
		return nil, fmt.Errorf("%w: customer_external_id is required", ErrValidation)
	}
	runs, err := s.store.ListInvoiceDunningRuns(store.InvoiceDunningRunListFilter{
		TenantID:           tenantID,
		CustomerExternalID: customerExternalID,
		ActiveOnly:         true,
		Limit:              100,
		Offset:             0,
	})
	if err != nil {
		return nil, err
	}
	results := make([]EnsureInvoiceDunningRunResult, 0, len(runs))
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		invoiceID := strings.TrimSpace(run.InvoiceID)
		if invoiceID == "" {
			continue
		}
		if _, ok := seen[invoiceID]; ok {
			continue
		}
		seen[invoiceID] = struct{}{}
		result, err := s.EnsureRunForInvoice(tenantID, invoiceID)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *DunningService) GetRunDetail(tenantID, runID string) (DunningRunDetail, error) {
	if s == nil || s.store == nil {
		return DunningRunDetail{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return DunningRunDetail{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}
	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return DunningRunDetail{}, err
	}
	events, err := s.store.ListInvoiceDunningEvents(tenantID, run.ID)
	if err != nil {
		return DunningRunDetail{}, err
	}
	intents, err := s.store.ListDunningNotificationIntents(store.DunningNotificationIntentListFilter{
		TenantID: tenantID,
		RunID:    run.ID,
		Limit:    100,
	})
	if err != nil {
		return DunningRunDetail{}, err
	}
	return DunningRunDetail{
		Run:                 run,
		Events:              events,
		NotificationIntents: intents,
	}, nil
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
	return s.queueCollectPaymentReminder(tenantID, runID, true)
}

func (s *DunningService) queueCollectPaymentReminder(tenantID, runID string, requireDue bool) (QueueCollectPaymentReminderResult, error) {
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
	if requireDue && run.NextActionAt != nil && run.NextActionAt.After(s.now()) {
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

func (s *DunningService) DispatchCollectPaymentReminder(tenantID, runID string) (QueueCollectPaymentReminderResult, error) {
	result, err := s.queueCollectPaymentReminder(tenantID, runID, false)
	if err != nil {
		return QueueCollectPaymentReminderResult{}, err
	}
	intent, event, err := s.dispatchCollectPaymentReminderIntent(normalizeTenantID(tenantID), result.NotificationIntent)
	result.NotificationIntent = intent
	result.DispatchEvent = event
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *DunningService) PauseRun(tenantID, runID string) (DunningRunControlResult, error) {
	if s == nil || s.store == nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return DunningRunControlResult{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	if run.ResolvedAt != nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning run is already resolved", store.ErrInvalidState)
	}
	if run.Paused || run.State == domain.DunningRunStatePaused {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning run is already paused", store.ErrInvalidState)
	}

	now := s.now()
	run.Paused = true
	run.State = domain.DunningRunStatePaused
	run.NextActionAt = nil
	run.NextActionType = ""
	run.Reason = "paused_by_operator"
	run.UpdatedAt = now

	run, err = s.store.UpdateInvoiceDunningRun(run)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              run.ID,
		TenantID:           tenantID,
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		EventType:          domain.DunningEventTypePaused,
		State:              run.State,
		Reason:             run.Reason,
		AttemptCount:       run.AttemptCount,
		CreatedAt:          now,
	})
	if err != nil {
		return DunningRunControlResult{}, err
	}
	return DunningRunControlResult{Run: run, Event: event}, nil
}

func (s *DunningService) ResumeRun(tenantID, runID string) (DunningRunControlResult, error) {
	if s == nil || s.store == nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return DunningRunControlResult{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	if run.ResolvedAt != nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning run is already resolved", store.ErrInvalidState)
	}
	if !run.Paused && run.State != domain.DunningRunStatePaused {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning run is not paused", store.ErrInvalidState)
	}

	policy, err := s.GetPolicy(tenantID)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	view, err := s.store.GetInvoicePaymentStatusView(tenantID, run.InvoiceID)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	eval, err := s.evaluate(policy, view)
	if err != nil {
		return DunningRunControlResult{}, err
	}

	now := s.now()
	run.Paused = false
	run.UpdatedAt = now
	metadata := map[string]any{
		"resumed_from": string(domain.DunningRunStatePaused),
	}
	if !eval.eligible {
		run.State = domain.DunningRunStateResolved
		run.NextActionAt = nil
		run.NextActionType = ""
		run.ResolvedAt = ptrTime(now)
		run.Resolution = eval.resolution
		run.Reason = eval.reason
		run, err = s.store.UpdateInvoiceDunningRun(run)
		if err != nil {
			return DunningRunControlResult{}, err
		}
		metadata["resolution"] = string(run.Resolution)
		event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              run.ID,
			TenantID:           tenantID,
			InvoiceID:          run.InvoiceID,
			CustomerExternalID: run.CustomerExternalID,
			EventType:          domain.DunningEventTypeResolved,
			State:              run.State,
			Reason:             run.Reason,
			AttemptCount:       run.AttemptCount,
			Metadata:           metadata,
			CreatedAt:          now,
		})
		if err != nil {
			return DunningRunControlResult{}, err
		}
		return DunningRunControlResult{Run: run, Event: event}, nil
	}

	run.State = eval.state
	run.NextActionAt = eval.nextActionAt
	run.NextActionType = eval.nextActionType
	run.ResolvedAt = nil
	run.Resolution = ""
	run.Reason = eval.reason
	for k, v := range eval.metadata {
		metadata[k] = v
	}
	metadata["resumed_to_state"] = string(run.State)
	metadata["resumed_to_action"] = string(run.NextActionType)
	run, err = s.store.UpdateInvoiceDunningRun(run)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              run.ID,
		TenantID:           tenantID,
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		EventType:          domain.DunningEventTypeResumed,
		State:              run.State,
		ActionType:         run.NextActionType,
		Reason:             run.Reason,
		AttemptCount:       run.AttemptCount,
		Metadata:           metadata,
		CreatedAt:          now,
	})
	if err != nil {
		return DunningRunControlResult{}, err
	}
	return DunningRunControlResult{Run: run, Event: event}, nil
}

func (s *DunningService) ResolveRun(tenantID, runID string) (DunningRunControlResult, error) {
	if s == nil || s.store == nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return DunningRunControlResult{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	if run.ResolvedAt != nil {
		return DunningRunControlResult{}, fmt.Errorf("%w: dunning run is already resolved", store.ErrInvalidState)
	}

	now := s.now()
	run.Paused = false
	run.State = domain.DunningRunStateResolved
	run.NextActionAt = nil
	run.NextActionType = ""
	run.ResolvedAt = ptrTime(now)
	run.Resolution = domain.DunningResolutionOperatorResolved
	run.Reason = "operator_resolved"
	run.UpdatedAt = now
	run, err = s.store.UpdateInvoiceDunningRun(run)
	if err != nil {
		return DunningRunControlResult{}, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              run.ID,
		TenantID:           tenantID,
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		EventType:          domain.DunningEventTypeResolved,
		State:              run.State,
		Reason:             run.Reason,
		AttemptCount:       run.AttemptCount,
		Metadata: map[string]any{
			"resolution": string(run.Resolution),
		},
		CreatedAt: now,
	})
	if err != nil {
		return DunningRunControlResult{}, err
	}
	return DunningRunControlResult{Run: run, Event: event}, nil
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
	intent, event, err := s.dispatchCollectPaymentReminderIntent(runs[0].TenantID, result.NotificationIntent)
	result.NotificationIntent = intent
	result.DispatchEvent = event
	if err != nil {
		return true, &result, err
	}
	return true, &result, nil
}

func (s *DunningService) ProcessCollectPaymentReminderBatch(tenantID string, limit int) (DunningCollectPaymentReminderBatchResult, error) {
	if s == nil || s.store == nil {
		return DunningCollectPaymentReminderBatchResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 100 {
		return DunningCollectPaymentReminderBatchResult{}, fmt.Errorf("%w: limit must be between 1 and 100", ErrValidation)
	}

	out := DunningCollectPaymentReminderBatchResult{
		TenantID: normalizeTenantID(tenantID),
		Limit:    limit,
	}
	for i := 0; i < limit; i++ {
		processed, result, err := s.ProcessNextCollectPaymentReminder(out.TenantID)
		if !processed {
			return out, nil
		}
		out.Processed++
		if result != nil {
			out.LastRunID = strings.TrimSpace(result.Run.ID)
			if result.NotificationIntent.Status == domain.DunningNotificationIntentStatusDispatched {
				out.Dispatched++
			}
		}
		if err != nil {
			out.Failed++
			out.LastError = err.Error()
			if result == nil {
				return out, err
			}
		}
	}
	return out, nil
}

func (s *DunningService) DispatchRetryPayment(tenantID, runID string) (RetryPaymentResult, error) {
	return s.dispatchRetryPayment(tenantID, runID, false)
}

func (s *DunningService) ProcessNextRetryPayment(tenantID string) (bool, *RetryPaymentResult, error) {
	runs, err := s.ListDueRuns(tenantID, ListDueDunningRunsRequest{
		ActionType: domain.DunningActionTypeRetryPayment,
		Limit:      1,
	})
	if err != nil {
		return false, nil, err
	}
	if len(runs) == 0 {
		return false, nil, nil
	}
	result, err := s.dispatchRetryPayment(runs[0].TenantID, runs[0].ID, true)
	if err != nil {
		return true, &result, err
	}
	return true, &result, nil
}

func (s *DunningService) ProcessRetryPaymentBatch(tenantID string, limit int) (DunningRetryPaymentBatchResult, error) {
	if s == nil || s.store == nil {
		return DunningRetryPaymentBatchResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 100 {
		return DunningRetryPaymentBatchResult{}, fmt.Errorf("%w: limit must be between 1 and 100", ErrValidation)
	}

	out := DunningRetryPaymentBatchResult{
		TenantID: normalizeTenantID(tenantID),
		Limit:    limit,
	}
	for i := 0; i < limit; i++ {
		processed, result, err := s.ProcessNextRetryPayment(out.TenantID)
		if !processed {
			return out, nil
		}
		out.Processed++
		if result != nil {
			out.LastRunID = strings.TrimSpace(result.Run.ID)
			if result.Event.EventType == domain.DunningEventTypeRetryAttempted {
				out.Dispatched++
			}
		}
		if err != nil {
			out.Failed++
			out.LastError = err.Error()
			if result == nil {
				return out, err
			}
		}
	}
	return out, nil
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
	paymentStatus := strings.ToLower(strings.TrimSpace(view.PaymentStatus))
	invoiceStatus := strings.ToLower(strings.TrimSpace(view.InvoiceStatus))
	if !invoiceRequiresDunning(view) {
		resolution := domain.DunningResolutionInvoiceNotCollectible
		reason := "invoice_not_collectible"
		if paymentStatus == "succeeded" || paymentStatus == "paid" {
			resolution = domain.DunningResolutionPaymentSucceeded
			reason = "payment_succeeded"
		}
		return dunningEvaluation{
			eligible:   false,
			reason:     reason,
			resolution: resolution,
			metadata: map[string]any{
				"payment_status": paymentStatus,
				"invoice_status": invoiceStatus,
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
			"payment_status":       paymentStatus,
			"invoice_status":       invoiceStatus,
			"payment_overdue":      view.PaymentOverdue != nil && *view.PaymentOverdue,
			"customer_external_id": strings.TrimSpace(view.CustomerExternalID),
		},
	}
	if paymentReady {
		if paymentStatus == "pending" {
			eval.state = domain.DunningRunStateAwaitingRetryResult
			eval.nextActionType = ""
			eval.nextActionAt = nil
			eval.reason = "payment_reprocessing"
			return eval, nil
		}
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

func (s *DunningService) dispatchCollectPaymentReminderIntent(tenantID string, intent domain.DunningNotificationIntent) (domain.DunningNotificationIntent, *domain.InvoiceDunningEvent, error) {
	if s == nil || s.store == nil {
		return domain.DunningNotificationIntent{}, nil, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	if s.paymentSetupRequests == nil {
		return domain.DunningNotificationIntent{}, nil, fmt.Errorf("%w: payment setup request sender is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	if intent.ActionType != domain.DunningActionTypeCollectPaymentReminder {
		return domain.DunningNotificationIntent{}, nil, fmt.Errorf("%w: intent action_type must be collect_payment_reminder", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, intent.RunID)
	if err != nil {
		return domain.DunningNotificationIntent{}, nil, err
	}
	customer, err := s.store.GetCustomerByExternalID(tenantID, run.CustomerExternalID)
	if err != nil {
		return domain.DunningNotificationIntent{}, nil, err
	}
	setup, err := s.store.GetCustomerPaymentSetup(tenantID, customer.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return domain.DunningNotificationIntent{}, nil, err
	}
	paymentMethodType := ""
	if err == nil {
		paymentMethodType = strings.TrimSpace(setup.PaymentMethodType)
	}

	updatedIntent := intent
	updatedIntent.TenantID = tenantID
	dispatchResult, dispatchErr := s.paymentSetupRequests.Resend(tenantID, run.CustomerExternalID, CustomerPaymentSetupRequestActor{
		SubjectType: "system",
		SubjectID:   "dunning",
		UserEmail:   "system@alpha.internal",
	}, paymentMethodType)
	if dispatchErr != nil {
		updatedIntent.Status = domain.DunningNotificationIntentStatusFailed
		updatedIntent.LastError = dispatchErr.Error()
		updatedIntent.DispatchedAt = nil
		updatedIntent, updateErr := s.store.UpdateDunningNotificationIntent(updatedIntent)
		if updateErr != nil {
			return domain.DunningNotificationIntent{}, nil, updateErr
		}
		event, eventErr := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              run.ID,
			TenantID:           tenantID,
			InvoiceID:          run.InvoiceID,
			CustomerExternalID: run.CustomerExternalID,
			EventType:          domain.DunningEventTypeNotificationFailed,
			State:              run.State,
			ActionType:         domain.DunningActionTypeCollectPaymentReminder,
			Reason:             "collect_payment_reminder_dispatch_failed",
			AttemptCount:       run.AttemptCount,
			Metadata: map[string]any{
				"notification_intent_id": updatedIntent.ID,
				"dispatch_error":         dispatchErr.Error(),
			},
			CreatedAt: s.now(),
		})
		if eventErr != nil {
			return domain.DunningNotificationIntent{}, nil, eventErr
		}
		return updatedIntent, &event, dispatchErr
	}

	updatedIntent.Status = domain.DunningNotificationIntentStatusDispatched
	updatedIntent.LastError = ""
	updatedIntent.DispatchedAt = ptrTime(dispatchResult.Dispatch.DispatchedAt)
	if backend := strings.TrimSpace(dispatchResult.Dispatch.Backend); backend != "" {
		updatedIntent.DeliveryBackend = backend
	}
	if email := strings.TrimSpace(dispatchResult.PaymentSetup.LastRequestToEmail); email != "" {
		updatedIntent.RecipientEmail = strings.ToLower(email)
	}
	updatedIntent, err = s.store.UpdateDunningNotificationIntent(updatedIntent)
	if err != nil {
		return domain.DunningNotificationIntent{}, nil, err
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              run.ID,
		TenantID:           tenantID,
		InvoiceID:          run.InvoiceID,
		CustomerExternalID: run.CustomerExternalID,
		EventType:          domain.DunningEventTypeNotificationSent,
		State:              run.State,
		ActionType:         domain.DunningActionTypeCollectPaymentReminder,
		Reason:             "collect_payment_reminder_dispatched",
		AttemptCount:       run.AttemptCount,
		Metadata: map[string]any{
			"notification_intent_id": updatedIntent.ID,
			"dispatch_action":        dispatchResult.Action,
			"delivery_backend":       dispatchResult.Dispatch.Backend,
		},
		CreatedAt: s.now(),
	})
	if err != nil {
		return domain.DunningNotificationIntent{}, nil, err
	}
	return updatedIntent, &event, nil
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
	if previousState == domain.DunningRunStateAwaitingRetryResult && (nextState == domain.DunningRunStateRetryDue || nextState == domain.DunningRunStateScheduled) {
		return domain.DunningEventTypeRetryFailed
	}
	if nextState == domain.DunningRunStateResolved {
		return domain.DunningEventTypeResolved
	}
	if previousState == domain.DunningRunStateAwaitingPaymentSetup &&
		(nextState == domain.DunningRunStateRetryDue || nextState == domain.DunningRunStateAwaitingRetryResult) {
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

func (s *DunningService) dispatchRetryPayment(tenantID, runID string, requireDue bool) (RetryPaymentResult, error) {
	if s == nil || s.store == nil {
		return RetryPaymentResult{}, fmt.Errorf("%w: dunning repository is required", ErrValidation)
	}
	if s.invoiceRetries == nil {
		return RetryPaymentResult{}, fmt.Errorf("%w: invoice retry executor is required", ErrValidation)
	}
	tenantID = normalizeTenantID(tenantID)
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RetryPaymentResult{}, fmt.Errorf("%w: run_id is required", ErrValidation)
	}

	run, err := s.store.GetInvoiceDunningRun(tenantID, runID)
	if err != nil {
		return RetryPaymentResult{}, err
	}
	if run.ResolvedAt != nil {
		return RetryPaymentResult{}, fmt.Errorf("%w: dunning run is already resolved", store.ErrInvalidState)
	}
	if run.Paused {
		return RetryPaymentResult{}, fmt.Errorf("%w: dunning run is paused", store.ErrInvalidState)
	}
	if run.NextActionType != domain.DunningActionTypeRetryPayment {
		return RetryPaymentResult{}, fmt.Errorf("%w: next action is not retry_payment", store.ErrInvalidState)
	}
	if requireDue && run.NextActionAt != nil && run.NextActionAt.After(s.now()) {
		return RetryPaymentResult{}, fmt.Errorf("%w: dunning run is not due yet", store.ErrInvalidState)
	}

	policy, err := s.GetPolicy(tenantID)
	if err != nil {
		return RetryPaymentResult{}, err
	}

	retryCtx := ContextWithLagoTenant(context.Background(), tenantID)
	statusCode, body, retryErr := s.invoiceRetries.RetryInvoicePayment(retryCtx, run.InvoiceID, []byte("{}"))
	now := s.now()
	updatedRun := run
	updatedRun.AttemptCount++
	updatedRun.LastAttemptAt = ptrTime(now)
	updatedRun.UpdatedAt = now

	result := RetryPaymentResult{
		Policy:       policy,
		StatusCode:   statusCode,
		ResponseBody: abbreviateRetryResponse(body),
	}

	if retryErr == nil && statusCode >= 200 && statusCode < 300 {
		updatedRun.State = domain.DunningRunStateAwaitingRetryResult
		updatedRun.NextActionAt = nil
		updatedRun.NextActionType = ""
		updatedRun.Reason = "retry_requested"
		updatedRun, err = s.store.UpdateInvoiceDunningRun(updatedRun)
		if err != nil {
			return RetryPaymentResult{}, err
		}
		event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
			RunID:              updatedRun.ID,
			TenantID:           tenantID,
			InvoiceID:          updatedRun.InvoiceID,
			CustomerExternalID: updatedRun.CustomerExternalID,
			EventType:          domain.DunningEventTypeRetryAttempted,
			State:              updatedRun.State,
			ActionType:         domain.DunningActionTypeRetryPayment,
			Reason:             updatedRun.Reason,
			AttemptCount:       updatedRun.AttemptCount,
			Metadata: map[string]any{
				"status_code": statusCode,
			},
			CreatedAt: now,
		})
		if err != nil {
			return RetryPaymentResult{}, err
		}
		result.Run = updatedRun
		result.Event = event
		return result, nil
	}

	nextActionAt, exhausted := nextRetryActionAt(now, policy, updatedRun.AttemptCount)
	if exhausted {
		switch policy.FinalAction {
		case domain.DunningFinalActionPause:
			updatedRun.Paused = true
			updatedRun.State = domain.DunningRunStatePaused
			updatedRun.NextActionAt = nil
			updatedRun.NextActionType = ""
			updatedRun.Reason = "retry_attempts_exhausted"
		default:
			updatedRun.State = domain.DunningRunStateEscalated
			updatedRun.NextActionAt = nil
			updatedRun.NextActionType = ""
			updatedRun.Reason = "retry_attempts_exhausted"
			result.Escalated = true
		}
	} else {
		updatedRun.State = domain.DunningRunStateScheduled
		updatedRun.NextActionAt = nextActionAt
		updatedRun.NextActionType = domain.DunningActionTypeRetryPayment
		updatedRun.Reason = "retry_failed"
	}
	updatedRun, err = s.store.UpdateInvoiceDunningRun(updatedRun)
	if err != nil {
		return RetryPaymentResult{}, err
	}
	eventMetadata := map[string]any{
		"status_code": statusCode,
	}
	if result.ResponseBody != "" {
		eventMetadata["response_body"] = result.ResponseBody
	}
	if retryErr != nil {
		eventMetadata["dispatch_error"] = retryErr.Error()
	}
	event, err := s.store.CreateInvoiceDunningEvent(domain.InvoiceDunningEvent{
		RunID:              updatedRun.ID,
		TenantID:           tenantID,
		InvoiceID:          updatedRun.InvoiceID,
		CustomerExternalID: updatedRun.CustomerExternalID,
		EventType:          domain.DunningEventTypeRetryFailed,
		State:              updatedRun.State,
		ActionType:         domain.DunningActionTypeRetryPayment,
		Reason:             updatedRun.Reason,
		AttemptCount:       updatedRun.AttemptCount,
		Metadata:           eventMetadata,
		CreatedAt:          now,
	})
	if err != nil {
		return RetryPaymentResult{}, err
	}
	result.Run = updatedRun
	result.Event = event
	if retryErr != nil {
		return result, retryErr
	}
	return result, fmt.Errorf("retry payment request returned status %d", statusCode)
}

func latestDunningEvent(items []domain.InvoiceDunningEvent) *domain.InvoiceDunningEvent {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.CreatedAt.After(latest.CreatedAt) {
			latest = item
		}
	}
	return &latest
}

func latestDunningNotificationIntent(items []domain.DunningNotificationIntent) *domain.DunningNotificationIntent {
	if len(items) == 0 {
		return nil
	}
	latest := items[0]
	for _, item := range items[1:] {
		if dunningNotificationSortTime(item).After(dunningNotificationSortTime(latest)) {
			latest = item
		}
	}
	return &latest
}

func dunningNotificationSortTime(item domain.DunningNotificationIntent) time.Time {
	if item.DispatchedAt != nil {
		return item.DispatchedAt.UTC()
	}
	return item.CreatedAt.UTC()
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

func nextRetryActionAt(now time.Time, policy domain.DunningPolicy, attemptCount int) (*time.Time, bool) {
	if policy.MaxRetryAttempts > 0 && attemptCount >= policy.MaxRetryAttempts {
		return nil, true
	}
	return nextReminderActionAt(now, policy.RetrySchedule, attemptCount)
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

func abbreviateRetryResponse(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) <= 256 {
		return trimmed
	}
	return trimmed[:256] + "..."
}
