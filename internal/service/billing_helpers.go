package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Organization bootstrapper (replaces Lago org provisioning with a no-op).
// ---------------------------------------------------------------------------

type OrganizationBootstrapper interface {
	BootstrapOrganization(ctx context.Context, name string) (OrganizationBootstrapResult, error)
}

type OrganizationBootstrapResult struct {
	OrganizationID string
	APIKey         string
}

// NoopOrganizationBootstrapper is used when Lago is not present.
// It returns a synthetic organization ID since we no longer provision Lago orgs.
type NoopOrganizationBootstrapper struct{}

func (b NoopOrganizationBootstrapper) BootstrapOrganization(_ context.Context, name string) (OrganizationBootstrapResult, error) {
	return OrganizationBootstrapResult{
		OrganizationID: fmt.Sprintf("org_%s", strings.ReplaceAll(strings.ToLower(name), " ", "_")),
		APIKey:         "",
	}, nil
}

// ---------------------------------------------------------------------------
// Context helpers (formerly used by Lago transport resolver).
// Now pass-through: our Stripe adapters resolve keys internally.
// ---------------------------------------------------------------------------

type billingScopeKey struct{}

type billingScope struct {
	TenantID       string
	OrganizationID string
}

func ContextWithBillingScope(ctx context.Context, tenantID, organizationID string) context.Context {
	return context.WithValue(ctx, billingScopeKey{}, billingScope{
		TenantID:       tenantID,
		OrganizationID: organizationID,
	})
}

func ContextWithBillingTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, billingScopeKey{}, billingScope{
		TenantID: tenantID,
	})
}

// ---------------------------------------------------------------------------
// JSON parsing helpers (formerly in lago_webhook_projection.go).
// Used by http_invoices.go and payment_reconcile_service.go.
// ---------------------------------------------------------------------------

func stringValue(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimSpace(s)
}

func boolPtr(v any) *bool {
	if v == nil {
		return nil
	}
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		val := strings.ToLower(strings.TrimSpace(b)) == "true"
		return &val
	default:
		return nil
	}
}

func int64Ptr(v any) *int64 {
	if v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return &i
	case int64:
		return &n
	case int:
		i := int64(n)
		return &i
	default:
		return nil
	}
}

func firstTimestamp(values ...any) time.Time {
	for _, v := range values {
		switch t := v.(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(t)); err == nil {
				return parsed
			}
			if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(t)); err == nil {
				return parsed
			}
		case time.Time:
			if !t.IsZero() {
				return t
			}
		}
	}
	return time.Time{}
}
