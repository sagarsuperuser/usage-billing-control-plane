package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Context helpers for billing scope propagation.
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
