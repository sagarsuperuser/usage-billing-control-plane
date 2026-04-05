package api

import (
	"log/slog"
	"strings"

	"github.com/alexedwards/scs/v2"

	"usage-billing-control-plane/internal/service"
)

type ServerOption func(*Server)

func WithMetricsProvider(provider func() map[string]any) ServerOption {
	return func(s *Server) {
		s.metricsFn = provider
	}
}

func WithReadinessCheck(check func() error) ServerOption {
	return func(s *Server) {
		s.readinessFn = check
	}
}

func WithAPIKeyAuthorizer(authorizer APIKeyAuthorizer) ServerOption {
	return func(s *Server) {
		s.authorizer = authorizer
	}
}

func WithSessionManager(sessionManager *scs.SessionManager) ServerOption {
	return func(s *Server) {
		s.sessionManager = sessionManager
	}
}

func WithAuditExportService(auditExportSvc *service.AuditExportService) ServerOption {
	return func(s *Server) {
		s.auditExportSvc = auditExportSvc
	}
}

func WithMeterSyncAdapter(adapter service.MeterSyncAdapter) ServerOption {
	return func(s *Server) {
		s.meterSyncAdapter = adapter
	}
}

func WithInvoiceBillingAdapter(adapter service.InvoiceBillingAdapter) ServerOption {
	return func(s *Server) {
		s.invoiceBillingAdapter = adapter
	}
}

func WithCustomerBillingAdapter(adapter service.CustomerBillingAdapter) ServerOption {
	return func(s *Server) {
		s.customerBillingAdapter = adapter
	}
}

func WithPlanSyncAdapter(adapter service.PlanSyncAdapter) ServerOption {
	return func(s *Server) {
		s.planSyncAdapter = adapter
	}
}

func WithSubscriptionSyncAdapter(adapter service.SubscriptionSyncAdapter) ServerOption {
	return func(s *Server) {
		s.subscriptionSyncAdapter = adapter
	}
}

func WithUsageSyncAdapter(adapter service.UsageSyncAdapter) ServerOption {
	return func(s *Server) {
		s.usageSyncAdapter = adapter
	}
}

func WithBillingProviderConnectionService(svc *service.BillingProviderConnectionService) ServerOption {
	return func(s *Server) {
		s.billingProviderConnectionService = svc
	}
}

func WithBrowserUserAuthService(svc *service.BrowserUserAuthService) ServerOption {
	return func(s *Server) {
		s.browserUserAuthService = svc
	}
}

func WithBrowserSSOService(svc *service.BrowserSSOService) ServerOption {
	return func(s *Server) {
		s.browserSSOService = svc
	}
}

func WithWorkspaceAccessService(svc *service.WorkspaceAccessService) ServerOption {
	return func(s *Server) {
		s.workspaceAccessService = svc
	}
}

func WithWorkspaceInvitationEmailSender(sender service.WorkspaceInvitationEmailSender) ServerOption {
	return func(s *Server) {
		s.workspaceInvitationEmailSender = sender
	}
}

func WithNotificationService(svc *service.NotificationService) ServerOption {
	return func(s *Server) {
		s.notificationService = svc
	}
}

func WithPasswordResetService(svc *service.PasswordResetService) ServerOption {
	return func(s *Server) {
		s.passwordResetService = svc
	}
}

func WithPasswordResetEmailSender(sender service.PasswordResetEmailSender) ServerOption {
	return func(s *Server) {
		s.passwordResetEmailSender = sender
	}
}

func WithUIPublicBaseURL(baseURL string) ServerOption {
	return func(s *Server) {
		s.uiPublicBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
}

func WithPaymentStatusService(svc *service.PaymentStatusService) ServerOption {
	return func(s *Server) {
		s.paymentStatusSvc = svc
	}
}

func WithStripeWebhookService(stripeWebhookSvc *service.StripeWebhookService) ServerOption {
	return func(s *Server) {
		s.stripeWebhookSvc = stripeWebhookSvc
	}
}

func WithInvoicePDFService(svc *service.InvoicePDFService) ServerOption {
	return func(s *Server) {
		s.invoicePDFService = svc
	}
}

func WithInvoiceGenerationService(svc *service.InvoiceGenerationService) ServerOption {
	return func(s *Server) {
		s.invoiceGenerationService = svc
	}
}

func WithStripeWebhookSecret(secret string) ServerOption {
	return func(s *Server) {
		s.stripeWebhookSecret = secret
	}
}

func WithLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		s.logger = logger
	}
}

func WithTaxSyncAdapter(adapter service.TaxSyncAdapter) ServerOption {
	return func(s *Server) {
		s.taxSyncAdapter = adapter
	}
}

func WithRateLimiter(rateLimiter RateLimiter, failOpen bool, loginFailOpen bool) ServerOption {
	return func(s *Server) {
		s.rateLimiter = rateLimiter
		s.rateLimitFailOpen = failOpen
		s.rateLimitLoginFailOpen = loginFailOpen
	}
}

func WithSessionOriginPolicy(require bool, allowedOrigins []string) ServerOption {
	return func(s *Server) {
		s.requireSessionOriginCheck = require
		s.allowedSessionOrigins = make(map[string]struct{}, len(allowedOrigins))
		for _, origin := range allowedOrigins {
			normalized, ok := normalizeAbsoluteOrigin(origin)
			if !ok {
				continue
			}
			s.allowedSessionOrigins[normalized] = struct{}{}
		}
	}
}
