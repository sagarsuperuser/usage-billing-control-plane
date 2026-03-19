package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type APIKeyService struct {
	store store.Repository
}

const (
	apiKeyStateActive  = "active"
	apiKeyStateRevoked = "revoked"
	apiKeyStateExpired = "expired"

	apiKeyAuditActionCreated = "created"
	apiKeyAuditActionRevoked = "revoked"
	apiKeyAuditActionRotated = "rotated"

	defaultListLimit = 20
	maxListLimit     = 100
)

type CreateAPIKeyRequest struct {
	Name                  string     `json:"name"`
	Role                  string     `json:"role"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty"`
	OwnerType             string     `json:"owner_type,omitempty"`
	OwnerID               string     `json:"owner_id,omitempty"`
	Purpose               string     `json:"purpose,omitempty"`
	Environment           string     `json:"environment,omitempty"`
	CreatedByUserID       string     `json:"created_by_user_id,omitempty"`
	CreatedByPlatformUser bool       `json:"created_by_platform_user,omitempty"`
	ActorPlatformAPIKeyID string     `json:"actor_platform_api_key_id,omitempty"`
}

type CreateAPIKeyResult struct {
	APIKey domain.APIKey `json:"api_key"`
	Secret string        `json:"secret"`
}

type ListAPIKeysRequest struct {
	Role         string `json:"role,omitempty"`
	State        string `json:"state,omitempty"`
	NameContains string `json:"name_contains,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Offset       int    `json:"offset,omitempty"`
	Cursor       string `json:"cursor,omitempty"`
}

type ListAPIKeysResult struct {
	Items      []domain.APIKey `json:"items"`
	Total      int             `json:"total"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type ListAPIKeyAuditEventsRequest struct {
	APIKeyID      string `json:"api_key_id,omitempty"`
	ActorAPIKeyID string `json:"actor_api_key_id,omitempty"`
	Action        string `json:"action,omitempty"`
	OwnerType     string `json:"owner_type,omitempty"`
	OwnerID       string `json:"owner_id,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Offset        int    `json:"offset,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
}

type ListAPIKeyAuditEventsResult struct {
	Items      []domain.APIKeyAuditEvent `json:"items"`
	Total      int                       `json:"total"`
	Limit      int                       `json:"limit"`
	Offset     int                       `json:"offset"`
	NextCursor string                    `json:"next_cursor,omitempty"`
}

type listCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func NewAPIKeyService(s store.Repository) *APIKeyService {
	return &APIKeyService{store: s}
}

func (s *APIKeyService) CreateAPIKey(tenantID, actorAPIKeyID string, req CreateAPIKeyRequest) (CreateAPIKeyResult, error) {
	tenantID = normalizeTenantID(tenantID)
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)

	tenant, err := s.store.GetTenant(tenantID)
	if err != nil {
		if err == store.ErrNotFound {
			return CreateAPIKeyResult{}, fmt.Errorf("%w: tenant not found", ErrValidation)
		}
		return CreateAPIKeyResult{}, err
	}
	if tenant.Status != domain.TenantStatusActive {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: tenant status must be active", ErrValidation)
	}

	role, err := normalizeAPIKeyRole(req.Role)
	if err != nil {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: invalid role", ErrValidation)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	ownerType, err := normalizeWorkspaceCredentialOwnerType(req.OwnerType)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	purpose := strings.TrimSpace(req.Purpose)
	environment := strings.TrimSpace(req.Environment)
	createdByUserID := strings.TrimSpace(req.CreatedByUserID)
	actorPlatformAPIKeyID := strings.TrimSpace(req.ActorPlatformAPIKeyID)

	secret, err := generateAPIKeySecret()
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	hashed := hashAPIKey(secret)
	prefix := keyPrefixFromHash(hashed)

	created, err := s.store.CreateAPIKey(domain.APIKey{
		KeyPrefix:             prefix,
		KeyHash:               hashed,
		Name:                  name,
		Role:                  role,
		TenantID:              tenantID,
		OwnerType:             ownerType,
		OwnerID:               ownerID,
		Purpose:               purpose,
		Environment:           environment,
		CreatedByUserID:       createdByUserID,
		CreatedByPlatformUser: req.CreatedByPlatformUser,
		CreatedAt:             time.Now().UTC(),
		ExpiresAt:             req.ExpiresAt,
	})
	if err != nil {
		if err == store.ErrAlreadyExists || err == store.ErrDuplicateKey {
			return CreateAPIKeyResult{}, fmt.Errorf("%w: api key collision, retry", ErrValidation)
		}
		return CreateAPIKeyResult{}, err
	}

	metadata := map[string]any{
		"role":        role,
		"name":        name,
		"owner_type":  ownerType,
		"owner_id":    ownerID,
		"purpose":     purpose,
		"environment": environment,
	}
	if createdByUserID != "" {
		metadata["created_by_user_id"] = createdByUserID
	}
	if req.CreatedByPlatformUser {
		metadata["created_by_platform_user"] = true
	}
	if actorPlatformAPIKeyID != "" {
		metadata["actor_platform_api_key_id"] = actorPlatformAPIKeyID
	}
	if req.ExpiresAt != nil {
		metadata["expires_at"] = req.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}

	_, err = s.store.CreateAPIKeyAuditEvent(domain.APIKeyAuditEvent{
		TenantID:      tenantID,
		APIKeyID:      created.ID,
		ActorAPIKeyID: actorAPIKeyID,
		Action:        apiKeyAuditActionCreated,
		Metadata:      metadata,
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return CreateAPIKeyResult{}, fmt.Errorf("create api key audit event: %w", err)
	}

	return CreateAPIKeyResult{
		APIKey: created,
		Secret: secret,
	}, nil
}

func (s *APIKeyService) ListAPIKeys(tenantID string, req ListAPIKeysRequest) (ListAPIKeysResult, error) {
	role, err := normalizeRoleFilter(req.Role)
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	state, err := normalizeStateFilter(req.State)
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	cursorCreated, cursorID, err := decodeCursor(req.Cursor)
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	if cursorCreated != nil && offset > 0 {
		return ListAPIKeysResult{}, fmt.Errorf("%w: offset cannot be used with cursor", ErrValidation)
	}

	out, err := s.store.ListAPIKeys(store.APIKeyListFilter{
		TenantID:      normalizeTenantID(tenantID),
		Role:          role,
		State:         state,
		NameContains:  strings.TrimSpace(req.NameContains),
		Limit:         limit,
		Offset:        offset,
		CursorID:      cursorID,
		CursorCreated: cursorCreated,
		ReferenceTime: time.Now().UTC(),
	})
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	nextCursor, err := encodeCursor(out.NextCursorCreated, out.NextCursorID)
	if err != nil {
		return ListAPIKeysResult{}, err
	}
	return ListAPIKeysResult{
		Items:      out.Items,
		Total:      out.Total,
		Limit:      out.Limit,
		Offset:     out.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *APIKeyService) RevokeAPIKey(tenantID, actorAPIKeyID, id string) (domain.APIKey, error) {
	tenantID = normalizeTenantID(tenantID)
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	id = strings.TrimSpace(id)
	current, err := s.store.GetAPIKeyByID(tenantID, id)
	if err != nil {
		return domain.APIKey{}, err
	}
	key, err := s.store.RevokeAPIKey(tenantID, id, time.Now().UTC())
	if err != nil {
		return domain.APIKey{}, err
	}
	_, err = s.store.CreateAPIKeyAuditEvent(domain.APIKeyAuditEvent{
		TenantID:      tenantID,
		APIKeyID:      key.ID,
		ActorAPIKeyID: actorAPIKeyID,
		Action:        apiKeyAuditActionRevoked,
		Metadata: map[string]any{
			"owner_type":        current.OwnerType,
			"owner_id":          current.OwnerID,
			"purpose":           current.Purpose,
			"environment":       current.Environment,
			"revocation_reason": current.RevocationReason,
		},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return domain.APIKey{}, fmt.Errorf("create api key audit event: %w", err)
	}
	return key, nil
}

func (s *APIKeyService) RotateAPIKey(tenantID, actorAPIKeyID, id string) (CreateAPIKeyResult, error) {
	tenantID = normalizeTenantID(tenantID)
	actorAPIKeyID = strings.TrimSpace(actorAPIKeyID)
	id = strings.TrimSpace(id)
	if id == "" {
		return CreateAPIKeyResult{}, fmt.Errorf("%w: id is required", ErrValidation)
	}

	current, err := s.store.GetAPIKeyByID(tenantID, id)
	if err != nil {
		return CreateAPIKeyResult{}, err
	}

	if _, err := s.RevokeAPIKey(tenantID, actorAPIKeyID, id); err != nil {
		return CreateAPIKeyResult{}, err
	}

	rotated, err := s.CreateAPIKey(tenantID, actorAPIKeyID, CreateAPIKeyRequest{
		Name:                  current.Name,
		Role:                  current.Role,
		ExpiresAt:             current.ExpiresAt,
		OwnerType:             current.OwnerType,
		OwnerID:               current.OwnerID,
		Purpose:               current.Purpose,
		Environment:           current.Environment,
		CreatedByUserID:       current.CreatedByUserID,
		CreatedByPlatformUser: current.CreatedByPlatformUser,
	})
	if err != nil {
		return CreateAPIKeyResult{}, err
	}
	_, err = s.store.CreateAPIKeyAuditEvent(domain.APIKeyAuditEvent{
		TenantID:      tenantID,
		APIKeyID:      id,
		ActorAPIKeyID: actorAPIKeyID,
		Action:        apiKeyAuditActionRotated,
		Metadata: map[string]any{
			"new_api_key_id": rotated.APIKey.ID,
			"owner_type":     current.OwnerType,
			"owner_id":       current.OwnerID,
			"purpose":        current.Purpose,
			"environment":    current.Environment,
		},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return CreateAPIKeyResult{}, fmt.Errorf("create api key audit event: %w", err)
	}
	return rotated, nil
}

func (s *APIKeyService) ListAPIKeyAuditEvents(tenantID string, req ListAPIKeyAuditEventsRequest) (ListAPIKeyAuditEventsResult, error) {
	limit, offset, err := normalizeListWindow(req.Limit, req.Offset)
	if err != nil {
		return ListAPIKeyAuditEventsResult{}, err
	}
	action, err := normalizeAuditAction(req.Action)
	if err != nil {
		return ListAPIKeyAuditEventsResult{}, err
	}
	cursorCreated, cursorID, err := decodeCursor(req.Cursor)
	if err != nil {
		return ListAPIKeyAuditEventsResult{}, err
	}
	if cursorCreated != nil && offset > 0 {
		return ListAPIKeyAuditEventsResult{}, fmt.Errorf("%w: offset cannot be used with cursor", ErrValidation)
	}

	out, err := s.store.ListAPIKeyAuditEvents(store.APIKeyAuditFilter{
		TenantID:      normalizeTenantID(tenantID),
		APIKeyID:      strings.TrimSpace(req.APIKeyID),
		ActorAPIKeyID: strings.TrimSpace(req.ActorAPIKeyID),
		Action:        action,
		OwnerType:     strings.TrimSpace(req.OwnerType),
		OwnerID:       strings.TrimSpace(req.OwnerID),
		Limit:         limit,
		Offset:        offset,
		CursorID:      cursorID,
		CursorCreated: cursorCreated,
	})
	if err != nil {
		return ListAPIKeyAuditEventsResult{}, err
	}
	nextCursor, err := encodeCursor(out.NextCursorCreated, out.NextCursorID)
	if err != nil {
		return ListAPIKeyAuditEventsResult{}, err
	}
	return ListAPIKeyAuditEventsResult{
		Items:      out.Items,
		Total:      out.Total,
		Limit:      out.Limit,
		Offset:     out.Offset,
		NextCursor: nextCursor,
	}, nil
}

func (s *APIKeyService) GenerateAPIKeyAuditCSV(tenantID string, req ListAPIKeyAuditEventsRequest) (string, error) {
	chunkSize := req.Limit
	if chunkSize <= 0 || chunkSize > maxListLimit {
		chunkSize = maxListLimit
	}

	var b strings.Builder
	writer := csv.NewWriter(&b)
	if err := writer.Write([]string{"id", "tenant_id", "api_key_id", "actor_api_key_id", "action", "metadata", "created_at"}); err != nil {
		return "", err
	}

	cursor := strings.TrimSpace(req.Cursor)
	for {
		page, err := s.ListAPIKeyAuditEvents(tenantID, ListAPIKeyAuditEventsRequest{
			APIKeyID:      req.APIKeyID,
			ActorAPIKeyID: req.ActorAPIKeyID,
			Action:        req.Action,
			OwnerType:     req.OwnerType,
			OwnerID:       req.OwnerID,
			Limit:         chunkSize,
			Cursor:        cursor,
		})
		if err != nil {
			return "", err
		}
		for _, item := range page.Items {
			metadataRaw, err := json.Marshal(item.Metadata)
			if err != nil {
				return "", err
			}
			if err := writer.Write([]string{
				item.ID,
				item.TenantID,
				item.APIKeyID,
				item.ActorAPIKeyID,
				item.Action,
				string(metadataRaw),
				item.CreatedAt.UTC().Format(time.RFC3339Nano),
			}); err != nil {
				return "", err
			}
		}

		if page.NextCursor == "" {
			break
		}
		if page.NextCursor == cursor {
			return "", fmt.Errorf("invalid cursor progression")
		}
		cursor = page.NextCursor
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return b.String(), nil
}

func generateAPIKeySecret() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key secret: %w", err)
	}
	return "lgo_" + hex.EncodeToString(buf), nil
}

func normalizeWorkspaceCredentialOwnerType(raw string) (string, error) {
	ownerType := strings.ToLower(strings.TrimSpace(raw))
	switch ownerType {
	case "", "workspace_credential":
		return "workspace_credential", nil
	case "bootstrap", "break_glass", "service_account":
		return ownerType, nil
	default:
		return "", fmt.Errorf("%w: invalid owner_type", ErrValidation)
	}
}

func normalizeAPIKeyRole(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "reader":
		return "reader", nil
	case "writer":
		return "writer", nil
	case "admin":
		return "admin", nil
	default:
		return "", fmt.Errorf("unsupported role")
	}
}

func normalizeRoleFilter(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	normalized, err := normalizeAPIKeyRole(raw)
	if err != nil {
		return "", fmt.Errorf("%w: invalid role filter", ErrValidation)
	}
	return normalized, nil
}

func normalizeStateFilter(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "":
		return "", nil
	case apiKeyStateActive, apiKeyStateRevoked, apiKeyStateExpired:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: invalid state filter", ErrValidation)
	}
}

func normalizeAuditAction(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "":
		return "", nil
	case apiKeyAuditActionCreated, apiKeyAuditActionRevoked, apiKeyAuditActionRotated:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: invalid action filter", ErrValidation)
	}
}

func normalizeListWindow(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultListLimit
	}
	if limit < 1 || limit > maxListLimit {
		return 0, 0, fmt.Errorf("%w: limit must be between 1 and %d", ErrValidation, maxListLimit)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset must be >= 0", ErrValidation)
	}
	return limit, offset, nil
}

func decodeCursor(raw string) (*time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	var c listCursor
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	c.ID = strings.TrimSpace(c.ID)
	if c.ID == "" || c.CreatedAt.IsZero() {
		return nil, "", fmt.Errorf("%w: invalid cursor", ErrValidation)
	}
	t := c.CreatedAt.UTC()
	return &t, c.ID, nil
}

func encodeCursor(createdAt *time.Time, id string) (string, error) {
	id = strings.TrimSpace(id)
	if createdAt == nil || id == "" {
		return "", nil
	}
	payload, err := json.Marshal(listCursor{
		CreatedAt: createdAt.UTC(),
		ID:        id,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func keyPrefixFromHash(hash string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) <= 16 {
		return hash
	}
	return hash[:16]
}
