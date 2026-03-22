package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type SubscriptionService struct {
	store                   store.Repository
	customers               *CustomerService
	subscriptionSyncAdapter SubscriptionSyncAdapter
}

type CreateSubscriptionRequest struct {
	TenantID            string     `json:"tenant_id,omitempty"`
	Code                string     `json:"code,omitempty"`
	DisplayName         string     `json:"display_name,omitempty"`
	CustomerExternalID  string     `json:"customer_external_id"`
	PlanID              string     `json:"plan_id"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	RequestPaymentSetup bool       `json:"request_payment_setup,omitempty"`
	PaymentMethodType   string     `json:"payment_method_type,omitempty"`
}

type UpdateSubscriptionRequest struct {
	DisplayName *string                    `json:"display_name,omitempty"`
	PlanID      *string                    `json:"plan_id,omitempty"`
	Status      *domain.SubscriptionStatus `json:"status,omitempty"`
	StartedAt   *time.Time                 `json:"started_at,omitempty"`
}

type SubscriptionSummary struct {
	ID                           string                    `json:"id"`
	TenantID                     string                    `json:"tenant_id,omitempty"`
	Code                         string                    `json:"code"`
	DisplayName                  string                    `json:"display_name"`
	Status                       domain.SubscriptionStatus `json:"status"`
	CustomerID                   string                    `json:"customer_id"`
	CustomerExternalID           string                    `json:"customer_external_id"`
	CustomerDisplayName          string                    `json:"customer_display_name"`
	PlanID                       string                    `json:"plan_id"`
	PlanCode                     string                    `json:"plan_code"`
	PlanName                     string                    `json:"plan_name"`
	BillingInterval              domain.BillingInterval    `json:"billing_interval"`
	Currency                     string                    `json:"currency"`
	BaseAmountCents              int64                     `json:"base_amount_cents"`
	PaymentSetupStatus           domain.PaymentSetupStatus `json:"payment_setup_status"`
	DefaultPaymentMethodVerified bool                      `json:"default_payment_method_verified"`
	PaymentSetupActionRequired   bool                      `json:"payment_setup_action_required"`
	StartedAt                    *time.Time                `json:"started_at,omitempty"`
	PaymentSetupRequestedAt      *time.Time                `json:"payment_setup_requested_at,omitempty"`
	ActivatedAt                  *time.Time                `json:"activated_at,omitempty"`
	CreatedAt                    time.Time                 `json:"created_at"`
	UpdatedAt                    time.Time                 `json:"updated_at"`
}

type SubscriptionDetail struct {
	SubscriptionSummary
	Customer       domain.Customer               `json:"customer"`
	Plan           domain.Plan                   `json:"plan"`
	BillingProfile domain.CustomerBillingProfile `json:"billing_profile"`
	PaymentSetup   domain.CustomerPaymentSetup   `json:"payment_setup"`
	MissingSteps   []string                      `json:"missing_steps"`
}

type SubscriptionPaymentSetupResult struct {
	Action       string             `json:"action"`
	CheckoutURL  string             `json:"checkout_url,omitempty"`
	Subscription SubscriptionDetail `json:"subscription"`
}

type CreateSubscriptionResult struct {
	Subscription        SubscriptionDetail `json:"subscription"`
	PaymentSetupStarted bool               `json:"payment_setup_started"`
	CheckoutURL         string             `json:"checkout_url,omitempty"`
}

var (
	subscriptionCodeInvalidRE = regexp.MustCompile(`[^a-z0-9_-]+`)
	subscriptionCodeMultiRE   = regexp.MustCompile(`_+`)
)

func NewSubscriptionService(s store.Repository, customers *CustomerService) *SubscriptionService {
	return &SubscriptionService{store: s, customers: customers}
}

func (s *SubscriptionService) WithSubscriptionSyncAdapter(adapter SubscriptionSyncAdapter) *SubscriptionService {
	s.subscriptionSyncAdapter = adapter
	return s
}

func (s *SubscriptionService) CreateSubscription(ctx context.Context, req CreateSubscriptionRequest) (CreateSubscriptionResult, error) {
	if s == nil || s.store == nil {
		return CreateSubscriptionResult{}, fmt.Errorf("%w: subscription repository is required", ErrValidation)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tenantID := normalizeTenantID(req.TenantID)
	customer, plan, err := s.resolveCustomerAndPlan(tenantID, req.CustomerExternalID, req.PlanID)
	if err != nil {
		return CreateSubscriptionResult{}, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = defaultSubscriptionDisplayName(customer, plan)
	}
	code := normalizeSubscriptionCode(req.Code)
	if code == "" {
		code = normalizeSubscriptionCode(fmt.Sprintf("%s_%s", customer.ExternalID, plan.Code))
	}
	if code == "" {
		return CreateSubscriptionResult{}, fmt.Errorf("%w: code is required", ErrValidation)
	}

	subscription, err := s.store.CreateSubscription(domain.Subscription{
		TenantID:    tenantID,
		Code:        code,
		DisplayName: displayName,
		CustomerID:  customer.ID,
		PlanID:      plan.ID,
		Status:      domain.SubscriptionStatusDraft,
		StartedAt:   normalizeOptionalUTC(req.StartedAt),
	})
	if err != nil {
		if err == store.ErrDuplicateKey {
			return CreateSubscriptionResult{}, fmt.Errorf("%w: subscription code already exists", ErrValidation)
		}
		return CreateSubscriptionResult{}, err
	}
	if s.subscriptionSyncAdapter != nil {
		if err := s.subscriptionSyncAdapter.SyncSubscription(ctx, subscription, customer, plan); err != nil {
			return CreateSubscriptionResult{}, err
		}
	}

	if req.RequestPaymentSetup {
		paymentResult, err := s.requestPaymentSetup(tenantID, subscription.ID, req.PaymentMethodType, "requested")
		if err != nil {
			return CreateSubscriptionResult{}, err
		}
		return CreateSubscriptionResult{
			Subscription:        paymentResult.Subscription,
			PaymentSetupStarted: paymentResult.Action == "requested" || paymentResult.Action == "resent",
			CheckoutURL:         paymentResult.CheckoutURL,
		}, nil
	}

	detail, err := s.GetSubscription(tenantID, subscription.ID)
	if err != nil {
		return CreateSubscriptionResult{}, err
	}
	return CreateSubscriptionResult{Subscription: detail}, nil
}

func (s *SubscriptionService) ListSubscriptions(tenantID string) ([]SubscriptionSummary, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: subscription repository is required", ErrValidation)
	}
	items, err := s.store.ListSubscriptions(normalizeTenantID(tenantID))
	if err != nil {
		return nil, err
	}
	out := make([]SubscriptionSummary, 0, len(items))
	for _, item := range items {
		summary, err := s.buildSubscriptionSummary(item)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *SubscriptionService) GetSubscription(tenantID, id string) (SubscriptionDetail, error) {
	if s == nil || s.store == nil {
		return SubscriptionDetail{}, fmt.Errorf("%w: subscription repository is required", ErrValidation)
	}
	subscription, err := s.store.GetSubscription(normalizeTenantID(tenantID), strings.TrimSpace(id))
	if err != nil {
		return SubscriptionDetail{}, err
	}
	return s.buildSubscriptionDetail(subscription)
}

func (s *SubscriptionService) UpdateSubscription(ctx context.Context, tenantID, id string, req UpdateSubscriptionRequest) (SubscriptionDetail, error) {
	if s == nil || s.store == nil {
		return SubscriptionDetail{}, fmt.Errorf("%w: subscription repository is required", ErrValidation)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	subscription, err := s.store.GetSubscription(normalizeTenantID(tenantID), strings.TrimSpace(id))
	if err != nil {
		return SubscriptionDetail{}, err
	}
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name == "" {
			return SubscriptionDetail{}, fmt.Errorf("%w: display_name is required", ErrValidation)
		}
		subscription.DisplayName = name
	}
	if req.PlanID != nil {
		planID := strings.TrimSpace(*req.PlanID)
		if planID == "" {
			return SubscriptionDetail{}, fmt.Errorf("%w: plan_id is required", ErrValidation)
		}
		if _, err := s.store.GetPlan(subscription.TenantID, planID); err != nil {
			if err == store.ErrNotFound {
				return SubscriptionDetail{}, fmt.Errorf("%w: plan not found", ErrValidation)
			}
			return SubscriptionDetail{}, err
		}
		subscription.PlanID = planID
	}
	if req.Status != nil {
		subscription.Status = normalizeSubscriptionLifecycle(*req.Status)
	}
	if req.StartedAt != nil {
		subscription.StartedAt = normalizeOptionalUTC(req.StartedAt)
	}
	updated, err := s.store.UpdateSubscription(subscription)
	if err != nil {
		if err == store.ErrDuplicateKey {
			return SubscriptionDetail{}, fmt.Errorf("%w: subscription code already exists", ErrValidation)
		}
		return SubscriptionDetail{}, err
	}
	if s.subscriptionSyncAdapter != nil {
		customer, err := s.store.GetCustomer(updated.TenantID, updated.CustomerID)
		if err != nil {
			return SubscriptionDetail{}, err
		}
		plan, err := s.store.GetPlan(updated.TenantID, updated.PlanID)
		if err != nil {
			return SubscriptionDetail{}, err
		}
		if err := s.subscriptionSyncAdapter.SyncSubscription(ctx, updated, customer, plan); err != nil {
			return SubscriptionDetail{}, err
		}
	}
	return s.buildSubscriptionDetail(updated)
}

func (s *SubscriptionService) RequestPaymentSetup(tenantID, id, paymentMethodType string) (SubscriptionPaymentSetupResult, error) {
	return s.requestPaymentSetup(tenantID, id, paymentMethodType, "requested")
}

func (s *SubscriptionService) ResendPaymentSetup(tenantID, id, paymentMethodType string) (SubscriptionPaymentSetupResult, error) {
	return s.requestPaymentSetup(tenantID, id, paymentMethodType, "resent")
}

func (s *SubscriptionService) requestPaymentSetup(tenantID, id, paymentMethodType, action string) (SubscriptionPaymentSetupResult, error) {
	if s == nil || s.store == nil || s.customers == nil {
		return SubscriptionPaymentSetupResult{}, fmt.Errorf("%w: subscription services are required", ErrValidation)
	}
	subscription, err := s.store.GetSubscription(normalizeTenantID(tenantID), strings.TrimSpace(id))
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	customer, err := s.store.GetCustomer(subscription.TenantID, subscription.CustomerID)
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	readiness, err := s.customers.GetCustomerReadiness(subscription.TenantID, customer.ExternalID)
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	if readiness.DefaultPaymentMethodVerified && readiness.PaymentSetupStatus == domain.PaymentSetupStatusReady {
		subscription.Status = domain.SubscriptionStatusActive
		now := time.Now().UTC()
		subscription.ActivatedAt = &now
		updated, err := s.store.UpdateSubscription(subscription)
		if err != nil {
			return SubscriptionPaymentSetupResult{}, err
		}
		detail, err := s.buildSubscriptionDetail(updated)
		if err != nil {
			return SubscriptionPaymentSetupResult{}, err
		}
		return SubscriptionPaymentSetupResult{Action: "already_ready", Subscription: detail}, nil
	}

	setupResult, err := s.customers.BeginCustomerPaymentSetup(subscription.TenantID, customer.ExternalID, BeginCustomerPaymentSetupRequest{
		PaymentMethodType: strings.TrimSpace(paymentMethodType),
	})
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	now := time.Now().UTC()
	subscription.PaymentSetupRequestedAt = &now
	subscription.Status = deriveSubscriptionStatus(subscription.Status, subscription.PaymentSetupRequestedAt, setupResult.PaymentSetup.SetupStatus, setupResult.PaymentSetup.DefaultPaymentMethodPresent)
	if subscription.Status == domain.SubscriptionStatusActive && subscription.ActivatedAt == nil {
		subscription.ActivatedAt = &now
	}
	updated, err := s.store.UpdateSubscription(subscription)
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	detail, err := s.buildSubscriptionDetail(updated)
	if err != nil {
		return SubscriptionPaymentSetupResult{}, err
	}
	return SubscriptionPaymentSetupResult{
		Action:       action,
		CheckoutURL:  setupResult.CheckoutURL,
		Subscription: detail,
	}, nil
}

func (s *SubscriptionService) resolveCustomerAndPlan(tenantID, customerExternalID, planID string) (domain.Customer, domain.Plan, error) {
	customer, err := s.customers.GetCustomerByExternalID(tenantID, strings.TrimSpace(customerExternalID))
	if err != nil {
		if err == store.ErrNotFound {
			return domain.Customer{}, domain.Plan{}, fmt.Errorf("%w: customer not found", ErrValidation)
		}
		return domain.Customer{}, domain.Plan{}, err
	}
	plan, err := s.store.GetPlan(tenantID, strings.TrimSpace(planID))
	if err != nil {
		if err == store.ErrNotFound {
			return domain.Customer{}, domain.Plan{}, fmt.Errorf("%w: plan not found", ErrValidation)
		}
		return domain.Customer{}, domain.Plan{}, err
	}
	return customer, plan, nil
}

func (s *SubscriptionService) buildSubscriptionSummary(subscription domain.Subscription) (SubscriptionSummary, error) {
	customer, err := s.store.GetCustomer(subscription.TenantID, subscription.CustomerID)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	plan, err := s.store.GetPlan(subscription.TenantID, subscription.PlanID)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	readiness, err := s.customers.GetCustomerReadiness(subscription.TenantID, customer.ExternalID)
	if err != nil {
		return SubscriptionSummary{}, err
	}
	return SubscriptionSummary{
		ID:                           subscription.ID,
		TenantID:                     subscription.TenantID,
		Code:                         subscription.Code,
		DisplayName:                  subscription.DisplayName,
		Status:                       deriveSubscriptionStatus(subscription.Status, subscription.PaymentSetupRequestedAt, readiness.PaymentSetupStatus, readiness.DefaultPaymentMethodVerified),
		CustomerID:                   customer.ID,
		CustomerExternalID:           customer.ExternalID,
		CustomerDisplayName:          customer.DisplayName,
		PlanID:                       plan.ID,
		PlanCode:                     plan.Code,
		PlanName:                     plan.Name,
		BillingInterval:              plan.BillingInterval,
		Currency:                     plan.Currency,
		BaseAmountCents:              plan.BaseAmountCents,
		PaymentSetupStatus:           readiness.PaymentSetupStatus,
		DefaultPaymentMethodVerified: readiness.DefaultPaymentMethodVerified,
		PaymentSetupActionRequired:   readiness.PaymentSetupStatus == domain.PaymentSetupStatusError,
		StartedAt:                    subscription.StartedAt,
		PaymentSetupRequestedAt:      subscription.PaymentSetupRequestedAt,
		ActivatedAt:                  subscription.ActivatedAt,
		CreatedAt:                    subscription.CreatedAt,
		UpdatedAt:                    subscription.UpdatedAt,
	}, nil
}

func (s *SubscriptionService) buildSubscriptionDetail(subscription domain.Subscription) (SubscriptionDetail, error) {
	summary, err := s.buildSubscriptionSummary(subscription)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	customer, err := s.store.GetCustomer(subscription.TenantID, subscription.CustomerID)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	plan, err := s.store.GetPlan(subscription.TenantID, subscription.PlanID)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	readiness, err := s.customers.GetCustomerReadiness(subscription.TenantID, customer.ExternalID)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	return SubscriptionDetail{
		SubscriptionSummary: summary,
		Customer:            customer,
		Plan:                plan,
		BillingProfile:      readiness.BillingProfile,
		PaymentSetup:        readiness.PaymentSetup,
		MissingSteps:        append([]string(nil), readiness.MissingSteps...),
	}, nil
}

func normalizeSubscriptionCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = subscriptionCodeInvalidRE.ReplaceAllString(raw, "_")
	raw = subscriptionCodeMultiRE.ReplaceAllString(raw, "_")
	return strings.Trim(raw, "_")
}

func normalizeSubscriptionLifecycle(status domain.SubscriptionStatus) domain.SubscriptionStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case string(domain.SubscriptionStatusPendingPaymentSetup):
		return domain.SubscriptionStatusPendingPaymentSetup
	case string(domain.SubscriptionStatusActive):
		return domain.SubscriptionStatusActive
	case string(domain.SubscriptionStatusActionRequired):
		return domain.SubscriptionStatusActionRequired
	case string(domain.SubscriptionStatusArchived):
		return domain.SubscriptionStatusArchived
	default:
		return domain.SubscriptionStatusDraft
	}
}

func deriveSubscriptionStatus(current domain.SubscriptionStatus, paymentSetupRequestedAt *time.Time, paymentSetupStatus domain.PaymentSetupStatus, defaultPaymentMethodVerified bool) domain.SubscriptionStatus {
	if normalizeSubscriptionLifecycle(current) == domain.SubscriptionStatusArchived {
		return domain.SubscriptionStatusArchived
	}
	if defaultPaymentMethodVerified && paymentSetupStatus == domain.PaymentSetupStatusReady {
		return domain.SubscriptionStatusActive
	}
	if paymentSetupStatus == domain.PaymentSetupStatusError {
		return domain.SubscriptionStatusActionRequired
	}
	if paymentSetupRequestedAt != nil || paymentSetupStatus == domain.PaymentSetupStatusPending {
		return domain.SubscriptionStatusPendingPaymentSetup
	}
	return domain.SubscriptionStatusDraft
}

func normalizeOptionalUTC(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	t := value.UTC()
	return &t
}

func defaultSubscriptionDisplayName(customer domain.Customer, plan domain.Plan) string {
	if strings.TrimSpace(customer.DisplayName) == "" {
		return plan.Name
	}
	if strings.TrimSpace(plan.Name) == "" {
		return customer.DisplayName
	}
	return strings.TrimSpace(customer.DisplayName) + " - " + strings.TrimSpace(plan.Name)
}
