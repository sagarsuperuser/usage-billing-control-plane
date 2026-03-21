package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"usage-billing-control-plane/internal/appconfig"
	"usage-billing-control-plane/internal/logging"
)

type cleanupCounts struct {
	ReplaySmokeCustomers                int64 `json:"replay_smoke_customers"`
	ReplaySmokeCustomerBillingProfiles  int64 `json:"replay_smoke_customer_billing_profiles"`
	ReplaySmokeCustomerPaymentSetup     int64 `json:"replay_smoke_customer_payment_setup"`
	ReplaySmokeUsageEvents              int64 `json:"replay_smoke_usage_events"`
	ReplaySmokeBilledEntries            int64 `json:"replay_smoke_billed_entries"`
	ReplaySmokeReplayJobs               int64 `json:"replay_smoke_replay_jobs"`
	ReplaySmokeMeters                   int64 `json:"replay_smoke_meters"`
	ReplaySmokeRatingRuleVersions       int64 `json:"replay_smoke_rating_rule_versions"`
	PaymentSmokeCustomers               int64 `json:"payment_smoke_customers"`
	PaymentSmokeCustomerBillingProfiles int64 `json:"payment_smoke_customer_billing_profiles"`
	PaymentSmokeCustomerPaymentSetup    int64 `json:"payment_smoke_customer_payment_setup"`
	PaymentSmokeInvoicePaymentViews     int64 `json:"payment_smoke_invoice_payment_status_views"`
	PaymentSmokeLagoWebhookEvents       int64 `json:"payment_smoke_lago_webhook_events"`
	PlaywrightLivePlatformAPIKeys       int64 `json:"playwright_live_platform_api_keys"`
	PlaywrightLiveTenantAPIKeys         int64 `json:"playwright_live_tenant_api_keys"`
	PlaywrightLiveAPIKeyAuditEvents     int64 `json:"playwright_live_api_key_audit_events"`
	PlaywrightLiveAPIKeyExportJobs      int64 `json:"playwright_live_api_key_export_jobs"`
	PlaywrightLiveUsers                 int64 `json:"playwright_live_users"`
	PlaywrightLiveMemberships           int64 `json:"playwright_live_memberships"`
	PlaywrightLivePasswordCredentials   int64 `json:"playwright_live_password_credentials"`
	PlaywrightLivePasswordResetTokens   int64 `json:"playwright_live_password_reset_tokens"`
	PlaywrightLiveWorkspaceInvites      int64 `json:"playwright_live_workspace_invitations"`
}

type cleanupResult struct {
	Command     string        `json:"command"`
	Environment string        `json:"environment"`
	Applied     bool          `json:"applied"`
	GeneratedAt time.Time     `json:"generated_at"`
	Counts      cleanupCounts `json:"counts"`
}

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())
	if len(os.Args) < 2 {
		fatal(logger, "missing subcommand", "supported", []string{"cleanup-staging-fixtures", "ensure-tenant-lago-mapping", "ensure-tenant-workspace-billing", "ensure-alpha-customers", "upsert-customer-billing-profile"})
	}

	switch strings.ToLower(strings.TrimSpace(os.Args[1])) {
	case "cleanup-staging-fixtures":
		runCleanupStagingFixtures(logger, os.Args[2:])
	case "ensure-tenant-lago-mapping":
		runEnsureTenantLagoMapping(logger, os.Args[2:])
	case "ensure-tenant-workspace-billing":
		runEnsureTenantWorkspaceBilling(logger, os.Args[2:])
	case "ensure-alpha-customers":
		runEnsureAlphaCustomers(logger, os.Args[2:])
	case "upsert-customer-billing-profile":
		runUpsertCustomerBillingProfile(logger, os.Args[2:])
	default:
		fatal(logger, "unsupported subcommand", "subcommand", os.Args[1], "supported", []string{"cleanup-staging-fixtures", "ensure-tenant-lago-mapping", "ensure-tenant-workspace-billing", "ensure-alpha-customers", "upsert-customer-billing-profile"})
	}
}

func runCleanupStagingFixtures(logger *slog.Logger, args []string) {
	fs := flag.NewFlagSet("cleanup-staging-fixtures", flag.ExitOnError)
	var (
		environment        string
		output             string
		apply              bool
		includeReplay      bool
		includePayment     bool
		includeLiveBrowser bool
	)
	fs.StringVar(&environment, "environment", firstNonEmpty(strings.TrimSpace(os.Getenv("ENVIRONMENT")), strings.TrimSpace(os.Getenv("APP_ENV")), "staging"), "target environment; apply is restricted to staging")
	fs.StringVar(&output, "output", "json", "output format: json or text")
	fs.BoolVar(&apply, "apply", false, "apply the cleanup instead of reporting counts only")
	fs.BoolVar(&includeReplay, "include-replay-fixtures", true, "include replay smoke fixtures")
	fs.BoolVar(&includePayment, "include-payment-fixtures", true, "include payment smoke fixtures")
	fs.BoolVar(&includeLiveBrowser, "include-live-browser-fixtures", true, "include playwright live browser fixtures")
	_ = fs.Parse(args)

	output = strings.ToLower(strings.TrimSpace(output))
	if output != "json" && output != "text" {
		fatal(logger, "unsupported output format", "output", output)
	}

	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" {
		environment = "staging"
	}
	if apply && environment != "staging" {
		fatal(logger, "cleanup apply is restricted to staging", "environment", environment)
	}

	dbCfg, err := appconfig.LoadDBConfigFromEnv()
	if err != nil {
		fatal(logger, err.Error())
	}

	db, err := appconfig.OpenPostgres(dbCfg)
	if err != nil {
		fatal(logger, "open database", "error", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(dbCfg.QueryTimeout, 30*time.Second))
	defer cancel()

	counts, err := collectCleanupCounts(ctx, db, includeReplay, includePayment, includeLiveBrowser)
	if err != nil {
		fatal(logger, "collect cleanup counts", "error", err)
	}

	if apply {
		if err := applyCleanup(ctx, db, includeReplay, includePayment, includeLiveBrowser); err != nil {
			fatal(logger, "apply cleanup", "error", err)
		}
	}

	res := cleanupResult{
		Command:     "cleanup-staging-fixtures",
		Environment: environment,
		Applied:     apply,
		GeneratedAt: time.Now().UTC(),
		Counts:      counts,
	}

	switch output {
	case "text":
		printCleanupText(res)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fatal(logger, "encode cleanup result", "error", err)
		}
	}
}

func collectCleanupCounts(ctx context.Context, db *sql.DB, includeReplay, includePayment, includeLiveBrowser bool) (cleanupCounts, error) {
	var counts cleanupCounts
	var err error

	if includeReplay {
		if counts.ReplaySmokeCustomers, err = countQuery(ctx, db, `SELECT count(*) FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeCustomerBillingProfiles, err = countQuery(ctx, db, `SELECT count(*) FROM customer_billing_profiles WHERE customer_id IN (SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%')`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeCustomerPaymentSetup, err = countQuery(ctx, db, `SELECT count(*) FROM customer_payment_setup WHERE customer_id IN (SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%')`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeUsageEvents, err = countQuery(ctx, db, `SELECT count(*) FROM usage_events WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeBilledEntries, err = countQuery(ctx, db, `SELECT count(*) FROM billed_entries WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%' OR replay_job_id IN (SELECT id FROM replay_jobs WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%')`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeReplayJobs, err = countQuery(ctx, db, `SELECT count(*) FROM replay_jobs WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeMeters, err = countQuery(ctx, db, `SELECT count(*) FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.ReplaySmokeRatingRuleVersions, err = countQuery(ctx, db, `SELECT count(*) FROM rating_rule_versions WHERE name LIKE 'Replay Smoke Flat %' OR name = 'Replay Smoke Flat'`); err != nil {
			return cleanupCounts{}, err
		}
	}

	if includePayment {
		paymentCustomerClause := `external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR external_id LIKE 'cust_payment_smoke_%'`
		if counts.PaymentSmokeCustomers, err = countQuery(ctx, db, `SELECT count(*) FROM customers WHERE `+paymentCustomerClause); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PaymentSmokeCustomerBillingProfiles, err = countQuery(ctx, db, `SELECT count(*) FROM customer_billing_profiles WHERE customer_id IN (SELECT id FROM customers WHERE `+paymentCustomerClause+`)`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PaymentSmokeCustomerPaymentSetup, err = countQuery(ctx, db, `SELECT count(*) FROM customer_payment_setup WHERE customer_id IN (SELECT id FROM customers WHERE `+paymentCustomerClause+`)`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PaymentSmokeInvoicePaymentViews, err = countQuery(ctx, db, `SELECT count(*) FROM invoice_payment_status_views WHERE customer_external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR customer_external_id LIKE 'cust_payment_smoke_%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PaymentSmokeLagoWebhookEvents, err = countQuery(ctx, db, `SELECT count(*) FROM lago_webhook_events WHERE customer_external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR customer_external_id LIKE 'cust_payment_smoke_%'`); err != nil {
			return cleanupCounts{}, err
		}
	}

	if includeLiveBrowser {
		userClause := `lower(email) LIKE 'playwright-live-%@alpha.test'`
		if counts.PlaywrightLivePlatformAPIKeys, err = countQuery(ctx, db, `SELECT count(*) FROM platform_api_keys WHERE name LIKE 'playwright-live-%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveTenantAPIKeys, err = countQuery(ctx, db, `SELECT count(*) FROM api_keys WHERE name LIKE 'playwright-live-%'`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveAPIKeyAuditEvents, err = countQuery(ctx, db, `SELECT count(*) FROM api_key_audit_events WHERE api_key_id IN (SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%' UNION ALL SELECT id FROM platform_api_keys WHERE name LIKE 'playwright-live-%')`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveAPIKeyExportJobs, err = countQuery(ctx, db, `SELECT count(*) FROM api_key_audit_export_jobs WHERE requested_by_api_key_id IN (SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%')`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveUsers, err = countQuery(ctx, db, `SELECT count(*) FROM users WHERE `+userClause); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveMemberships, err = countQuery(ctx, db, `SELECT count(*) FROM user_tenant_memberships WHERE user_id IN (SELECT id FROM users WHERE `+userClause+`)`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLivePasswordCredentials, err = countQuery(ctx, db, `SELECT count(*) FROM user_password_credentials WHERE user_id IN (SELECT id FROM users WHERE `+userClause+`)`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLivePasswordResetTokens, err = countQuery(ctx, db, `SELECT count(*) FROM password_reset_tokens WHERE user_id IN (SELECT id FROM users WHERE `+userClause+`)`); err != nil {
			return cleanupCounts{}, err
		}
		if counts.PlaywrightLiveWorkspaceInvites, err = countQuery(ctx, db, `SELECT count(*) FROM workspace_invitations WHERE lower(email) LIKE 'playwright-live-%@alpha.test'`); err != nil {
			return cleanupCounts{}, err
		}
	}

	return counts, nil
}

func applyCleanup(ctx context.Context, db *sql.DB, includeReplay, includePayment, includeLiveBrowser bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var statements []string
	if includeLiveBrowser {
		statements = append(statements,
			`DELETE FROM api_key_audit_events WHERE api_key_id IN (SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%' UNION ALL SELECT id FROM platform_api_keys WHERE name LIKE 'playwright-live-%')`,
			`DELETE FROM api_key_audit_export_jobs WHERE requested_by_api_key_id IN (SELECT id FROM api_keys WHERE name LIKE 'playwright-live-%')`,
			`DELETE FROM platform_api_keys WHERE name LIKE 'playwright-live-%'`,
			`DELETE FROM api_keys WHERE name LIKE 'playwright-live-%'`,
			`DELETE FROM workspace_invitations WHERE lower(email) LIKE 'playwright-live-%@alpha.test'`,
			`DELETE FROM users WHERE lower(email) LIKE 'playwright-live-%@alpha.test'`,
		)
	}
	if includePayment {
		paymentCustomerClause := `external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR external_id LIKE 'cust_payment_smoke_%'`
		statements = append(statements,
			`DELETE FROM lago_webhook_events WHERE customer_external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR customer_external_id LIKE 'cust_payment_smoke_%'`,
			`DELETE FROM invoice_payment_status_views WHERE customer_external_id IN ('cust_e2e_success', 'cust_e2e_failure') OR customer_external_id LIKE 'cust_payment_smoke_%'`,
			`DELETE FROM customer_payment_setup WHERE customer_id IN (SELECT id FROM customers WHERE `+paymentCustomerClause+`)`,
			`DELETE FROM customer_billing_profiles WHERE customer_id IN (SELECT id FROM customers WHERE `+paymentCustomerClause+`)`,
			`DELETE FROM customers WHERE `+paymentCustomerClause,
		)
	}
	if includeReplay {
		statements = append(statements,
			`DELETE FROM billed_entries WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%' OR replay_job_id IN (SELECT id FROM replay_jobs WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%')`,
			`DELETE FROM replay_jobs WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%'`,
			`DELETE FROM usage_events WHERE customer_id IN (SELECT external_id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%') OR meter_id IN (SELECT id FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter') OR idempotency_key LIKE 'replay-smoke-%'`,
			`DELETE FROM customer_payment_setup WHERE customer_id IN (SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%')`,
			`DELETE FROM customer_billing_profiles WHERE customer_id IN (SELECT id FROM customers WHERE external_id LIKE 'cust_replay_smoke_%')`,
			`DELETE FROM customers WHERE external_id LIKE 'cust_replay_smoke_%'`,
			`DELETE FROM meters WHERE meter_key LIKE 'replay_smoke_meter_%' OR meter_key = 'replay_smoke_meter'`,
			`DELETE FROM rating_rule_versions WHERE name LIKE 'Replay Smoke Flat %' OR name = 'Replay Smoke Flat'`,
		)
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func countQuery(ctx context.Context, db *sql.DB, query string) (int64, error) {
	var n int64
	if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func printCleanupText(res cleanupResult) {
	fmt.Printf("command=%s\n", res.Command)
	fmt.Printf("environment=%s\n", res.Environment)
	fmt.Printf("applied=%t\n", res.Applied)
	fmt.Printf("generated_at=%s\n", res.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("replay_smoke_customers=%d\n", res.Counts.ReplaySmokeCustomers)
	fmt.Printf("replay_smoke_customer_billing_profiles=%d\n", res.Counts.ReplaySmokeCustomerBillingProfiles)
	fmt.Printf("replay_smoke_customer_payment_setup=%d\n", res.Counts.ReplaySmokeCustomerPaymentSetup)
	fmt.Printf("replay_smoke_usage_events=%d\n", res.Counts.ReplaySmokeUsageEvents)
	fmt.Printf("replay_smoke_billed_entries=%d\n", res.Counts.ReplaySmokeBilledEntries)
	fmt.Printf("replay_smoke_replay_jobs=%d\n", res.Counts.ReplaySmokeReplayJobs)
	fmt.Printf("replay_smoke_meters=%d\n", res.Counts.ReplaySmokeMeters)
	fmt.Printf("replay_smoke_rating_rule_versions=%d\n", res.Counts.ReplaySmokeRatingRuleVersions)
	fmt.Printf("payment_smoke_customers=%d\n", res.Counts.PaymentSmokeCustomers)
	fmt.Printf("payment_smoke_customer_billing_profiles=%d\n", res.Counts.PaymentSmokeCustomerBillingProfiles)
	fmt.Printf("payment_smoke_customer_payment_setup=%d\n", res.Counts.PaymentSmokeCustomerPaymentSetup)
	fmt.Printf("payment_smoke_invoice_payment_status_views=%d\n", res.Counts.PaymentSmokeInvoicePaymentViews)
	fmt.Printf("payment_smoke_lago_webhook_events=%d\n", res.Counts.PaymentSmokeLagoWebhookEvents)
	fmt.Printf("playwright_live_platform_api_keys=%d\n", res.Counts.PlaywrightLivePlatformAPIKeys)
	fmt.Printf("playwright_live_tenant_api_keys=%d\n", res.Counts.PlaywrightLiveTenantAPIKeys)
	fmt.Printf("playwright_live_api_key_audit_events=%d\n", res.Counts.PlaywrightLiveAPIKeyAuditEvents)
	fmt.Printf("playwright_live_api_key_export_jobs=%d\n", res.Counts.PlaywrightLiveAPIKeyExportJobs)
	fmt.Printf("playwright_live_users=%d\n", res.Counts.PlaywrightLiveUsers)
	fmt.Printf("playwright_live_memberships=%d\n", res.Counts.PlaywrightLiveMemberships)
	fmt.Printf("playwright_live_password_credentials=%d\n", res.Counts.PlaywrightLivePasswordCredentials)
	fmt.Printf("playwright_live_password_reset_tokens=%d\n", res.Counts.PlaywrightLivePasswordResetTokens)
	fmt.Printf("playwright_live_workspace_invitations=%d\n", res.Counts.PlaywrightLiveWorkspaceInvites)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
