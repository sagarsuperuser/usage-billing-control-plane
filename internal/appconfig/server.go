package appconfig

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"usage-billing-control-plane/internal/api"
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
	RateLimit        RateLimitConfig
	Lago             LagoConfig
	BillingProviders BillingProviderConfig
	Payment          PaymentReconcileConfig
	APIKeysRaw       string
	AuditExport      AuditExportConfig
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
	RunAPIServer                 bool
	RunReplayWorker              bool
	RunReplayDispatcher          bool
	RunPaymentReconcileWorker    bool
	RunPaymentReconcileScheduler bool
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

type RateLimitConfig struct {
	Enabled       bool
	FailOpen      bool
	LoginFailOpen bool
	RedisURL      string
	KeyPrefix     string
	PolicyRates   map[string]string
}

type LagoConfig struct {
	APIURL              string
	APIKey              string
	HTTPTimeout         time.Duration
	WebhookPublicKeyTTL time.Duration
}

type BillingProviderConfig struct {
	SecretStoreBackend         string
	SecretStoreAWSRegion       string
	SecretStoreAWSEndpoint     string
	SecretStorePrefix          string
	SecretStoreAccessKeyID     string
	SecretStoreSecretAccessKey string
	SecretStoreSessionToken    string
	StripeSuccessRedirectURL   string
}

type PaymentReconcileConfig struct {
	TaskQueue         string
	CronSchedule      string
	WorkflowID        string
	Batch             int
	StaleAfterSeconds int
}

type AuditExportConfig struct {
	Enabled     bool
	S3          service.S3Config
	DownloadTTL time.Duration
	WorkerPoll  time.Duration
}

func LoadServerConfigFromEnv() (ServerConfig, error) {
	runtimeEnv := resolveRuntimeEnvironment()
	productionLike := isProductionLikeEnvironment(runtimeEnv)

	roles := RoleConfig{
		RunAPIServer:                 getBoolEnv("RUN_API_SERVER", true),
		RunReplayWorker:              getBoolEnv("RUN_REPLAY_WORKER", true),
		RunReplayDispatcher:          getBoolEnv("RUN_REPLAY_DISPATCHER", true),
		RunPaymentReconcileWorker:    getBoolEnv("RUN_PAYMENT_RECONCILE_WORKER", false),
		RunPaymentReconcileScheduler: getBoolEnv("RUN_PAYMENT_RECONCILE_SCHEDULER", false),
	}
	if !roles.RunAPIServer && !roles.RunReplayWorker && !roles.RunReplayDispatcher && !roles.RunPaymentReconcileWorker && !roles.RunPaymentReconcileScheduler {
		return ServerConfig{}, fmt.Errorf("at least one role must be enabled: RUN_API_SERVER, RUN_REPLAY_WORKER, RUN_REPLAY_DISPATCHER, RUN_PAYMENT_RECONCILE_WORKER, RUN_PAYMENT_RECONCILE_SCHEDULER")
	}
	if !roles.RunAPIServer && (roles.RunPaymentReconcileWorker || roles.RunPaymentReconcileScheduler) {
		return ServerConfig{}, fmt.Errorf("payment reconcile roles require RUN_API_SERVER=true")
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

	lagoAPIURL := strings.TrimSpace(os.Getenv("LAGO_API_URL"))
	if lagoAPIURL == "" {
		return ServerConfig{}, fmt.Errorf("LAGO_API_URL is required")
	}
	lagoAPIKey := strings.TrimSpace(os.Getenv("LAGO_API_KEY"))
	if lagoAPIKey == "" {
		return ServerConfig{}, fmt.Errorf("LAGO_API_KEY is required")
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
			CookieName:         firstNonEmpty(strings.TrimSpace(os.Getenv("UI_SESSION_COOKIE_NAME")), "lago_alpha_ui_session"),
			CookieSecure:       uiSessionCookieSecure,
			CookieSameSite:     uiSessionCookieSameSite,
			CookieSameSiteName: uiSessionCookieSameSiteName,
			RequireOrigin:      uiSessionRequireOrigin,
			AllowedOrigins:     allowedSessionOrigins,
		},
		RateLimit: rateLimit,
		Lago: LagoConfig{
			APIURL:              lagoAPIURL,
			APIKey:              lagoAPIKey,
			HTTPTimeout:         time.Duration(getIntEnv("LAGO_HTTP_TIMEOUT_MS", 10000)) * time.Millisecond,
			WebhookPublicKeyTTL: time.Duration(getIntEnv("LAGO_WEBHOOK_PUBLIC_KEY_TTL_SEC", 300)) * time.Second,
		},
		BillingProviders: BillingProviderConfig{
			SecretStoreBackend:         billingProviderSecretStoreBackend,
			SecretStoreAWSRegion:       firstNonEmpty(strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_REGION")), strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))),
			SecretStoreAWSEndpoint:     strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_AWS_ENDPOINT")),
			SecretStorePrefix:          strings.TrimSpace(os.Getenv("BILLING_PROVIDER_SECRET_STORE_PREFIX")),
			SecretStoreAccessKeyID:     billingProviderSecretStoreAccessKeyID,
			SecretStoreSecretAccessKey: billingProviderSecretStoreSecretAccessKey,
			SecretStoreSessionToken:    billingProviderSecretStoreSessionToken,
			StripeSuccessRedirectURL:   billingProviderStripeSuccessRedirectURL,
		},
		Payment: PaymentReconcileConfig{
			TaskQueue:         firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_TEMPORAL_TASK_QUEUE")), paymentsync.DefaultTemporalPaymentReconcileTaskQueue),
			CronSchedule:      firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_CRON_SCHEDULE")), paymentsync.DefaultPaymentReconcileCronSchedule),
			WorkflowID:        firstNonEmpty(strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_WORKFLOW_ID")), paymentsync.DefaultPaymentReconcileWorkflowID),
			Batch:             getIntEnv("PAYMENT_RECONCILE_BATCH", 100),
			StaleAfterSeconds: getIntEnv("PAYMENT_RECONCILE_STALE_AFTER_SEC", 300),
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
	}

	return cfg, nil
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
