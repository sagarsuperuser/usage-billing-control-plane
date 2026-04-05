package billingcycle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

const (
	DefaultBillingCycleTaskQueue    = "alpha-billing-cycle"
	DefaultBillingCycleWorkflowID   = "billing-cycle/generate-invoices"
	DefaultBillingCycleCronSchedule = "@every 5m"
	BillingCycleWorkflowName        = "alpha.billing_cycle.generate_invoices.workflow"
	BillingCycleRunActivityName     = "alpha.billing_cycle.generate_invoices.run_activity"
)

// ---------------------------------------------------------------------------
// Workflow input / definition
// ---------------------------------------------------------------------------

type GenerateInvoicesWorkflowInput struct {
	TenantID string `json:"tenant_id"`
	Batch    int    `json:"batch"`
}

func GenerateInvoicesWorkflow(ctx workflow.Context, input GenerateInvoicesWorkflowInput) error {
	if strings.TrimSpace(input.TenantID) == "" {
		return temporal.NewNonRetryableApplicationError("tenant_id is required", "validation_error", nil)
	}
	if input.Batch <= 0 {
		return temporal.NewNonRetryableApplicationError("batch must be > 0", "validation_error", nil)
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(ctx, BillingCycleRunActivityName, input).Get(ctx, nil)
}

// ---------------------------------------------------------------------------
// Activities
// ---------------------------------------------------------------------------

type BillingCycleActivities struct {
	generationSvc   *service.InvoiceGenerationService
	finalizationSvc *service.InvoiceFinalizationService
	store           store.Repository
	logger          *slog.Logger
}

type GenerateInvoicesBatchResult struct {
	Generated int `json:"generated"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

func NewBillingCycleActivities(
	repo store.Repository,
	db *sql.DB,
	stripeClient *service.StripeClient,
	secretStore service.BillingSecretStore,
	loggers ...*slog.Logger,
) *BillingCycleActivities {
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &BillingCycleActivities{
		generationSvc:   service.NewInvoiceGenerationService(repo, db),
		finalizationSvc: service.NewInvoiceFinalizationService(repo, stripeClient, secretStore).
			WithPDFService(service.NewInvoicePDFService(nil)),
		store:           repo,
		logger:          logger,
	}
}

func (a *BillingCycleActivities) RunGenerateInvoicesBatch(ctx context.Context, input GenerateInvoicesWorkflowInput) (GenerateInvoicesBatchResult, error) {
	if a == nil || a.generationSvc == nil {
		return GenerateInvoicesBatchResult{}, fmt.Errorf("billing cycle activities not configured")
	}

	now := time.Now().UTC()
	subs, err := a.store.GetSubscriptionsDueBilling(now, input.Batch)
	if err != nil {
		return GenerateInvoicesBatchResult{}, fmt.Errorf("get due subscriptions: %w", err)
	}

	var result GenerateInvoicesBatchResult
	for _, sub := range subs {
		if sub.NextBillingAt == nil || sub.CurrentBillingPeriodEnd == nil {
			result.Skipped++
			continue
		}

		genResult, err := a.generationSvc.Generate(ctx, service.GenerateInvoiceInput{
			TenantID:       sub.TenantID,
			SubscriptionID: sub.ID,
			PeriodStart:    *sub.CurrentBillingPeriodEnd,
			PeriodEnd:      advancePeriod(*sub.CurrentBillingPeriodEnd),
		})
		if err != nil {
			a.logger.Error("invoice generation failed",
				"component", "billing_cycle", "subscription_id", sub.ID, "tenant_id", sub.TenantID, "error", err)
			result.Errors++
			continue
		}
		if genResult.AlreadyExists {
			result.Skipped++
			continue
		}

		// Attempt immediate finalization (grace_period_days=0 is the default).
		// Look up the customer's Stripe ID for payment execution.
		paymentSetup, err := a.store.GetCustomerPaymentSetup(sub.TenantID, sub.CustomerID)
		if err != nil || paymentSetup.ProviderCustomerReference == "" {
			// Customer has no Stripe customer ID — finalization deferred.
			a.logger.Info("invoice finalization deferred: no stripe customer",
				"component", "billing_cycle", "invoice_id", genResult.Invoice.ID, "tenant_id", sub.TenantID)
			result.Generated++
			continue
		}

		_, err = a.finalizationSvc.Finalize(ctx, service.FinalizeInvoiceInput{
			TenantID:         sub.TenantID,
			InvoiceID:        genResult.Invoice.ID,
			StripeCustomerID: paymentSetup.ProviderCustomerReference,
		})
		if err != nil {
			a.logger.Error("invoice finalization failed",
				"component", "billing_cycle", "invoice_id", genResult.Invoice.ID, "tenant_id", sub.TenantID, "error", err)
			// Invoice was created as draft — finalization can be retried.
		}

		result.Generated++
		activity.RecordHeartbeat(ctx, result)
	}

	return result, nil
}

func advancePeriod(from time.Time) time.Time {
	// Default to monthly. Yearly intervals are handled by the generation service
	// which reads the plan's BillingInterval directly.
	return from.AddDate(0, 1, 0)
}

// ---------------------------------------------------------------------------
// Worker registration
// ---------------------------------------------------------------------------

func RegisterBillingCycleWorker(
	w worker.Worker,
	repo store.Repository,
	db *sql.DB,
	stripeClient *service.StripeClient,
	secretStore service.BillingSecretStore,
	loggers ...*slog.Logger,
) error {
	if repo == nil {
		return fmt.Errorf("repository is required")
	}
	activities := NewBillingCycleActivities(repo, db, stripeClient, secretStore, loggers...)
	w.RegisterWorkflowWithOptions(GenerateInvoicesWorkflow, workflow.RegisterOptions{Name: BillingCycleWorkflowName})
	w.RegisterActivityWithOptions(activities.RunGenerateInvoicesBatch, activity.RegisterOptions{Name: BillingCycleRunActivityName})
	return nil
}

// ---------------------------------------------------------------------------
// Cron workflow scheduler
// ---------------------------------------------------------------------------

func EnsureBillingCycleCronWorkflow(
	ctx context.Context,
	temporalClient client.Client,
	taskQueue string,
	workflowID string,
	cronSchedule string,
	input GenerateInvoicesWorkflowInput,
) error {
	if temporalClient == nil {
		return fmt.Errorf("temporal client is required")
	}
	if strings.TrimSpace(taskQueue) == "" {
		taskQueue = DefaultBillingCycleTaskQueue
	}
	if strings.TrimSpace(workflowID) == "" {
		workflowID = DefaultBillingCycleWorkflowID
	}
	if strings.TrimSpace(cronSchedule) == "" {
		cronSchedule = DefaultBillingCycleCronSchedule
	}
	input.TenantID = strings.TrimSpace(input.TenantID)
	if input.TenantID == "" {
		input.TenantID = "default"
	}
	if input.Batch <= 0 {
		input.Batch = 50
	}

	_, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             taskQueue,
		CronSchedule:          cronSchedule,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, BillingCycleWorkflowName, input)
	if err == nil {
		return nil
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return fmt.Errorf("start billing cycle cron workflow: %w", err)
}
