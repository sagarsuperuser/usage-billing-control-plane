package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
	temporalclient "go.temporal.io/sdk/client"
	temporalsdkworker "go.temporal.io/sdk/worker"

	"usage-billing-control-plane/internal/api"
	"usage-billing-control-plane/internal/appconfig"
	"usage-billing-control-plane/internal/dunningflow"
	"usage-billing-control-plane/internal/logging"
	"usage-billing-control-plane/internal/paymentsync"
	"usage-billing-control-plane/internal/replay"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())

	cfg, err := appconfig.LoadServerConfigFromEnv()
	if err != nil {
		fatal(logger, "load server config", "error", err)
	}

	logger.Info("runtime env detected", "component", "server", "environment", cfg.RuntimeEnv, "production_like", cfg.ProductionLike)

	db, err := openDB(cfg.DB)
	if err != nil {
		fatal(logger, "open database", "error", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(
		db,
		store.WithQueryTimeout(cfg.DB.QueryTimeout),
		store.WithMigrationTimeout(cfg.DB.MigrationTimeout),
	)
	if cfg.DB.RunMigrationsOnBoot {
		if err := repo.Migrate(); err != nil {
			fatal(logger, "run boot migrations", "error", err)
		}
		logger.Info("boot migrations applied", "component", "server")
	} else {
		logger.Info("boot migrations skipped", "component", "server")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info(
		"role config",
		"component", "server",
		"api", cfg.Roles.RunAPIServer,
		"replay_worker", cfg.Roles.RunReplayWorker,
		"replay_dispatcher", cfg.Roles.RunReplayDispatcher,
		"payment_reconcile_worker", cfg.Roles.RunPaymentReconcileWorker,
		"payment_reconcile_scheduler", cfg.Roles.RunPaymentReconcileScheduler,
		"dunning_worker", cfg.Roles.RunDunningWorker,
		"dunning_scheduler", cfg.Roles.RunDunningScheduler,
	)

	var (
		temporalClient           temporalclient.Client
		temporalReplayWorker     temporalsdkworker.Worker
		temporalPaymentWorker    temporalsdkworker.Worker
		temporalDunningWorker    temporalsdkworker.Worker
		replayTemporalDispatcher *replay.TemporalDispatcher
		rateLimiterCloser        interface{ Close() error }
	)
	if cfg.Roles.RunReplayWorker || cfg.Roles.RunReplayDispatcher || cfg.Roles.RunPaymentReconcileWorker || cfg.Roles.RunPaymentReconcileScheduler || cfg.Roles.RunDunningWorker || cfg.Roles.RunDunningScheduler {
		temporalClient, err = temporalclient.Dial(temporalclient.Options{
			HostPort:  cfg.Temporal.Address,
			Namespace: cfg.Temporal.Namespace,
		})
		if err != nil {
			fatal(logger, "initialize temporal client", "error", err)
		}

		if cfg.Roles.RunReplayWorker {
			temporalReplayWorker = temporalsdkworker.New(temporalClient, cfg.Temporal.ReplayTaskQueue, temporalsdkworker.Options{})
			replay.RegisterTemporalReplayWorker(temporalReplayWorker, repo)
			if err := temporalReplayWorker.Start(); err != nil {
				temporalClient.Close()
				fatal(logger, "start temporal replay worker", "error", err)
			}
			logger.Info(
				"replay worker started",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"replay_task_queue", cfg.Temporal.ReplayTaskQueue,
			)
		}

		if cfg.Roles.RunReplayDispatcher {
			replayTemporalDispatcher = replay.NewTemporalDispatcher(
				repo,
				temporalClient,
				cfg.Temporal.ReplayTaskQueue,
				cfg.Temporal.ReplayDispatcherPoll,
				cfg.Temporal.ReplayDispatcherBatch,
				logger,
			)
			go replayTemporalDispatcher.Run(ctx)
			logger.Info(
				"replay dispatcher started",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"replay_task_queue", cfg.Temporal.ReplayTaskQueue,
				"poll_ms", cfg.Temporal.ReplayDispatcherPoll.Milliseconds(),
				"batch", cfg.Temporal.ReplayDispatcherBatch,
			)
		}
	}

	closeReplayRuntime := func() {
		if temporalReplayWorker != nil {
			temporalReplayWorker.Stop()
		}
		if temporalPaymentWorker != nil {
			temporalPaymentWorker.Stop()
		}
		if temporalDunningWorker != nil {
			temporalDunningWorker.Stop()
		}
		if temporalClient != nil {
			temporalClient.Close()
		}
		if rateLimiterCloser != nil {
			if err := rateLimiterCloser.Close(); err != nil {
				logger.Warn("rate limiter close failed", "component", "server", "error", err)
			}
		}
	}

	if !cfg.Roles.RunAPIServer {
		logger.Info("roles only mode waiting for shutdown", "component", "server")
		<-ctx.Done()
		closeReplayRuntime()
		logger.Info("shutdown complete", "component", "server")
		return
	}

	if cfg.UISession.RequireOrigin && cfg.ProductionLike && len(cfg.UISession.AllowedOrigins) == 0 {
		logger.Warn("session origin allowlist empty; same-origin only", "component", "server")
	}

	var rateLimiter api.RateLimiter
	if cfg.RateLimit.Enabled {
		redisRateLimiter, err := api.NewRedisRateLimiter(api.RedisRateLimiterConfig{
			RedisURL:    cfg.RateLimit.RedisURL,
			KeyPrefix:   cfg.RateLimit.KeyPrefix,
			PolicyRates: cfg.RateLimit.PolicyRates,
		})
		if err != nil {
			fatal(logger, "initialize rate limiter", "error", err)
		}
		rateLimiter = redisRateLimiter
		rateLimiterCloser = redisRateLimiter
		logger.Info(
			"rate limiter enabled",
			"component", "server",
			"backend", "redis",
			"fail_open", cfg.RateLimit.FailOpen,
			"login_fail_open", cfg.RateLimit.LoginFailOpen,
		)
	} else if cfg.ProductionLike {
		logger.Warn("rate limiter disabled", "component", "server", "environment", cfg.RuntimeEnv)
	}

	uiSessionManager := scs.New()
	uiSessionManager.Store = postgresstore.New(db)
	uiSessionManager.Lifetime = cfg.UISession.Lifetime
	uiSessionManager.Cookie.Name = cfg.UISession.CookieName
	uiSessionManager.Cookie.HttpOnly = true
	uiSessionManager.Cookie.Secure = cfg.UISession.CookieSecure
	uiSessionManager.Cookie.Path = "/"
	uiSessionManager.Cookie.SameSite = cfg.UISession.CookieSameSite

	serverOpts := []api.ServerOption{
		api.WithMetricsProvider(buildMetricsProvider(replayTemporalDispatcher, db)),
		api.WithSessionManager(uiSessionManager),
		api.WithSessionOriginPolicy(cfg.UISession.RequireOrigin, cfg.UISession.AllowedOrigins),
		api.WithLogger(logger),
		api.WithReadinessCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		}),
	}
	if rateLimiter != nil {
		serverOpts = append(serverOpts, api.WithRateLimiter(rateLimiter, cfg.RateLimit.FailOpen, cfg.RateLimit.LoginFailOpen))
	}

	lagoTransport, err := service.NewLagoHTTPTransport(service.LagoClientConfig{
		BaseURL: cfg.Lago.APIURL,
		APIKey:  cfg.Lago.APIKey,
		Timeout: cfg.Lago.HTTPTimeout,
	})
	if err != nil {
		fatal(logger, "initialize lago client", "error", err)
	}
	invoiceBillingAdapter := service.NewLagoInvoiceAdapter(lagoTransport)
	customerBillingAdapter := service.NewLagoCustomerBillingAdapter(lagoTransport)
	serverOpts = append(
		serverOpts,
		api.WithMeterSyncAdapter(service.NewLagoMeterSyncAdapter(lagoTransport)),
		api.WithTaxSyncAdapter(service.NewLagoTaxSyncAdapter(lagoTransport)),
		api.WithPlanSyncAdapter(service.NewLagoPlanSyncAdapter(lagoTransport, repo)),
		api.WithSubscriptionSyncAdapter(service.NewLagoSubscriptionSyncAdapter(lagoTransport, repo)),
		api.WithUsageSyncAdapter(service.NewLagoUsageSyncAdapter(lagoTransport)),
		api.WithInvoiceBillingAdapter(invoiceBillingAdapter),
		api.WithCustomerBillingAdapter(customerBillingAdapter),
	)
	logger.Info("lago adapter enabled", "component", "server", "base_url", cfg.Lago.APIURL)

	var billingSecretStore service.BillingSecretStore
	if strings.TrimSpace(cfg.BillingProviders.SecretStoreBackend) != "" {
		billingSecretStore, err = newBillingSecretStore(context.Background(), cfg.BillingProviders)
		if err != nil {
			fatal(logger, "initialize billing provider secret store", "error", err)
		}
		billingProviderSvc := service.NewBillingProviderConnectionService(
			repo,
			billingSecretStore,
			service.NewLagoBillingProviderAdapter(lagoTransport, cfg.BillingProviders.StripeSuccessRedirectURL),
		).WithDefaultLagoOrganizationID(cfg.BillingProviders.DefaultLagoOrganizationID)
		serverOpts = append(serverOpts, api.WithBillingProviderConnectionService(billingProviderSvc))
		logger.Info(
			"billing provider connections enabled",
			"component", "server",
			"secret_store_backend", cfg.BillingProviders.SecretStoreBackend,
			"default_lago_organization_id", cfg.BillingProviders.DefaultLagoOrganizationID,
			"stripe_success_redirect_url", cfg.BillingProviders.StripeSuccessRedirectURL,
		)
	} else {
		logger.Info("billing provider connections disabled", "component", "server")
	}

	if cfg.Roles.RunPaymentReconcileWorker || cfg.Roles.RunPaymentReconcileScheduler {
		if temporalClient == nil {
			fatal(logger, "temporal client is required when payment reconcile roles are enabled")
		}

		if cfg.Roles.RunPaymentReconcileWorker {
			temporalPaymentWorker = temporalsdkworker.New(temporalClient, cfg.Payment.TaskQueue, temporalsdkworker.Options{})
			if err := paymentsync.RegisterTemporalPaymentReconcileWorker(temporalPaymentWorker, repo, service.NewLagoInvoiceAdapter(lagoTransport)); err != nil {
				fatal(logger, "register payment reconcile worker", "error", err)
			}
			if err := temporalPaymentWorker.Start(); err != nil {
				closeReplayRuntime()
				fatal(logger, "start payment reconcile worker", "error", err)
			}
			logger.Info(
				"payment reconcile worker started",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"task_queue", cfg.Payment.TaskQueue,
			)
		}

		if cfg.Roles.RunPaymentReconcileScheduler {
			input := paymentsync.PaymentReconcileWorkflowInput{
				Limit:             cfg.Payment.Batch,
				StaleAfterSeconds: cfg.Payment.StaleAfterSeconds,
			}

			startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := paymentsync.EnsurePaymentReconcileCronWorkflow(startCtx, temporalClient, cfg.Payment.TaskQueue, cfg.Payment.WorkflowID, cfg.Payment.CronSchedule, input)
			cancel()
			if err != nil {
				closeReplayRuntime()
				fatal(logger, "ensure payment reconcile cron workflow", "error", err)
			}
			logger.Info(
				"payment reconcile scheduler enabled",
				"component", "server",
				"task_queue", cfg.Payment.TaskQueue,
				"workflow_id", cfg.Payment.WorkflowID,
				"cron", cfg.Payment.CronSchedule,
				"batch", input.Limit,
				"stale_after_sec", input.StaleAfterSeconds,
			)
		}
	}

	var webhookKeyProvider service.LagoWebhookHMACKeyProvider
	var tenantWebhookKeyProvider service.LagoWebhookHMACKeyProvider
	var staticWebhookKeyProvider service.LagoWebhookHMACKeyProvider
	if billingSecretStore != nil {
		tenantWebhookKeyProvider = service.NewTenantBackedLagoWebhookHMACKeyProvider(repo, billingSecretStore)
	}
	if strings.TrimSpace(cfg.Lago.WebhookHMACKey) != "" {
		staticWebhookKeyProvider, err = service.NewStaticLagoWebhookHMACKeyProvider(cfg.Lago.WebhookHMACKey)
		if err != nil {
			fatal(logger, "initialize static lago webhook hmac key", "error", err)
		}
	}
	switch {
	case tenantWebhookKeyProvider != nil && staticWebhookKeyProvider != nil:
		webhookKeyProvider, err = service.NewFallbackLagoWebhookHMACKeyProvider(tenantWebhookKeyProvider, staticWebhookKeyProvider)
		if err != nil {
			fatal(logger, "initialize fallback lago webhook hmac key provider", "error", err)
		}
		logger.Info("using tenant-backed lago webhook hmac verification with static fallback", "component", "server")
	case tenantWebhookKeyProvider != nil:
		webhookKeyProvider = tenantWebhookKeyProvider
		logger.Info("using tenant-backed lago webhook hmac verification", "component", "server")
	case staticWebhookKeyProvider != nil:
		webhookKeyProvider = staticWebhookKeyProvider
		logger.Info("using static lago webhook hmac verification", "component", "server")
	default:
		fatal(logger, "initialize lago webhook verifier", "error", fmt.Errorf("lago webhook hmac verification requires billing provider secret store or LAGO_WEBHOOK_HMAC_KEY"))
	}
	webhookVerifier, err := service.NewLagoHMACWebhookVerifier(webhookKeyProvider)
	if err != nil {
		fatal(logger, "initialize lago webhook verifier", "error", err)
	}
	workspaceBillingBindingService := service.NewWorkspaceBillingBindingService(repo)
	webhookDunningSvc, err := service.NewDunningService(repo)
	if err != nil {
		fatal(logger, "initialize webhook dunning service", "error", err)
	}
	lagoWebhookSvc := service.NewLagoWebhookService(
		repo,
		webhookVerifier,
		service.NewTenantBackedLagoOrganizationTenantMapper(repo),
		service.NewCustomerService(repo, customerBillingAdapter).WithWorkspaceBillingBindingService(workspaceBillingBindingService),
	).WithDunningService(webhookDunningSvc)
	serverOpts = append(serverOpts, api.WithLagoWebhookService(lagoWebhookSvc))
	logger.Info("lago webhook sync enabled", "component", "server", "mapper_backend", "tenant_store")

	authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
	if err != nil {
		fatal(logger, "initialize api key authorizer", "error", err)
	}
	serverOpts = append(serverOpts, api.WithAPIKeyAuthorizer(authorizer))
	logger.Info("api auth enabled", "component", "server", "backend", "postgres")

	browserUserAuthService, err := service.NewBrowserUserAuthService(repo)
	if err != nil {
		fatal(logger, "initialize browser user auth", "error", err)
	}
	serverOpts = append(serverOpts, api.WithBrowserUserAuthService(browserUserAuthService))

	var (
		invitationEmailSender    service.WorkspaceInvitationEmailSender
		passwordResetEmailSender service.PasswordResetEmailSender
		paymentSetupEmailSender  service.CustomerPaymentSetupRequestEmailSender
	)
	if strings.TrimSpace(cfg.Email.SMTPHost) != "" {
		invitationEmailSender, err = service.NewSMTPWorkspaceInvitationEmailSender(service.SMTPWorkspaceInvitationEmailConfig{
			Host:      cfg.Email.SMTPHost,
			Port:      cfg.Email.SMTPPort,
			Username:  cfg.Email.SMTPUsername,
			Password:  cfg.Email.SMTPPassword,
			FromEmail: cfg.Email.FromEmail,
			FromName:  cfg.Email.FromName,
		})
		if err != nil {
			fatal(logger, "initialize workspace invitation email sender", "error", err)
		}

		passwordResetEmailSender, err = service.NewSMTPPasswordResetEmailSender(service.SMTPPasswordResetEmailConfig{
			Host:      cfg.Email.SMTPHost,
			Port:      cfg.Email.SMTPPort,
			Username:  cfg.Email.SMTPUsername,
			Password:  cfg.Email.SMTPPassword,
			FromEmail: cfg.Email.FromEmail,
			FromName:  cfg.Email.FromName,
		})
		if err != nil {
			fatal(logger, "initialize password reset email sender", "error", err)
		}

		paymentSetupEmailSender, err = service.NewSMTPPaymentSetupRequestEmailSender(service.SMTPPaymentSetupRequestEmailConfig{
			Host:      cfg.Email.SMTPHost,
			Port:      cfg.Email.SMTPPort,
			Username:  cfg.Email.SMTPUsername,
			Password:  cfg.Email.SMTPPassword,
			FromEmail: cfg.Email.FromEmail,
			FromName:  cfg.Email.FromName,
		})
		if err != nil {
			fatal(logger, "initialize payment setup request email sender", "error", err)
		}
		serverOpts = append(serverOpts, api.WithPasswordResetService(service.NewPasswordResetService(repo, cfg.Email.ResetTokenTTL)))
		logger.Info(
			"alpha notification service enabled",
			"component", "server",
			"smtp_host", cfg.Email.SMTPHost,
			"from_email", cfg.Email.FromEmail,
		)
		logger.Info(
			"password reset email enabled",
			"component", "server",
			"smtp_host", cfg.Email.SMTPHost,
			"from_email", cfg.Email.FromEmail,
			"reset_ttl", cfg.Email.ResetTokenTTL.String(),
		)
		logger.Info(
			"payment setup request email enabled",
			"component", "server",
			"smtp_host", cfg.Email.SMTPHost,
			"from_email", cfg.Email.FromEmail,
		)
	} else {
		logger.Info("workspace invitation email disabled", "component", "server")
		logger.Info("password reset email disabled", "component", "server")
		logger.Info("payment setup request email disabled", "component", "server")
	}
	serverOpts = append(serverOpts, api.WithNotificationService(service.NewNotificationService(
		invitationEmailSender,
		passwordResetEmailSender,
		paymentSetupEmailSender,
		invoiceBillingAdapter,
	)))
	logger.Info(
		"billing notification delegation enabled",
		"component", "server",
		"backend", "lago",
	)

	if cfg.Roles.RunDunningWorker || cfg.Roles.RunDunningScheduler {
		if temporalClient == nil {
			fatal(logger, "temporal client is required when dunning roles are enabled")
		}
		notificationSvc := service.NewNotificationService(
			invitationEmailSender,
			passwordResetEmailSender,
			paymentSetupEmailSender,
			invoiceBillingAdapter,
		)
		if cfg.Roles.RunDunningWorker {
			temporalDunningWorker = temporalsdkworker.New(temporalClient, cfg.Dunning.TaskQueue, temporalsdkworker.Options{})
			if err := dunningflow.RegisterTemporalDunningWorker(temporalDunningWorker, repo, customerBillingAdapter, invoiceBillingAdapter, notificationSvc); err != nil {
				fatal(logger, "register dunning worker", "error", err)
			}
			if err := temporalDunningWorker.Start(); err != nil {
				closeReplayRuntime()
				fatal(logger, "start dunning worker", "error", err)
			}
			logger.Info(
				"dunning worker started",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"dunning_task_queue", cfg.Dunning.TaskQueue,
			)
		}
		if cfg.Roles.RunDunningScheduler {
			startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := dunningflow.EnsureCollectPaymentReminderCronWorkflow(startCtx, temporalClient, cfg.Dunning.TaskQueue, cfg.Dunning.WorkflowID, cfg.Dunning.CronSchedule, dunningflow.CollectPaymentReminderWorkflowInput{
				TenantID: cfg.Dunning.TenantID,
				Limit:    cfg.Dunning.Batch,
			})
			cancel()
			if err != nil {
				closeReplayRuntime()
				fatal(logger, "ensure dunning cron workflow", "error", err)
			}
			logger.Info(
				"dunning scheduler ensured",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"dunning_task_queue", cfg.Dunning.TaskQueue,
				"workflow_id", cfg.Dunning.WorkflowID,
				"cron_schedule", cfg.Dunning.CronSchedule,
				"batch", cfg.Dunning.Batch,
				"tenant_id", cfg.Dunning.TenantID,
			)

			retryWorkflowID := strings.TrimSpace(cfg.Dunning.WorkflowID)
			if retryWorkflowID == "" || retryWorkflowID == dunningflow.DefaultDunningWorkflowID {
				retryWorkflowID = dunningflow.DefaultDunningRetryWorkflowID
			} else {
				retryWorkflowID += "/retry"
			}
			startCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
			err = dunningflow.EnsureRetryPaymentCronWorkflow(startCtx, temporalClient, cfg.Dunning.TaskQueue, retryWorkflowID, cfg.Dunning.CronSchedule, dunningflow.CollectPaymentReminderWorkflowInput{
				TenantID: cfg.Dunning.TenantID,
				Limit:    cfg.Dunning.Batch,
			})
			cancel()
			if err != nil {
				closeReplayRuntime()
				fatal(logger, "ensure dunning retry cron workflow", "error", err)
			}
			logger.Info(
				"dunning retry scheduler ensured",
				"component", "server",
				"temporal_address", cfg.Temporal.Address,
				"temporal_namespace", cfg.Temporal.Namespace,
				"dunning_task_queue", cfg.Dunning.TaskQueue,
				"workflow_id", retryWorkflowID,
				"cron_schedule", cfg.Dunning.CronSchedule,
				"batch", cfg.Dunning.Batch,
				"tenant_id", cfg.Dunning.TenantID,
			)
		}
	}

	if len(cfg.SSO.OIDCProviders) > 0 {
		oidcProviders := make([]service.BrowserSSOProvider, 0, len(cfg.SSO.OIDCProviders))
		for _, providerCfg := range cfg.SSO.OIDCProviders {
			provider, providerErr := service.NewOIDCBrowserSSOProvider(ctx, service.OIDCBrowserSSOProviderConfig{
				Key:          providerCfg.Key,
				DisplayName:  providerCfg.DisplayName,
				IssuerURL:    providerCfg.IssuerURL,
				ClientID:     providerCfg.ClientID,
				ClientSecret: providerCfg.ClientSecret,
				Scopes:       providerCfg.Scopes,
			})
			if providerErr != nil {
				fatal(logger, "initialize oidc provider", "provider", providerCfg.Key, "error", providerErr)
			}
			oidcProviders = append(oidcProviders, provider)
		}
		browserSSOService, err := service.NewBrowserSSOService(
			repo,
			browserUserAuthService,
			oidcProviders,
			service.BrowserSSOServiceConfig{
				AutoProvisionUsers: cfg.SSO.AutoProvisionUsers,
			},
		)
		if err != nil {
			fatal(logger, "initialize browser sso service", "error", err)
		}
		serverOpts = append(serverOpts,
			api.WithBrowserSSOService(browserSSOService),
			api.WithUIPublicBaseURL(cfg.SSO.PublicBaseURL),
		)
		logger.Info(
			"browser sso enabled",
			"component", "server",
			"provider_count", len(cfg.SSO.OIDCProviders),
			"auto_provision_users", cfg.SSO.AutoProvisionUsers,
			"ui_public_base_url", cfg.SSO.PublicBaseURL,
		)
	} else {
		logger.Info("browser sso disabled", "component", "server")
	}

	if cfg.APIKeysRaw != "" {
		bootstrapResult, err := api.BootstrapAPIKeysFromConfig(repo, cfg.APIKeysRaw)
		if err != nil {
			fatal(logger, "bootstrap API_KEYS", "error", err)
		}
		logger.Info(
			"api auth bootstrap keys",
			"component", "server",
			"created", bootstrapResult.Created,
			"existing", bootstrapResult.Existing,
		)
	} else {
		logger.Info("api auth bootstrap skipped", "component", "server", "reason", "api_keys_env_empty")
	}

	if cfg.AuditExport.Enabled {
		objectStore, err := service.NewS3ObjectStore(context.Background(), cfg.AuditExport.S3)
		if err != nil {
			fatal(logger, "initialize audit export object store", "error", err)
		}

		ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := objectStore.EnsureBucket(ensureCtx); err != nil {
			ensureCancel()
			fatal(logger, "ensure audit export bucket", "error", err)
		}
		ensureCancel()

		auditExportSvc := service.NewAuditExportService(repo, objectStore, cfg.AuditExport.DownloadTTL)
		serverOpts = append(serverOpts, api.WithAuditExportService(auditExportSvc))

		auditExportWorker := service.NewAuditExportWorker(auditExportSvc, cfg.AuditExport.WorkerPoll)
		go auditExportWorker.Run(ctx)

		logger.Info("audit exports enabled", "component", "server", "backend", "s3")
	} else {
		logger.Info("audit exports disabled", "component", "server")
	}

	handler := api.NewServer(repo, serverOpts...).Handler()
	handler = uiSessionManager.LoadAndSave(handler)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		closeReplayRuntime()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("start server", "component", "server", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal(logger, "server failed", "error", err)
	}
}

func openDB(cfg appconfig.DBConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}

func buildMetricsProvider(replayTemporalDispatcher *replay.TemporalDispatcher, db *sql.DB) func() map[string]any {
	return func() map[string]any {
		ds := db.Stats()
		out := map[string]any{
			"replay_execution_mode": "temporal",
			"database": map[string]any{
				"max_open_connections": ds.MaxOpenConnections,
				"open_connections":     ds.OpenConnections,
				"in_use":               ds.InUse,
				"idle":                 ds.Idle,
				"wait_count":           ds.WaitCount,
				"wait_duration_ms":     ds.WaitDuration.Milliseconds(),
				"max_idle_closed":      ds.MaxIdleClosed,
				"max_lifetime_closed":  ds.MaxLifetimeClosed,
			},
		}
		if replayTemporalDispatcher != nil {
			out["replay_temporal_dispatcher"] = replayTemporalDispatcher.Stats()
		}
		return out
	}
}

func newBillingSecretStore(ctx context.Context, cfg appconfig.BillingProviderConfig) (service.BillingSecretStore, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.SecretStoreBackend)) {
	case "memory":
		return service.NewMemoryBillingSecretStore(), nil
	case "aws-secretsmanager":
		return service.NewAWSSecretsManagerBillingSecretStore(ctx, service.AWSSecretsManagerBillingSecretStoreConfig{
			Region:          cfg.SecretStoreAWSRegion,
			Endpoint:        cfg.SecretStoreAWSEndpoint,
			Prefix:          cfg.SecretStorePrefix,
			AccessKeyID:     cfg.SecretStoreAccessKeyID,
			SecretAccessKey: cfg.SecretStoreSecretAccessKey,
			SessionToken:    cfg.SecretStoreSessionToken,
		})
	case "":
		return nil, fmt.Errorf("billing provider secret store backend is required")
	default:
		return nil, fmt.Errorf("unsupported billing provider secret store backend %q", cfg.SecretStoreBackend)
	}
}
