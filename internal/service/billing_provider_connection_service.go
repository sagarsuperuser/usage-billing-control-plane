package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type BillingProviderAdapter interface {
	EnsureStripeProvider(ctx context.Context, input EnsureStripeProviderInput) (EnsureStripeProviderResult, error)
}

type StripeConnectionVerifier interface {
	VerifyStripeSecret(ctx context.Context, secretKey string) (StripeConnectionVerificationResult, error)
}

type StripeConnectionVerificationResult struct {
	AccountID  string
	Livemode   bool
	VerifiedAt time.Time
}

type EnsureStripeProviderInput struct {
	ConnectionID       string
	DisplayName        string
	Environment        string
	SecretKey          string
	LagoOrganizationID string
	LagoProviderCode   string
	OwnerTenantID      string
}

type EnsureStripeProviderResult struct {
	LagoOrganizationID string
	LagoProviderCode   string
	LagoWebhookHMACKey string
	ConnectedAt        time.Time
	LastSyncedAt       time.Time
}

type BillingProviderConnectionService struct {
	store                     store.Repository
	secretStore               BillingSecretStore
	adapter                   BillingProviderAdapter
	verifier                  StripeConnectionVerifier
	defaultOrganizationID string
}

type CreateBillingProviderConnectionRequest struct {
	ProviderType       string `json:"provider_type"`
	Environment        string `json:"environment"`
	DisplayName        string `json:"display_name"`
	Scope              string `json:"scope"`
	OwnerTenantID      string `json:"owner_tenant_id,omitempty"`
	StripeSecretKey    string `json:"stripe_secret_key,omitempty"`
	LagoOrganizationID string `json:"lago_organization_id,omitempty"`
	LagoProviderCode   string `json:"lago_provider_code,omitempty"`
	LagoWebhookHMACKey string `json:"lago_webhook_hmac_key,omitempty"`
}

type UpdateBillingProviderConnectionRequest struct {
	DisplayName        *string `json:"display_name,omitempty"`
	Environment        *string `json:"environment,omitempty"`
	Scope              *string `json:"scope,omitempty"`
	OwnerTenantID      *string `json:"owner_tenant_id,omitempty"`
	LagoOrganizationID *string `json:"lago_organization_id,omitempty"`
	LagoProviderCode   *string `json:"lago_provider_code,omitempty"`
	LagoWebhookHMACKey *string `json:"lago_webhook_hmac_key,omitempty"`
}

type ListBillingProviderConnectionsRequest struct {
	ProviderType  string `json:"provider_type,omitempty"`
	Environment   string `json:"environment,omitempty"`
	Status        string `json:"status,omitempty"`
	Scope         string `json:"scope,omitempty"`
	OwnerTenantID string `json:"owner_tenant_id,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
}

type BillingProviderConnectionRecheckBatchRequest struct {
	Limit      int
	StaleAfter time.Duration
}

type BillingProviderConnectionRecheckBatchResult struct {
	Scanned int
	Checked int
	Healthy int
	Failed  int
	Skipped int
}

type ProvisionWorkspaceBillingConnectionInput struct {
	ConnectionID       string
	OwnerTenantID      string
	LagoOrganizationID string
	LagoProviderCode   string
}

func NewBillingProviderConnectionService(repo store.Repository, secretStore BillingSecretStore, adapter BillingProviderAdapter) *BillingProviderConnectionService {
	return &BillingProviderConnectionService{store: repo, secretStore: secretStore, adapter: adapter}
}

func (s *BillingProviderConnectionService) WithDefaultOrganizationID(id string) *BillingProviderConnectionService {
	if s == nil {
		return nil
	}
	s.defaultOrganizationID = strings.TrimSpace(id)
	return s
}

func (s *BillingProviderConnectionService) DefaultOrganizationID() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.defaultOrganizationID)
}

func (s *BillingProviderConnectionService) WithStripeConnectionVerifier(verifier StripeConnectionVerifier) *BillingProviderConnectionService {
	if s == nil {
		return nil
	}
	s.verifier = verifier
	return s
}

func (s *BillingProviderConnectionService) CreateBillingProviderConnection(ctx context.Context, req CreateBillingProviderConnectionRequest, actorType, actorID string) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	if s.secretStore == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing secret store is required", ErrValidation)
	}
	providerType, err := normalizeBillingProviderType(req.ProviderType)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	environment, err := normalizeBillingProviderEnvironment(req.Environment)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	scope, ownerTenantID, err := s.normalizeBillingProviderScope(req.Scope, req.OwnerTenantID)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: display_name is required", ErrValidation)
	}
	actorType = strings.ToLower(strings.TrimSpace(actorType))
	actorID = strings.TrimSpace(actorID)
	if actorType == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: actor type is required", ErrValidation)
	}
	stripeSecretKey := strings.TrimSpace(req.StripeSecretKey)
	lagoWebhookHMACKey := strings.TrimSpace(req.LagoWebhookHMACKey)
	if providerType == domain.BillingProviderTypeStripe && stripeSecretKey == "" && lagoWebhookHMACKey == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: stripe_secret_key or lago_webhook_hmac_key is required", ErrValidation)
	}

	id, err := newBillingProviderConnectionID()
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	secretRef, err := s.secretStore.PutConnectionSecrets(ctx, id, BillingProviderSecrets{
		StripeSecretKey:    stripeSecretKey,
		LagoWebhookHMACKey: lagoWebhookHMACKey,
	})
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}

	now := time.Now().UTC()
	connection, err := s.store.CreateBillingProviderConnection(domain.BillingProviderConnection{
		ID:                 id,
		ProviderType:       providerType,
		Environment:        environment,
		DisplayName:        displayName,
		Scope:              scope,
		OwnerTenantID:      ownerTenantID,
		Status:             domain.BillingProviderConnectionStatusPending,
		LagoOrganizationID: strings.TrimSpace(req.LagoOrganizationID),
		LagoProviderCode:   strings.TrimSpace(req.LagoProviderCode),
		SecretRef:          secretRef,
		CreatedByType:      actorType,
		CreatedByID:        actorID,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	if err != nil {
		_ = s.secretStore.DeleteSecret(ctx, secretRef)
		return domain.BillingProviderConnection{}, err
	}
	return connection, nil
}

func (s *BillingProviderConnectionService) GetBillingProviderConnection(id string) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: id is required", ErrValidation)
	}
	return s.store.GetBillingProviderConnection(id)
}

func (s *BillingProviderConnectionService) ListBillingProviderConnections(req ListBillingProviderConnectionsRequest) ([]domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	providerType, err := normalizeBillingProviderTypeFilter(req.ProviderType)
	if err != nil {
		return nil, err
	}
	environment, err := normalizeBillingProviderEnvironmentFilter(req.Environment)
	if err != nil {
		return nil, err
	}
	status, err := normalizeBillingProviderStatusFilter(req.Status)
	if err != nil {
		return nil, err
	}
	scope, err := normalizeBillingProviderScopeFilter(req.Scope)
	if err != nil {
		return nil, err
	}
	return s.store.ListBillingProviderConnections(store.BillingProviderConnectionListFilter{
		ProviderType:  providerType,
		Environment:   environment,
		Status:        status,
		Scope:         scope,
		OwnerTenantID: normalizeOptionalTenantID(req.OwnerTenantID),
		Limit:         limit,
		Offset:        offset,
	})
}

func (s *BillingProviderConnectionService) UpdateBillingProviderConnection(id string, req UpdateBillingProviderConnectionRequest) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	current, err := s.GetBillingProviderConnection(id)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	updated := current
	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if displayName == "" {
			return domain.BillingProviderConnection{}, fmt.Errorf("%w: display_name is required", ErrValidation)
		}
		updated.DisplayName = displayName
	}
	if req.Environment != nil {
		environment, err := normalizeBillingProviderEnvironment(*req.Environment)
		if err != nil {
			return domain.BillingProviderConnection{}, err
		}
		updated.Environment = environment
	}
	if req.Scope != nil || req.OwnerTenantID != nil {
		rawScope := string(updated.Scope)
		if req.Scope != nil {
			rawScope = *req.Scope
		}
		rawOwnerTenantID := updated.OwnerTenantID
		if req.OwnerTenantID != nil {
			rawOwnerTenantID = *req.OwnerTenantID
		}
		scope, ownerTenantID, err := s.normalizeBillingProviderScope(rawScope, rawOwnerTenantID)
		if err != nil {
			return domain.BillingProviderConnection{}, err
		}
		updated.Scope = scope
		updated.OwnerTenantID = ownerTenantID
	}
	if req.LagoOrganizationID != nil {
		updated.LagoOrganizationID = strings.TrimSpace(*req.LagoOrganizationID)
	}
	if req.LagoProviderCode != nil {
		updated.LagoProviderCode = strings.TrimSpace(*req.LagoProviderCode)
	}
	if req.LagoWebhookHMACKey != nil {
		if strings.TrimSpace(updated.SecretRef) == "" {
			return domain.BillingProviderConnection{}, fmt.Errorf("%w: secret_ref is required to update webhook hmac key", ErrValidation)
		}
		secrets, err := s.secretStore.GetConnectionSecrets(context.Background(), updated.SecretRef)
		if err != nil {
			return domain.BillingProviderConnection{}, err
		}
		secrets.LagoWebhookHMACKey = strings.TrimSpace(*req.LagoWebhookHMACKey)
		if _, err := s.secretStore.UpdateConnectionSecrets(context.Background(), updated.SecretRef, secrets); err != nil {
			return domain.BillingProviderConnection{}, err
		}
	}
	updated.UpdatedAt = time.Now().UTC()
	return s.store.UpdateBillingProviderConnection(updated)
}

func (s *BillingProviderConnectionService) RotateBillingProviderConnectionSecret(ctx context.Context, id, stripeSecretKey string) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	if s.secretStore == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing secret store is required", ErrValidation)
	}
	current, err := s.GetBillingProviderConnection(id)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	if current.Status == domain.BillingProviderConnectionStatusDisabled {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: disabled connections cannot rotate secret", ErrValidation)
	}
	stripeSecretKey = strings.TrimSpace(stripeSecretKey)
	if stripeSecretKey == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: stripe_secret_key is required", ErrValidation)
	}
	var secretRef string
	if strings.TrimSpace(current.SecretRef) == "" {
		secretRef, err = s.secretStore.PutStripeSecret(ctx, current.ID, stripeSecretKey)
	} else {
		secretRef, err = s.secretStore.RotateStripeSecret(ctx, current.SecretRef, stripeSecretKey)
	}
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	current.SecretRef = secretRef
	current.Status = domain.BillingProviderConnectionStatusPending
	current.LastSyncError = ""
	current.LastSyncedAt = nil
	current.UpdatedAt = time.Now().UTC()
	return s.store.UpdateBillingProviderConnection(current)
}

func (s *BillingProviderConnectionService) SyncBillingProviderConnection(ctx context.Context, id string) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	if s.secretStore == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing secret store is required", ErrValidation)
	}
	if s.verifier == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: stripe connection verifier is required", ErrValidation)
	}
	current, err := s.GetBillingProviderConnection(id)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	if current.Status == domain.BillingProviderConnectionStatusDisabled {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: disabled connections cannot sync", ErrValidation)
	}
	if current.ProviderType != domain.BillingProviderTypeStripe {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: unsupported provider type %q", ErrValidation, current.ProviderType)
	}
	secrets, err := s.secretStore.GetConnectionSecrets(ctx, current.SecretRef)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	if strings.TrimSpace(secrets.StripeSecretKey) == "" {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: stripe secret is required before sync", ErrValidation)
	}
	verificationResult, verifyErr := s.verifier.VerifyStripeSecret(ctx, secrets.StripeSecretKey)
	if verifyErr != nil {
		current.Status = domain.BillingProviderConnectionStatusSyncError
		current.LastSyncError = strings.TrimSpace(verifyErr.Error())
		current.UpdatedAt = time.Now().UTC()
		updated, updateErr := s.store.UpdateBillingProviderConnection(current)
		if updateErr != nil {
			return domain.BillingProviderConnection{}, updateErr
		}
		return updated, verifyErr
	}
	if verificationResult.VerifiedAt.IsZero() {
		verificationResult.VerifiedAt = time.Now().UTC()
	}
	current.Status = domain.BillingProviderConnectionStatusConnected
	now := time.Now().UTC()
	if current.ConnectedAt == nil {
		current.ConnectedAt = &now
	}
	current.LastSyncedAt = &now
	current.LastSyncError = ""
	current.UpdatedAt = now
	return s.store.UpdateBillingProviderConnection(current)
}

func (s *BillingProviderConnectionService) ProvisionWorkspaceBillingConnection(ctx context.Context, input ProvisionWorkspaceBillingConnectionInput) (EnsureStripeProviderResult, error) {
	if s == nil || s.store == nil {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	if s.secretStore == nil {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: billing secret store is required", ErrValidation)
	}
	if s.adapter == nil {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: billing provider adapter is required", ErrValidation)
	}

	connectionID := strings.TrimSpace(input.ConnectionID)
	if connectionID == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: billing provider connection id is required", ErrValidation)
	}
	resolvedLagoOrganizationID := strings.TrimSpace(input.LagoOrganizationID)
	if resolvedLagoOrganizationID == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: lago organization id is required", ErrValidation)
	}

	current, err := s.GetBillingProviderConnection(connectionID)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	if current.Status == domain.BillingProviderConnectionStatusDisabled {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: disabled connections cannot be assigned to a workspace", ErrValidation)
	}
	if current.Status != domain.BillingProviderConnectionStatusConnected {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: billing provider connection must be checked before workspace assignment", ErrValidation)
	}
	if current.ProviderType != domain.BillingProviderTypeStripe {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: unsupported provider type %q", ErrValidation, current.ProviderType)
	}

	secrets, err := s.secretStore.GetConnectionSecrets(ctx, current.SecretRef)
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	if strings.TrimSpace(secrets.StripeSecretKey) == "" {
		return EnsureStripeProviderResult{}, fmt.Errorf("%w: stripe secret is required before workspace assignment", ErrValidation)
	}

	result, err := s.adapter.EnsureStripeProvider(ctx, EnsureStripeProviderInput{
		ConnectionID:       current.ID,
		DisplayName:        current.DisplayName,
		Environment:        current.Environment,
		SecretKey:          secrets.StripeSecretKey,
		LagoOrganizationID: resolvedLagoOrganizationID,
		LagoProviderCode:   strings.TrimSpace(input.LagoProviderCode),
		OwnerTenantID:      strings.TrimSpace(input.OwnerTenantID),
	})
	if err != nil {
		return EnsureStripeProviderResult{}, err
	}
	if strings.TrimSpace(result.LagoWebhookHMACKey) != "" {
		secrets.LagoWebhookHMACKey = strings.TrimSpace(result.LagoWebhookHMACKey)
		if _, err := s.secretStore.UpdateConnectionSecrets(ctx, current.SecretRef, secrets); err != nil {
			return EnsureStripeProviderResult{}, err
		}
	}
	if result.ConnectedAt.IsZero() {
		result.ConnectedAt = time.Now().UTC()
	}
	if result.LastSyncedAt.IsZero() {
		result.LastSyncedAt = result.ConnectedAt
	}
	if strings.TrimSpace(result.LagoOrganizationID) == "" {
		result.LagoOrganizationID = resolvedLagoOrganizationID
	}
	return result, nil
}

func (s *BillingProviderConnectionService) RecheckBillingProviderConnectionsBatch(ctx context.Context, req BillingProviderConnectionRecheckBatchRequest) (BillingProviderConnectionRecheckBatchResult, error) {
	if s == nil || s.store == nil {
		return BillingProviderConnectionRecheckBatchResult{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	if req.Limit <= 0 {
		return BillingProviderConnectionRecheckBatchResult{}, fmt.Errorf("%w: limit must be > 0", ErrValidation)
	}
	if req.StaleAfter <= 0 {
		return BillingProviderConnectionRecheckBatchResult{}, fmt.Errorf("%w: stale_after must be > 0", ErrValidation)
	}

	pageSize := req.Limit
	if pageSize < 25 {
		pageSize = 25
	}

	var result BillingProviderConnectionRecheckBatchResult
	offset := 0
	now := time.Now().UTC()
	for result.Checked < req.Limit {
		items, err := s.store.ListBillingProviderConnections(store.BillingProviderConnectionListFilter{
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			return result, err
		}
		if len(items) == 0 {
			break
		}
		result.Scanned += len(items)
		for _, item := range items {
			if result.Checked >= req.Limit {
				break
			}
			if item.Status == domain.BillingProviderConnectionStatusDisabled {
				result.Skipped++
				continue
			}
			if item.LastSyncedAt != nil && now.Sub(item.LastSyncedAt.UTC()) < req.StaleAfter {
				result.Skipped++
				continue
			}
			result.Checked++
			if _, err := s.SyncBillingProviderConnection(ctx, item.ID); err != nil {
				result.Failed++
			} else {
				result.Healthy++
			}
		}
		offset += len(items)
		if len(items) < pageSize {
			break
		}
	}
	return result, nil
}

func (s *BillingProviderConnectionService) DisableBillingProviderConnection(id string) (domain.BillingProviderConnection, error) {
	if s == nil || s.store == nil {
		return domain.BillingProviderConnection{}, fmt.Errorf("%w: billing provider repository is required", ErrValidation)
	}
	current, err := s.GetBillingProviderConnection(id)
	if err != nil {
		return domain.BillingProviderConnection{}, err
	}
	if current.Status == domain.BillingProviderConnectionStatusDisabled {
		return current, nil
	}
	now := time.Now().UTC()
	current.Status = domain.BillingProviderConnectionStatusDisabled
	current.DisabledAt = &now
	current.UpdatedAt = now
	return s.store.UpdateBillingProviderConnection(current)
}

func (s *BillingProviderConnectionService) normalizeBillingProviderScope(rawScope, rawOwnerTenantID string) (domain.BillingProviderConnectionScope, string, error) {
	scope, err := normalizeBillingProviderScope(rawScope)
	if err != nil {
		return "", "", err
	}
	ownerTenantID := normalizeOptionalTenantID(rawOwnerTenantID)
	switch scope {
	case domain.BillingProviderConnectionScopePlatform:
		if ownerTenantID != "" {
			return "", "", fmt.Errorf("%w: owner_tenant_id must be empty for platform scope", ErrValidation)
		}
	case domain.BillingProviderConnectionScopeTenant:
		if ownerTenantID == "" {
			return "", "", fmt.Errorf("%w: owner_tenant_id is required for tenant scope", ErrValidation)
		}
		if _, err := s.store.GetTenant(ownerTenantID); err != nil {
			if err == store.ErrNotFound {
				return "", "", fmt.Errorf("%w: owner tenant not found", ErrValidation)
			}
			return "", "", err
		}
	}
	return scope, ownerTenantID, nil
}

func normalizeBillingProviderType(raw string) (domain.BillingProviderType, error) {
	switch domain.BillingProviderType(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.BillingProviderTypeStripe:
		return domain.BillingProviderTypeStripe, nil
	default:
		return "", fmt.Errorf("%w: provider_type must be stripe", ErrValidation)
	}
}

func normalizeBillingProviderTypeFilter(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	value, err := normalizeBillingProviderType(raw)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func normalizeBillingProviderEnvironment(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "test":
		return "test", nil
	case "live":
		return "live", nil
	default:
		return "", fmt.Errorf("%w: environment must be test or live", ErrValidation)
	}
}

func normalizeBillingProviderEnvironmentFilter(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	return normalizeBillingProviderEnvironment(raw)
}

func normalizeBillingProviderScope(raw string) (domain.BillingProviderConnectionScope, error) {
	switch domain.BillingProviderConnectionScope(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.BillingProviderConnectionScopePlatform:
		return domain.BillingProviderConnectionScopePlatform, nil
	case domain.BillingProviderConnectionScopeTenant:
		return domain.BillingProviderConnectionScopeTenant, nil
	default:
		return "", fmt.Errorf("%w: scope must be platform or tenant", ErrValidation)
	}
}

func normalizeBillingProviderScopeFilter(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	value, err := normalizeBillingProviderScope(raw)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func normalizeBillingProviderStatusFilter(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case string(domain.BillingProviderConnectionStatusPending),
		string(domain.BillingProviderConnectionStatusConnected),
		string(domain.BillingProviderConnectionStatusSyncError),
		string(domain.BillingProviderConnectionStatusDisabled):
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("%w: status must be pending, connected, sync_error, or disabled", ErrValidation)
	}
}

func normalizeOptionalTenantID(raw string) string {
	return strings.TrimSpace(raw)
}

func newBillingProviderConnectionID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate billing provider connection id: %w", err)
	}
	return "bpc_" + hex.EncodeToString(buf), nil
}
