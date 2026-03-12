package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	temporalclient "go.temporal.io/sdk/client"
	temporalsdkworker "go.temporal.io/sdk/worker"

	"lago-usage-billing-alpha/internal/api"
	"lago-usage-billing-alpha/internal/paymentsync"
	"lago-usage-billing-alpha/internal/replay"
	"lago-usage-billing-alpha/internal/service"
	"lago-usage-billing-alpha/internal/store"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(getIntEnv("DB_MAX_OPEN_CONNS", 20))
	db.SetMaxIdleConns(getIntEnv("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(time.Duration(getIntEnv("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), time.Duration(getIntEnv("DB_PING_TIMEOUT_SEC", 5))*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	queryTimeout := time.Duration(getIntEnv("DB_QUERY_TIMEOUT_MS", 5000)) * time.Millisecond
	migrationTimeout := time.Duration(getIntEnv("DB_MIGRATION_TIMEOUT_SEC", 60)) * time.Second
	repo := store.NewPostgresStore(
		db,
		store.WithQueryTimeout(queryTimeout),
		store.WithMigrationTimeout(migrationTimeout),
	)
	if getBoolEnv("RUN_MIGRATIONS_ON_BOOT", false) {
		if err := repo.Migrate(); err != nil {
			log.Fatalf("failed to run migrations: %v", err)
		}
		log.Printf("level=info component=server event=boot_migrations_applied")
	} else {
		log.Printf("level=info component=server event=boot_migrations_skipped")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runAPIServer := getBoolEnv("RUN_API_SERVER", true)
	runReplayWorker := getBoolEnv("RUN_REPLAY_WORKER", true)
	runReplayDispatcher := getBoolEnv("RUN_REPLAY_DISPATCHER", true)
	runPaymentReconcileWorker := getBoolEnv("RUN_PAYMENT_RECONCILE_WORKER", false)
	runPaymentReconcileScheduler := getBoolEnv("RUN_PAYMENT_RECONCILE_SCHEDULER", false)
	if !runAPIServer && !runReplayWorker && !runReplayDispatcher && !runPaymentReconcileWorker && !runPaymentReconcileScheduler {
		log.Fatal("at least one role must be enabled: RUN_API_SERVER, RUN_REPLAY_WORKER, RUN_REPLAY_DISPATCHER, RUN_PAYMENT_RECONCILE_WORKER, RUN_PAYMENT_RECONCILE_SCHEDULER")
	}
	if !runAPIServer && (runPaymentReconcileWorker || runPaymentReconcileScheduler) {
		log.Fatal("payment reconcile roles require RUN_API_SERVER=true")
	}
	log.Printf(
		"level=info component=server event=role_config api=%t replay_worker=%t replay_dispatcher=%t payment_reconcile_worker=%t payment_reconcile_scheduler=%t",
		runAPIServer,
		runReplayWorker,
		runReplayDispatcher,
		runPaymentReconcileWorker,
		runPaymentReconcileScheduler,
	)

	temporalAddress := strings.TrimSpace(os.Getenv("TEMPORAL_ADDRESS"))
	if temporalAddress == "" {
		temporalAddress = "localhost:7233"
	}
	temporalNamespace := strings.TrimSpace(os.Getenv("TEMPORAL_NAMESPACE"))
	if temporalNamespace == "" {
		temporalNamespace = "default"
	}
	replayTaskQueue := strings.TrimSpace(os.Getenv("REPLAY_TEMPORAL_TASK_QUEUE"))
	if replayTaskQueue == "" {
		replayTaskQueue = replay.DefaultTemporalReplayTaskQueue
	}

	var (
		temporalClient           temporalclient.Client
		temporalReplayWorker     temporalsdkworker.Worker
		temporalPaymentWorker    temporalsdkworker.Worker
		replayTemporalDispatcher *replay.TemporalDispatcher
	)
	if runReplayWorker || runReplayDispatcher || runPaymentReconcileWorker || runPaymentReconcileScheduler {
		temporalClient, err = temporalclient.Dial(temporalclient.Options{
			HostPort:  temporalAddress,
			Namespace: temporalNamespace,
		})
		if err != nil {
			log.Fatalf("failed to initialize temporal client: %v", err)
		}

		if runReplayWorker {
			temporalReplayWorker = temporalsdkworker.New(temporalClient, replayTaskQueue, temporalsdkworker.Options{})
			replay.RegisterTemporalReplayWorker(temporalReplayWorker, repo)
			if err := temporalReplayWorker.Start(); err != nil {
				temporalClient.Close()
				log.Fatalf("failed to start temporal replay worker: %v", err)
			}
			log.Printf(
				"level=info component=server event=replay_worker_started temporal_address=%s temporal_namespace=%s replay_task_queue=%s",
				temporalAddress,
				temporalNamespace,
				replayTaskQueue,
			)
		}

		if runReplayDispatcher {
			dispatcherPoll := time.Duration(getIntEnv("REPLAY_TEMPORAL_DISPATCH_POLL_MS", 750)) * time.Millisecond
			dispatcherBatch := getIntEnv("REPLAY_TEMPORAL_DISPATCH_BATCH", 25)
			replayTemporalDispatcher = replay.NewTemporalDispatcher(repo, temporalClient, replayTaskQueue, dispatcherPoll, dispatcherBatch)
			go replayTemporalDispatcher.Run(ctx)
			log.Printf(
				"level=info component=server event=replay_dispatcher_started temporal_address=%s temporal_namespace=%s replay_task_queue=%s poll_ms=%d batch=%d",
				temporalAddress,
				temporalNamespace,
				replayTaskQueue,
				dispatcherPoll.Milliseconds(),
				dispatcherBatch,
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
		if temporalClient != nil {
			temporalClient.Close()
		}
	}

	if !runAPIServer {
		log.Printf("level=info component=server event=roles_only_mode waiting_for_shutdown")
		<-ctx.Done()
		closeReplayRuntime()
		log.Printf("level=info component=server event=shutdown_complete")
		return
	}

	serverOpts := []api.ServerOption{
		api.WithMetricsProvider(buildMetricsProvider(replayTemporalDispatcher, db)),
		api.WithReadinessCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		}),
	}

	lagoAPIURL := strings.TrimSpace(os.Getenv("LAGO_API_URL"))
	if lagoAPIURL == "" {
		log.Fatal("LAGO_API_URL is required")
	}
	lagoAPIKey := strings.TrimSpace(os.Getenv("LAGO_API_KEY"))
	if lagoAPIKey == "" {
		log.Fatal("LAGO_API_KEY is required")
	}
	lagoClient, err := service.NewLagoClient(service.LagoClientConfig{
		BaseURL: lagoAPIURL,
		APIKey:  lagoAPIKey,
		Timeout: time.Duration(getIntEnv("LAGO_HTTP_TIMEOUT_MS", 10000)) * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("failed to initialize lago client: %v", err)
	}
	serverOpts = append(serverOpts, api.WithLagoClient(lagoClient))
	log.Printf("level=info component=server event=lago_adapter_enabled base_url=%s", lagoAPIURL)

	if runPaymentReconcileWorker || runPaymentReconcileScheduler {
		if temporalClient == nil {
			log.Fatal("temporal client is required when payment reconcile roles are enabled")
		}

		paymentTaskQueue := strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_TEMPORAL_TASK_QUEUE"))
		if paymentTaskQueue == "" {
			paymentTaskQueue = paymentsync.DefaultTemporalPaymentReconcileTaskQueue
		}

		if runPaymentReconcileWorker {
			temporalPaymentWorker = temporalsdkworker.New(temporalClient, paymentTaskQueue, temporalsdkworker.Options{})
			if err := paymentsync.RegisterTemporalPaymentReconcileWorker(temporalPaymentWorker, repo, lagoClient); err != nil {
				log.Fatalf("failed to register payment reconcile worker: %v", err)
			}
			if err := temporalPaymentWorker.Start(); err != nil {
				closeReplayRuntime()
				log.Fatalf("failed to start payment reconcile worker: %v", err)
			}
			log.Printf(
				"level=info component=server event=payment_reconcile_worker_started temporal_address=%s temporal_namespace=%s task_queue=%s",
				temporalAddress,
				temporalNamespace,
				paymentTaskQueue,
			)
		}

		if runPaymentReconcileScheduler {
			schedule := strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_CRON_SCHEDULE"))
			workflowID := strings.TrimSpace(os.Getenv("PAYMENT_RECONCILE_WORKFLOW_ID"))
			if schedule == "" {
				schedule = paymentsync.DefaultPaymentReconcileCronSchedule
			}
			if workflowID == "" {
				workflowID = paymentsync.DefaultPaymentReconcileWorkflowID
			}
			input := paymentsync.PaymentReconcileWorkflowInput{
				Limit:             getIntEnv("PAYMENT_RECONCILE_BATCH", 100),
				StaleAfterSeconds: getIntEnv("PAYMENT_RECONCILE_STALE_AFTER_SEC", 300),
			}

			startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := paymentsync.EnsurePaymentReconcileCronWorkflow(startCtx, temporalClient, paymentTaskQueue, workflowID, schedule, input)
			cancel()
			if err != nil {
				closeReplayRuntime()
				log.Fatalf("failed to ensure payment reconcile cron workflow: %v", err)
			}
			log.Printf(
				"level=info component=server event=payment_reconcile_scheduler_enabled task_queue=%s workflow_id=%s cron=%s batch=%d stale_after_sec=%d",
				paymentTaskQueue,
				workflowID,
				schedule,
				input.Limit,
				input.StaleAfterSeconds,
			)
		}
	}

	orgTenantMap, err := service.ParseLagoOrganizationTenantMap(strings.TrimSpace(os.Getenv("LAGO_ORG_TENANT_MAP")))
	if err != nil {
		log.Fatalf("failed to parse LAGO_ORG_TENANT_MAP: %v", err)
	}
	webhookVerifier, err := service.NewLagoJWTWebhookVerifier(
		lagoClient,
		time.Duration(getIntEnv("LAGO_WEBHOOK_PUBLIC_KEY_TTL_SEC", 300))*time.Second,
	)
	if err != nil {
		log.Fatalf("failed to initialize lago webhook verifier: %v", err)
	}
	lagoWebhookSvc := service.NewLagoWebhookService(
		repo,
		webhookVerifier,
		service.NewStaticLagoOrganizationTenantMapper("default", orgTenantMap),
	)
	serverOpts = append(serverOpts, api.WithLagoWebhookService(lagoWebhookSvc))
	log.Printf("level=info component=server event=lago_webhook_sync_enabled mapper_entries=%d", len(orgTenantMap))

	if getBoolEnv("API_AUTH_ENABLED", true) {
		authorizer, err := api.NewDBAPIKeyAuthorizer(repo)
		if err != nil {
			log.Fatalf("failed to initialize api key authorizer: %v", err)
		}
		serverOpts = append(serverOpts, api.WithAPIKeyAuthorizer(authorizer))
		log.Printf("level=info component=server event=api_auth_enabled backend=postgres")

		rawAPIKeys := strings.TrimSpace(os.Getenv("API_KEYS"))
		if rawAPIKeys != "" {
			bootstrapResult, err := api.BootstrapAPIKeysFromConfig(repo, rawAPIKeys)
			if err != nil {
				log.Fatalf("failed to bootstrap API_KEYS: %v", err)
			}
			log.Printf(
				"level=info component=server event=api_auth_bootstrap_keys created=%d existing=%d",
				bootstrapResult.Created,
				bootstrapResult.Existing,
			)
		} else {
			log.Printf("level=info component=server event=api_auth_bootstrap_skipped reason=api_keys_env_empty")
		}
	} else {
		log.Printf("level=warn component=server event=api_auth_disabled")
	}

	if getBoolEnv("AUDIT_EXPORTS_ENABLED", false) {
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

		objectStore, err := service.NewS3ObjectStore(context.Background(), service.S3Config{
			Region:          strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_REGION")),
			Bucket:          strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_BUCKET")),
			Endpoint:        strings.TrimSpace(os.Getenv("AUDIT_EXPORT_S3_ENDPOINT")),
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
			SessionToken:    sessionToken,
			ForcePathStyle:  getBoolEnv("AUDIT_EXPORT_S3_FORCE_PATH_STYLE", true),
		})
		if err != nil {
			log.Fatalf("failed to initialize audit export object store: %v", err)
		}

		ensureCtx, ensureCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := objectStore.EnsureBucket(ensureCtx); err != nil {
			ensureCancel()
			log.Fatalf("failed to ensure audit export bucket: %v", err)
		}
		ensureCancel()

		downloadTTL := time.Duration(getIntEnv("AUDIT_EXPORT_DOWNLOAD_TTL_SEC", 86400)) * time.Second
		auditExportSvc := service.NewAuditExportService(repo, objectStore, downloadTTL)
		serverOpts = append(serverOpts, api.WithAuditExportService(auditExportSvc))

		auditExportPoll := time.Duration(getIntEnv("AUDIT_EXPORT_WORKER_POLL_MS", 500)) * time.Millisecond
		auditExportWorker := service.NewAuditExportWorker(auditExportSvc, auditExportPoll)
		go auditExportWorker.Run(ctx)

		log.Printf("level=info component=server event=audit_exports_enabled backend=s3")
	} else {
		log.Printf("level=info component=server event=audit_exports_disabled")
	}

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewServer(repo, serverOpts...).Handler(),
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

	log.Printf("level=info component=server event=start addr=%s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
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

func getIntEnv(key string, defaultVal int) int {
	raw := os.Getenv(key)
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
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}

	switch raw {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return defaultVal
	}
}
