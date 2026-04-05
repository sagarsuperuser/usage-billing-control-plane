package appconfig

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/billingcheck"
	"usage-billing-control-plane/internal/billingcycle"
	"usage-billing-control-plane/internal/dunningflow"
	"usage-billing-control-plane/internal/paymentsync"
	"usage-billing-control-plane/internal/replay"
	"usage-billing-control-plane/internal/service"
)

type ServerConfig struct {
	RuntimeEnv       string
	ProductionLike   bool
	Port             string
	DB               DBConfig
	Roles            RoleConfig
	Temporal         TemporalConfig
	UISession        UISessionConfig
	SSO              SSOConfig
	RateLimit        RateLimitConfig
	BillingProviders BillingProviderConfig
	BillingChecks    BillingConnectionCheckConfig
	Payment          PaymentReconcileConfig
	Dunning          DunningConfig
	BillingCycle     BillingCycleConfig
	APIKeysRaw       string
	AuditExport      AuditExportConfig
	Email            InvitationEmailConfig
}

type DBConfig struct {
	URL                 string
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	PingTimeout         time.Duration
	QueryTimeout        time.Duration
	MigrationTimeout    time.Duration
	RunMigrationsOnBoot bool
}

type RoleConfig struct {
	RunAPIServer                       bool
	RunReplayWorker                    bool
	RunReplayDispatcher                bool
	RunBillingConnectionCheckWorker    bool
	RunBillingConnectionCheckScheduler bool
	RunPaymentReconcileWorker          bool
	RunPaymentReconcileScheduler       bool
	RunDunningWorker                   bool
	RunDunningScheduler                bool
	RunBillingCycleWorker              bool
	RunBillingCycleScheduler           bool
}

func (r RoleConfig) AnyEnabled() bool {
	return r.RunAPIServer || r.RunReplayWorker || r.RunReplayDispatcher ||
		r.RunBillingConnectionCheckWorker || r.RunBillingConnectionCheckScheduler ||
		r.RunPaymentReconcileWorker || r.RunPaymentReconcileScheduler ||
		r.RunDunningWorker || r.RunDunningScheduler ||
		r.RunBillingCycleWorker || r.RunBillingCycleScheduler
}

type TemporalConfig struct {
	Address               string
	Namespace             string
	ReplayTaskQueue       string
	ReplayDispatcherPoll  time.Duration
	ReplayDispatcherBatch int
}

type UISessionConfig struct {
	Lifetime           time.Duration
	CookieName         string
	CookieSecure       bool
	CookieSameSite     http.SameSite
	CookieSameSiteName string
	RequireOrigin      bool
	AllowedOrigins     []string
}

type SSOConfig struct {
	PublicBaseURL      string
	AutoProvisionUsers bool
	OIDCProviders      []OIDCProviderConfig
}

type OIDCProviderConfig struct {
	Key          string
	DisplayName  string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

type RateLimitConfig struct {
	Enabled       bool
	FailOpen      bool
	LoginFailOpen bool
	RedisURL      string
	KeyPrefix     string
	PolicyRates   map[string]string
}

type BillingProviderConfig struct {
	SecretStoreBackend         string
	SecretStoreAWSRegion       string
	SecretStoreAWSEndpoint     string
	SecretStorePrefix          string
	SecretStoreAccessKeyID     string
	SecretStoreSecretAccessKey string
	SecretStoreSessionToken    string
	DefaultOrganizationID  string
	StripeSuccessRedirectURL   string
	StripeWebhookSecret       string
}

type PaymentReconcileConfig struct {
	TaskQueue         string
	CronSchedule      string
	WorkflowID        string
	Batch             int
	StaleAfterSeconds int
}

type BillingConnectionCheckConfig struct {
	TaskQueue         string
	CronSchedule      string
	WorkflowID        string
	Batch             int
	StaleAfterSeconds int
}

type DunningConfig struct {
	TaskQueue    string
	CronSchedule string
	WorkflowID   string
	Batch        int
	TenantID     string
}

type BillingCycleConfig struct {
	TaskQueue    string
	CronSchedule string
	WorkflowID   string
	Batch        int
	TenantID     string
}

type AuditExportConfig struct {
	Enabled     bool
	S3          service.S3Config
	DownloadTTL time.Duration
	WorkerPoll  time.Duration
}

type InvitationEmailConfig struct {
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	FromEmail     string
	FromName      string
	ResetTokenTTL time.Duration
}

func LoadServerConfigFromEnv() (ServerConfig, error) {
	runtimeEnv := resolveRuntimeEnvironment()
	productionLike := isProductionLikeEnvironment(runtimeEnv)

	roles := RoleConfig{
		RunAPIServer:                       getBoolEnv("RUN_API_SERVER", true),
		RunReplayWorker:                    getBoolEnv("RUN_REPLAY_WORKER", true),
		RunReplayDispatcher:                getBoolEnv("RUN_REPLAY_DISPATCHER", true),
		RunBillingConnectionCheckWorker:    getBoolEnv("RUN_BILLING_CONNECTION_CHECK_WORKER", false),
		RunBillingConnectionCheckScheduler: getBoolEnv("RUN_BILLING_CONNECTION_CHECK_SCHEDULER", false),
		RunPaymentReconcileWorker:          getBoolEnv("RUN_PAYMENT_RECONCILE_WORKER", false),
		RunPaymentReconcileScheduler:       getBoolEnv("RUN_PAYMENT_RECONCILE_SCHEDULER", false),
		RunDunningWorker:                   getBoolEnv("RUN_DUNNING_WORKER", false),
		RunDunningScheduler:                getBoolEnv("RUN_DUNNING_SCHEDULER", false),
		RunBillingCycleWorker:              getBoolEnv("RUN_BILLING_CYCLE_WORKER", false),
		RunBillingCycleScheduler:           getBoolEnv("RUN_BILLING_CYCLE_SCHEDULER", false),
	}
	if !roles.AnyEnabled() {
		return ServerConfig{}, fmt.Errorf("at least one role must be enabled")
	}
	uiSessionCookieSecure := getBoolEnv("UI_SESSION_COOKIE_SECURE", productionLike)
	uiSessionCookieSameSite, uiSessionCookieSameSiteName := parseSameSiteMode(strings.TrimSpace(os.Getenv("UI_SESSION_COOKIE_SAMESITE")))
	uiSessionRequireOrigin := getBoolEnv("UI_SESSION_REQUIRE_ORIGIN", productionLike)
	allowedSessionOrigins, err := api.ParseAllowedOrigins(strings.TrimSpace(os.Getenv("UI_SESSION_ALLOWED_ORIGINS")))
	if err != nil {
		return ServerConfig{}, fmt.Errorf("failed to parse UI_SESSION_ALLOWED_ORIGINS: %w", err)
	}
	if err := validateAuthRuntimeConfig(authRuntimeConfig{
		Environment:               runtimeEnv,
		UISessionCookieSecure:     uiSessionCookieSecure,
		UISessionCookieSameSite:   uiSessionCookieSameSite,
		UISessionCookieSameSiteID: uiSessionCookieSameSiteName,
	}); err != nil {
		return ServerConfig{}, fmt.Errorf("invalid auth runtime config: %w", err)
	}

	rateLimitEnabled := getBoolEnv("RATE_LIMIT_ENABLED", productionLike)
	rateLimit := RateLimitConfig{
		Enabled:       rateLimitEnabled,
		FailOpen:      getBoolEnv("RATE_LIMIT_FAIL_OPEN", true),
		LoginFailOpen: getBoolEnv("RATE_LIMIT_LOGIN_FAIL_OPEN", false),
		RedisURL:      strings.TrimSpace(os.Getenv("RATE_LIMIT_REDIS_URL")),
		KeyPrefix:     strings.TrimSpace(os.Getenv("RATE_LIMIT_KEY_PREFIX")),
		PolicyRates:   api.DefaultRateLimitPolicyRates(),
	}
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyPreAuthLogin, "RATE_LIMIT_PREAUTH_LOGIN")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyPreAuthProtected, "RATE_LIMIT_PREAUTH_PROTECTED")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyWebhook, "RATE_LIMIT_WEBHOOK")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyAuthRead, "RATE_LIMIT_AUTH_READ")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyAuthWrite, "RATE_LIMIT_AUTH_WRITE")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyAuthAdmin, "RATE_LIMIT_AUTH_ADMIN")
	overrideRateLimitPolicy(rateLimit.PolicyRates, api.RateLimitPolicyAuthInternal, "RATE_LIMIT_AUTH_INTERNAL")
	if rateLimit.Enabled && rateLimit.RedisURL == "" {
		return ServerConfig{}, fmt.Errorf("RATE_LIMIT_REDIS_URL is required when RATE_LIMIT_ENABLED=true")
	}

	oidcProviders, err := parseOIDCProviderConfigs(strings.TrimSpace(os.Getenv("UI_OIDC_PROVIDERS_JSON")))
	if err != nil {
		return ServerConfig{}, fmt.Errorf("failed to parse UI_OIDC_PROVIDERS_JSON: %w", err)
	}
	ssoPublicBaseURL := strings.TrimSpace(os.Getenv("UI_PUBLIC_BASE_URL"))
	if len(oidcProviders) > 0 && ssoPublicBaseURL == "" {
		return ServerConfig{}, fmt.Errorf("UI_PUBLIC_BASE_URL is required when UI_OIDC_PROVIDERS_JSON is configured")
	}

	billingProviderSecretStoreBackend := strings.ToLower(strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_BACKEND")))
	billingProviderSecretStoreAccessKeyID := strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_ACCESS_KEY_ID"))
	if billingProviderSecretStoreAccessKeyID == "" {
		billingProviderSecretStoreAccessKeyID = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	}
	billingProviderSecretStoreSecretAccessKey := strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_SECRET_ACCESS_KEY"))
	if billingProviderSecretStoreSecretAccessKey == "" {
		billingProviderSecretStoreSecretAccessKey = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	}
	billingProviderSecretStoreSessionToken := strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_SESSION_TOKEN"))
	if billingProviderSecretStoreSessionToken == "" {
		billingProviderSecretStoreSessionToken = strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
	}
	billingProviderStripeSuccessRedirectURL := strings.TrimSpace(os.Getenv("BILLING_PROVIDER_STRIPE_SUCCESS_REDIRECT_URL"))
	billingProviderDefaultOrganizationID := strings.TrimSpace(os.Getenv("BILLING_PROVIDER_DEFAULT_ORGANIZATION_ID"))
	if billingProviderSecretStoreBackend != "" {
		switch billingProviderSecretStoreBackend {
		case "memory", "aws-secretsmanager":
		default:
			return ServerConfig{}, fmt.Errorf("BILLING_PROVIDER_SECRET_STORE_BACKEND must be memory or aws-secretsmanager")
		}
		if billingProviderStripeSuccessRedirectURL == "" {
			return ServerConfig{}, fmt.Errorf("BILLING_PROVIDER_STRIPE_SUCCESS_REDIRECT_URL is required when billing provider connections are enabled")
		}
	}
	accessKeyID := strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_ACCESS_KEY_ID"))
	if accessKeyID == "" {
		accessKeyID = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	}
	secretAccessKey := strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_SECRET_ACCESS_KEY"))
	if secretAccessKey == "" {
		secretAccessKey = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	}
	sessionToken := strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_SESSION_TOKEN"))
	if sessionToken == "" {
		sessionToken = strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
	}
	inviteSMTPHost := strings.TrimSpace(os.Getenv("INVITATION_EMAIL_SMTP_HOST"))
	inviteSMTPPort := getIntEnv("INVITATION_EMAIL_SMTP_PORT", 587)
	inviteSMTPUsername := strings.TrimSpace(os.Getenv("INVITATION_EMAIL_SMTP_USERNAME"))
	inviteSMTPPassword := strings.TrimSpace(os.Getenv("INVITATION_EMAIL_SMTP_PASSWORD"))
	inviteFromEmail := strings.TrimSpace(os.Getenv("INVITATION_EMAIL_FROM_EMAIL"))
	inviteFromName := strings.TrimSpace(os.Getenv("INVITATION_EMAIL_FROM_NAME"))
	passwordResetTTL := getDurationEnv("PASSWORD_RESET_TOKEN_TTL", time.Hour)
	if inviteSMTPHost != "" {
		if inviteFromEmail == "" {
			return ServerConfig{}, fmt.Errorf("INVITATION_EMAIL_FROM_EMAIL is required when invitation email delivery is configured")
		}
	}

	dbCfg, err := LoadDBConfigFromEnv()
	if err != nil {
		return ServerConfig{}, err
	}

	cfg := ServerConfig{
		RuntimeEnv:     runtimeEnv,
		ProductionLike: productionLike,
		Port:           firstNonEmpty(strings.TrimSpace(os.Getenv("PORT")), "8080"),
		DB:             dbCfg,
		Roles:          roles,
		Temporal: TemporalConfig{
			Address:               firstNonEmpty(strings.TrimSpace(os.Getenv("TEMPORAL_ADDRESS")), "localhost:7233"),
			Namespace:             firstNonEmpty(strings.TrimSpace(os.Getenv("TEMPORAL_NAMESPACE")), "default"),
			ReplayTaskQueue:       firstNonEmpty(strings.TrimSpace(os.Getenv("REPLAY_TEMPORAL_TASK_QUEUE")), replay.DefaultTemporalReplayTaskQueue),
			ReplayDispatcherPoll:  time.Duration(getIntEnv("REPLAY_TEMPORAL_DISPATCH_POLL_MS", 750)) * time.Millisecond,
			ReplayDispatcherBatch: getIntEnv("REPLAY_TEMPORAL_DISPATCH_BATCH", 25),
		},
		UISession: UISessionConfig{
			Lifetime:           time.Duration(getIntEnv("UI_SESSION_LIFETIME_SEC", 43200)) * time.Second,
			CookieName:         firstNonEmpty(strings.TrimSpace(os.Getenv("UI_SESSION_COOKIE_NAME")), "alpha_ui_session"),
			CookieSecure:       uiSessionCookieSecure,
			CookieSameSite:     uiSessionCookieSameSite,
			CookieSameSiteName: uiSessionCookieSameSiteName,
			RequireOrigin:      uiSessionRequireOrigin,
			AllowedOrigins:     allowedSessionOrigins,
		},
		SSO: SSOConfig{
			PublicBaseURL:      strings.TrimRight(ssoPublicBaseURL, "/"),
			AutoProvisionUsers: getBoolEnv("UI_SSO_AUTO_PROVISION_USERS", false),
			OIDCProviders:      oidcProviders,
		},
		RateLimit: rateLimit,
		BillingProviders: BillingProviderConfig{
			SecretStoreBackend:         billingProviderSecretStoreBackend,
			SecretStoreAWSRegion:       firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))),
			SecretStoreAWSEndpoint:     strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_AWS_ENDPOINT")),
			SecretStorePrefix:          strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_PREFIX")),
			SecretStoreAccessKeyID:     billingProviderSecretStoreAccessKeyID,
			SecretStoreSecretAccessKey: billingProviderSecretStoreSecretAccessKey,
			SecretStoreSessionToken:    billingProviderSecretStoreSessionToken,
			DefaultOrganizationID:  billingProviderDefaultOrganizationID,
			StripeSuccessRedirectURL:   billingProviderStripeSuccessRedirectURL,
			StripeWebhookSecret:       strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		},
		BillingChecks: BillingConnectionCheckConfig{
			TaskQueue:         firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CONNECTION_CHECK_TEMPORAL_TASK_QUEUE")), billingcheck.DefaultTemporalBillingConnectionCheckTaskQueue),
			CronSchedule:      firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CONNECTION_CHECK_CRON_SCHEDULE")), billingcheck.DefaultBillingConnectionCheckCronSchedule),
			WorkflowID:        firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CONNECTION_CHECK_WORKFLOW_ID")), billingcheck.DefaultBillingConnectionCheckWorkflowID),
			Batch:             getIntEnv("BILLING_CONNECTION_CHECK_BATCH", 50),
			StaleAfterSeconds: getIntEnv("BILLING_CONNECTION_CHECK_STALE_AFTER_SEC", 21600),
		},
		Payment: PaymentReconcileConfig{
			TaskQueue:         firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_TEMPORAL_TASK_QUEUE")), paymentsync.DefaultTemporalPaymentReconcileTaskQueue),
			CronSchedule:      firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_CRON_SCHEDULE")), paymentsync.DefaultPaymentReconcileCronSchedule),
			WorkflowID:        firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_WORKFLOW_ID")), paymentsync.DefaultPaymentReconcileWorkflowID),
			Batch:             getIntEnv("PAYMENT_RECONCILE_BATCH", 100),
			StaleAfterSeconds: getIntEnv("PAYMENT_RECONCILE_STALE_AFTER_SEC", 300),
		},
		Dunning: DunningConfig{
			TaskQueue:    firstNonEmpty(strings.TrimSpace(os.Getenv("DUNNING_TEMPORAL_TASK_QUEUE")), dunningflow.DefaultTemporalDunningTaskQueue),
			CronSchedule: firstNonEmpty(strings.TrimSpace(os.Getenv("DUNNING_CRON_SCHEDULE")), dunningflow.DefaultDunningCronSchedule),
			WorkflowID:   firstNonEmpty(strings.TrimSpace(os.Getenv("DUNNING_WORKFLOW_ID")), dunningflow.DefaultDunningWorkflowID),
			Batch:        getIntEnv("DUNNING_BATCH", 20),
			TenantID:     firstNonEmpty(strings.TrimSpace(os.Getenv("DUNNING_TENANT_ID")), "default"),
		},
		BillingCycle: BillingCycleConfig{
			TaskQueue:    firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CYCLE_TEMPORAL_TASK_QUEUE")), billingcycle.DefaultBillingCycleTaskQueue),
			CronSchedule: firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CYCLE_CRON_SCHEDULE")), billingcycle.DefaultBillingCycleCronSchedule),
			WorkflowID:   firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CYCLE_WORKFLOW_ID")), billingcycle.DefaultBillingCycleWorkflowID),
			Batch:        getIntEnv("BILLING_CYCLE_BATCH", 50),
			TenantID:     firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_CYCLE_TENANT_ID")), "default"),
		},
		APIKeysRaw: strings.TrimSpace(os.Getenv("API_KEYS")),
		AuditExport: AuditExportConfig{
			Enabled: getBoolEnv("AUDIT_EXPORTS_ENABLED", false),
			S3: service.S3Config{
				Region:          strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_REGION")),
				Bucket:          strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_BUCKET")),
				Endpoint:        strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_ENDPOINT")),
				AccessKeyID:     accessKeyID,
				SecretAccessKey: secretAccessKey,
				SessionToken:    sessionToken,
				ForcePathStyle:  getBoolEnv("AUDIT_EXPORT_S3_FORCE_PATH_STYLE", true),
			},
			DownloadTTL: time.Duration(getIntEnv("AUDIT_EXPORT_DOWNLOAD_TTL_SEC", 86400)) * time.Second,
			WorkerPoll:  time.Duration(getIntEnv("AUDIT_EXPORT_WORKER_POLL_MS", 500)) * time.Millisecond,
		},
		Email: InvitationEmailConfig{
			SMTPHost:      inviteSMTPHost,
			SMTPPort:      inviteSMTPPort,
			SMTPUsername:  inviteSMTPUsername,
			SMTPPassword:  inviteSMTPPassword,
			FromEmail:     inviteFromEmail,
			FromName:      inviteFromName,
			ResetTokenTTL: passwordResetTTL,
		},
	}

	return cfg, nil
}

func parseOIDCProviderConfigs(raw string) ([]OIDCProviderConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var providers []OIDCProviderConfig
	if err := json.Unmarshal([]byte(raw), &providers); err != nil {
		return nil, err
	}
	for i := range providers {
		providers[i].Key = strings.ToLower(strings.TrimSpace(providers[i].Key))
		providers[i].DisplayName = strings.TrimSpace(providers[i].DisplayName)
		providers[i].IssuerURL = strings.TrimSpace(providers[i].IssuerURL)
		providers[i].ClientID = strings.TrimSpace(providers[i].ClientID)
		providers[i].ClientSecret = strings.TrimSpace(providers[i].ClientSecret)
		if providers[i].Key == "" || providers[i].DisplayName == "" || providers[i].IssuerURL == "" || providers[i].ClientID == "" || providers[i].ClientSecret == "" {
			return nil, fmt.Errorf("oidc provider key, display_name, issuer_url, client_id, and client_secret are required")
		}
	}
	return providers, nil
}

type authRuntimeConfig struct {
	Environment               string
	UISessionCookieSecure     bool
	UISessionCookieSameSite   http.SameSite
	UISessionCookieSameSiteID string
}

func validateAuthRuntimeConfig(cfg authRuntimeConfig) error {
	if isProductionLikeEnvironment(cfg.Environment) && !cfg.UISessionCookieSecure {
		return fmt.Errorf("UI_SESSION_COOKIE_SECURE must be true in %s", cfg.Environment)
	}
	if cfg.UISessionCookieSameSite == http.SameSiteNoneMode && !cfg.UISessionCookieSecure {
		return fmt.Errorf("UI_SESSION_COOKIE_SAMESITE=%s requires UI_SESSION_COOKIE_SECURE=true", cfg.UISessionCookieSameSiteID)
	}
	return nil
}

func resolveRuntimeEnvironment() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("APP_ENV")),
		strings.TrimSpace(os.Getenv("ENVIRONMENT")),
	}
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" {
			return candidate
		}
	}
	return "local"
}

func isProductionLikeEnvironment(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

func parseSameSiteMode(raw string) (http.SameSite, string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode, "strict"
	case "none":
		return http.SameSiteNoneMode, "none"
	case "lax", "":
		return http.SameSiteLaxMode, "lax"
	default:
		return http.SameSiteLaxMode, "lax"
	}
}

func overrideRateLimitPolicy(policyRates map[string]string, policy, envKey string) {
	if policyRates == nil {
		return
	}
	value := strings.TrimSpace(os.Getenv(envKey))
	if value == "" {
		return
	}
	policyRates[policy] = value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func getIntEnv(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func getBoolEnv(key string, defaultVal bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}

	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}
