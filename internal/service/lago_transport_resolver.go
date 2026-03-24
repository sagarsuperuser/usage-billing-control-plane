package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/store"
)

type lagoTransportScopeContextKey struct{}

type lagoTransportScope struct {
	TenantID       string
	OrganizationID string
}

func ContextWithLagoTenant(ctx context.Context, tenantID string) context.Context {
	return ContextWithLagoScope(ctx, tenantID, "")
}

func ContextWithLagoScope(ctx context.Context, tenantID, organizationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	scope := lagoTransportScope{
		TenantID:       strings.TrimSpace(tenantID),
		OrganizationID: strings.TrimSpace(organizationID),
	}
	if scope.TenantID == "" && scope.OrganizationID == "" {
		return ctx
	}
	return context.WithValue(ctx, lagoTransportScopeContextKey{}, scope)
}

func lagoScopeFromContext(ctx context.Context) lagoTransportScope {
	if ctx == nil {
		return lagoTransportScope{}
	}
	scope, _ := ctx.Value(lagoTransportScopeContextKey{}).(lagoTransportScope)
	scope.TenantID = strings.TrimSpace(scope.TenantID)
	scope.OrganizationID = strings.TrimSpace(scope.OrganizationID)
	return scope
}

type LagoTenantCredentialRepository interface {
	GetTenant(id string) (domain.Tenant, error)
	GetTenantByLagoOrganizationID(organizationID string) (domain.Tenant, error)
}

type LagoTransportResolver interface {
	Resolve(ctx context.Context, tenantID, organizationID string) (*LagoHTTPTransport, error)
}

type staticLagoTransportResolver struct {
	transport *LagoHTTPTransport
}

func newStaticLagoTransportResolver(transport *LagoHTTPTransport) LagoTransportResolver {
	return &staticLagoTransportResolver{transport: transport}
}

func (r *staticLagoTransportResolver) Resolve(_ context.Context, _ string, _ string) (*LagoHTTPTransport, error) {
	if r == nil || r.transport == nil {
		return nil, fmt.Errorf("%w: lago transport is required", ErrValidation)
	}
	return r.transport, nil
}

type TenantBackedLagoTransportResolver struct {
	repo             LagoTenantCredentialRepository
	secretStore      LagoTenantAPIKeySecretStore
	baseURL          string
	timeout          time.Duration
	defaultTransport *LagoHTTPTransport
	cache            sync.Map
}

func NewTenantBackedLagoTransportResolver(repo LagoTenantCredentialRepository, secretStore LagoTenantAPIKeySecretStore, cfg LagoClientConfig) (*TenantBackedLagoTransportResolver, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: lago base url is required", ErrValidation)
	}
	resolver := &TenantBackedLagoTransportResolver{
		repo:        repo,
		secretStore: secretStore,
		baseURL:     baseURL,
		timeout:     cfg.Timeout,
	}
	if resolver.timeout <= 0 {
		resolver.timeout = defaultLagoHTTPTimeout
	}
	if apiKey := strings.TrimSpace(cfg.APIKey); apiKey != "" {
		transport, err := NewLagoHTTPTransport(LagoClientConfig{
			BaseURL: resolver.baseURL,
			APIKey:  apiKey,
			Timeout: resolver.timeout,
		})
		if err != nil {
			return nil, err
		}
		resolver.defaultTransport = transport
	}
	return resolver, nil
}

func (r *TenantBackedLagoTransportResolver) Resolve(ctx context.Context, tenantID, organizationID string) (*LagoHTTPTransport, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: lago transport resolver is required", ErrValidation)
	}
	tenantID = strings.TrimSpace(tenantID)
	organizationID = strings.TrimSpace(organizationID)
	if tenantID == "" && organizationID == "" {
		scope := lagoScopeFromContext(ctx)
		tenantID = scope.TenantID
		organizationID = scope.OrganizationID
	}
	if tenantID != "" {
		tenantID = normalizeTenantID(tenantID)
	}

	if r.repo != nil && tenantID != "" {
		tenant, err := r.repo.GetTenant(tenantID)
		if err == nil {
			apiKey, resolveErr := r.resolveTenantAPIKey(ctx, tenant)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if apiKey != "" {
				return r.transportForKey(apiKey)
			}
			if organizationID == "" {
				organizationID = strings.TrimSpace(tenant.LagoOrganizationID)
			}
		} else if err != store.ErrNotFound {
			return nil, err
		}
	}

	if r.repo != nil && organizationID != "" {
		tenant, err := r.repo.GetTenantByLagoOrganizationID(organizationID)
		if err == nil {
			apiKey, resolveErr := r.resolveTenantAPIKey(ctx, tenant)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if apiKey != "" {
				return r.transportForKey(apiKey)
			}
		} else if err != store.ErrNotFound {
			return nil, err
		}
	}

	if r.defaultTransport != nil {
		return r.defaultTransport, nil
	}
	return nil, fmt.Errorf("%w: no lago api key configured for tenant=%q organization=%q", ErrDependency, tenantID, organizationID)
}

func (r *TenantBackedLagoTransportResolver) resolveTenantAPIKey(ctx context.Context, tenant domain.Tenant) (string, error) {
	if secretRef := strings.TrimSpace(tenant.LagoAPIKeySecretRef); secretRef != "" {
		if r.secretStore == nil {
			return "", fmt.Errorf("%w: lago tenant api key secret store is required for tenant=%q", ErrDependency, normalizeTenantID(tenant.ID))
		}
		apiKey, err := r.secretStore.GetTenantLagoAPIKey(ctx, secretRef)
		if err != nil {
			return "", fmt.Errorf("resolve tenant lago api key for tenant=%q: %w", normalizeTenantID(tenant.ID), err)
		}
		return strings.TrimSpace(apiKey), nil
	}
	if strings.TrimSpace(tenant.ID) == "" {
		return "", nil
	}
	return "", fmt.Errorf("%w: no tenant lago api key secret ref configured for tenant=%q", ErrDependency, normalizeTenantID(tenant.ID))
}

func (r *TenantBackedLagoTransportResolver) transportForKey(apiKey string) (*LagoHTTPTransport, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%w: lago api key is required", ErrValidation)
	}
	if cached, ok := r.cache.Load(apiKey); ok {
		transport, _ := cached.(*LagoHTTPTransport)
		if transport != nil {
			return transport, nil
		}
	}
	transport, err := NewLagoHTTPTransport(LagoClientConfig{
		BaseURL: r.baseURL,
		APIKey:  apiKey,
		Timeout: r.timeout,
	})
	if err != nil {
		return nil, err
	}
	actual, _ := r.cache.LoadOrStore(apiKey, transport)
	resolved, _ := actual.(*LagoHTTPTransport)
	if resolved == nil {
		return transport, nil
	}
	return resolved, nil
}

func resolveLagoTransport(ctx context.Context, transport *LagoHTTPTransport, resolver LagoTransportResolver, tenantID, organizationID string) (*LagoHTTPTransport, error) {
	if resolver != nil {
		return resolver.Resolve(ctx, tenantID, organizationID)
	}
	if transport != nil {
		return transport, nil
	}
	return nil, fmt.Errorf("%w: lago transport is required", ErrValidation)
}
