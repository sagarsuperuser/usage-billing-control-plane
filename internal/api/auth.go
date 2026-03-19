package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type Role string
type Scope string
type PlatformRole string

const (
	RoleReader Role = "reader"
	RoleWriter Role = "writer"
	RoleAdmin  Role = "admin"

	ScopeTenant   Scope = "tenant"
	ScopePlatform Scope = "platform"

	PlatformRoleAdmin PlatformRole = "platform_admin"

	apiKeyHeader       = "X-API-Key"
	apiKeyPrefixLength = 16
	defaultTenantID    = "default"
)

var (
	errUnauthorized  = errors.New("unauthorized")
	errTenantBlocked = errors.New("tenant blocked")
)

type tenantBlockedError struct {
	TenantID string
	Status   domain.TenantStatus
}

func (e tenantBlockedError) Error() string {
	return fmt.Sprintf("%s: tenant %q status=%s", errTenantBlocked.Error(), normalizeTenantID(e.TenantID), e.Status)
}

func (e tenantBlockedError) Unwrap() error { return errTenantBlocked }

type Principal struct {
	SubjectType  string
	SubjectID    string
	UserEmail    string
	Scope        Scope
	Role         Role
	PlatformRole PlatformRole
	TenantID     string
	APIKeyID     string
}

type APIKeyAuthorizer interface {
	Authorize(r *http.Request) (Principal, error)
}

type APIKeyStore interface {
	CreateAPIKey(input domain.APIKey) (domain.APIKey, error)
	GetTenant(id string) (domain.Tenant, error)
	GetAPIKeyByPrefix(prefix string) (domain.APIKey, error)
	GetActiveAPIKeyByPrefix(prefix string, at time.Time) (domain.APIKey, error)
	GetServiceAccount(tenantID, id string) (domain.ServiceAccount, error)
	TouchAPIKeyLastUsed(id string, usedAt time.Time) error
	CreatePlatformAPIKey(input domain.PlatformAPIKey) (domain.PlatformAPIKey, error)
	GetPlatformAPIKeyByPrefix(prefix string) (domain.PlatformAPIKey, error)
	GetActivePlatformAPIKeyByPrefix(prefix string, at time.Time) (domain.PlatformAPIKey, error)
	TouchPlatformAPIKeyLastUsed(id string, usedAt time.Time) error
}

type BootstrapResult struct {
	Created  int
	Existing int
}

type StaticAPIKeyAuthorizer struct {
	keys map[string]Role
}

func NewStaticAPIKeyAuthorizer(keys map[string]Role) (*StaticAPIKeyAuthorizer, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("at least one api key is required")
	}
	cleaned := make(map[string]Role, len(keys))
	for key, role := range keys {
		k := strings.TrimSpace(key)
		if k == "" {
			return nil, fmt.Errorf("api key cannot be empty")
		}
		parsedRole, err := ParseRole(string(role))
		if err != nil {
			return nil, err
		}
		cleaned[k] = parsedRole
	}
	return &StaticAPIKeyAuthorizer{keys: cleaned}, nil
}

func (a *StaticAPIKeyAuthorizer) Authorize(r *http.Request) (Principal, error) {
	if a == nil {
		return Principal{}, errUnauthorized
	}
	key := strings.TrimSpace(r.Header.Get(apiKeyHeader))
	if key == "" {
		return Principal{}, errUnauthorized
	}
	role, ok := a.keys[key]
	if !ok {
		return Principal{}, errUnauthorized
	}
	return Principal{
		SubjectType: "api_key",
		Scope:       ScopeTenant,
		Role:        role,
		TenantID:    defaultTenantID,
	}, nil
}

type DBAPIKeyAuthorizer struct {
	store APIKeyStore
}

func NewDBAPIKeyAuthorizer(keyStore APIKeyStore) (*DBAPIKeyAuthorizer, error) {
	if keyStore == nil {
		return nil, fmt.Errorf("api key store is required")
	}
	return &DBAPIKeyAuthorizer{store: keyStore}, nil
}

func (a *DBAPIKeyAuthorizer) Authorize(r *http.Request) (Principal, error) {
	if a == nil || a.store == nil {
		return Principal{}, errUnauthorized
	}

	rawKey := strings.TrimSpace(r.Header.Get(apiKeyHeader))
	if rawKey == "" {
		return Principal{}, errUnauthorized
	}

	hashed := HashAPIKey(rawKey)
	prefix := KeyPrefixFromHash(hashed)
	record, err := a.store.GetActiveAPIKeyByPrefix(prefix, time.Now().UTC())
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return Principal{}, fmt.Errorf("load active api key: %w", err)
		}
	} else {
		if !hashMatches(record.KeyHash, hashed) {
			return Principal{}, errUnauthorized
		}

		role, err := ParseRole(record.Role)
		if err != nil {
			return Principal{}, fmt.Errorf("invalid stored role for key prefix %q: %w", prefix, err)
		}

		tenant, err := a.store.GetTenant(record.TenantID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return Principal{}, errUnauthorized
			}
			return Principal{}, fmt.Errorf("load tenant: %w", err)
		}
		if tenant.Status != domain.TenantStatusActive {
			return Principal{}, tenantBlockedError{
				TenantID: record.TenantID,
				Status:   tenant.Status,
			}
		}
		if requiresServiceAccountLifecycle(record) {
			serviceAccount, err := a.loadCredentialServiceAccount(record)
			if err != nil {
				return Principal{}, err
			}
			if serviceAccount.Status != domain.ServiceAccountStatusActive {
				return Principal{}, errUnauthorized
			}
		}

		_ = a.store.TouchAPIKeyLastUsed(record.ID, time.Now().UTC())
		return Principal{
			SubjectType: "api_key",
			SubjectID:   record.ID,
			Scope:       ScopeTenant,
			Role:        role,
			TenantID:    normalizeTenantID(record.TenantID),
			APIKeyID:    record.ID,
		}, nil
	}

	platformRecord, err := a.store.GetActivePlatformAPIKeyByPrefix(prefix, time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Principal{}, errUnauthorized
		}
		return Principal{}, fmt.Errorf("load active platform api key: %w", err)
	}
	if !hashMatches(platformRecord.KeyHash, hashed) {
		return Principal{}, errUnauthorized
	}
	platformRole, err := ParsePlatformRole(platformRecord.Role)
	if err != nil {
		return Principal{}, fmt.Errorf("invalid stored platform role for key prefix %q: %w", prefix, err)
	}
	_ = a.store.TouchPlatformAPIKeyLastUsed(platformRecord.ID, time.Now().UTC())
	return Principal{
		SubjectType:  "api_key",
		SubjectID:    platformRecord.ID,
		Scope:        ScopePlatform,
		PlatformRole: platformRole,
		APIKeyID:     platformRecord.ID,
	}, nil
}

func requiresServiceAccountLifecycle(record domain.APIKey) bool {
	switch strings.TrimSpace(record.OwnerType) {
	case "service_account", "bootstrap", "break_glass":
		return true
	default:
		return false
	}
}

func (a *DBAPIKeyAuthorizer) loadCredentialServiceAccount(record domain.APIKey) (domain.ServiceAccount, error) {
	ownerID := strings.TrimSpace(record.OwnerID)
	switch strings.TrimSpace(record.OwnerType) {
	case "service_account":
		if ownerID == "" {
			return domain.ServiceAccount{}, errUnauthorized
		}
	case "bootstrap", "break_glass":
		if ownerID == "" {
			return domain.ServiceAccount{Status: domain.ServiceAccountStatusActive}, nil
		}
	}
	serviceAccount, err := a.store.GetServiceAccount(record.TenantID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.ServiceAccount{}, errUnauthorized
		}
		return domain.ServiceAccount{}, fmt.Errorf("load service account: %w", err)
	}
	return serviceAccount, nil
}

func BootstrapAPIKeysFromConfig(keyStore APIKeyStore, raw string) (BootstrapResult, error) {
	if keyStore == nil {
		return BootstrapResult{}, fmt.Errorf("api key store is required")
	}

	parsed, err := ParseAPIKeysConfig(raw)
	if err != nil {
		return BootstrapResult{}, err
	}

	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := BootstrapResult{}
	now := time.Now().UTC()
	for _, rawKey := range keys {
		role := parsed[rawKey]
		hashed := HashAPIKey(rawKey)
		prefix := KeyPrefixFromHash(hashed)

		existing, err := keyStore.GetAPIKeyByPrefix(prefix)
		if err == nil {
			if !hashMatches(existing.KeyHash, hashed) {
				return BootstrapResult{}, fmt.Errorf("api key collision at prefix %q", prefix)
			}
			existingRole, err := ParseRole(existing.Role)
			if err != nil {
				return BootstrapResult{}, fmt.Errorf("invalid role for existing key %q: %w", prefix, err)
			}
			if existingRole != role {
				return BootstrapResult{}, fmt.Errorf("api key %q already exists with role %q, expected %q", prefix, existingRole, role)
			}
			if normalizeTenantID(existing.TenantID) != defaultTenantID {
				return BootstrapResult{}, fmt.Errorf("api key %q already exists for tenant %q", prefix, normalizeTenantID(existing.TenantID))
			}
			if existing.RevokedAt != nil {
				return BootstrapResult{}, fmt.Errorf("api key %q is revoked", prefix)
			}
			result.Existing++
			continue
		}
		if !errors.Is(err, store.ErrNotFound) {
			return BootstrapResult{}, fmt.Errorf("lookup api key %q: %w", prefix, err)
		}

		_, err = keyStore.CreateAPIKey(domain.APIKey{
			KeyPrefix: prefix,
			KeyHash:   hashed,
			Name:      "bootstrap-" + prefix,
			Role:      string(role),
			TenantID:  defaultTenantID,
			CreatedAt: now,
		})
		if err != nil {
			if errors.Is(err, store.ErrAlreadyExists) || errors.Is(err, store.ErrDuplicateKey) {
				result.Existing++
				continue
			}
			return BootstrapResult{}, fmt.Errorf("create api key %q: %w", prefix, err)
		}
		result.Created++
	}

	return result, nil
}

func ParseAPIKeysConfig(raw string) (map[string]Role, error) {
	parts := strings.Split(raw, ",")
	out := make(map[string]Role, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		pair := strings.SplitN(entry, ":", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid API_KEYS entry %q: expected key:role", entry)
		}
		key := strings.TrimSpace(pair[0])
		if key == "" {
			return nil, fmt.Errorf("invalid API_KEYS entry %q: key cannot be empty", entry)
		}
		role, err := ParseRole(pair[1])
		if err != nil {
			return nil, fmt.Errorf("invalid API_KEYS entry %q: %w", entry, err)
		}
		out[key] = role
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("API_KEYS must contain at least one key:role pair")
	}
	return out, nil
}

func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func KeyPrefixFromHash(hash string) string {
	normalized := normalizeHex(hash)
	if len(normalized) <= apiKeyPrefixLength {
		return normalized
	}
	return normalized[:apiKeyPrefixLength]
}

func hashMatches(storedHash, computedHash string) bool {
	normalized := normalizeHex(storedHash)
	if normalized == "" || len(normalized) != len(computedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(normalized), []byte(computedHash)) == 1
}

func normalizeHex(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

type principalContextKey struct{}

var requestPrincipalContextKey principalContextKey

func withPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, requestPrincipalContextKey, principal)
}

func principalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	v := ctx.Value(requestPrincipalContextKey)
	principal, ok := v.(Principal)
	return principal, ok
}

func ParsePlatformRole(raw string) (PlatformRole, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(PlatformRoleAdmin):
		return PlatformRoleAdmin, nil
	default:
		return "", fmt.Errorf("unsupported platform role %q", raw)
	}
}

func normalizeTenantID(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return defaultTenantID
	}
	return v
}

func normalizeOptionalTenantID(v string) string {
	return strings.TrimSpace(v)
}

func ParseRole(raw string) (Role, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RoleReader):
		return RoleReader, nil
	case string(RoleWriter):
		return RoleWriter, nil
	case string(RoleAdmin):
		return RoleAdmin, nil
	default:
		return "", fmt.Errorf("unsupported role %q", raw)
	}
}

func roleAllows(actual, required Role) bool {
	return roleRank(actual) >= roleRank(required)
}

func roleRank(role Role) int {
	switch role {
	case RoleReader:
		return 1
	case RoleWriter:
		return 2
	case RoleAdmin:
		return 3
	default:
		return 0
	}
}
