package service

import "context"

type LagoTenantAPIKeySecretStore interface {
	PutTenantLagoAPIKey(ctx context.Context, tenantID, apiKey string) (string, error)
	GetTenantLagoAPIKey(ctx context.Context, secretRef string) (string, error)
	RotateTenantLagoAPIKey(ctx context.Context, secretRef, tenantID, apiKey string) (string, error)
	DeleteSecret(ctx context.Context, secretRef string) error
}
