package billingcheck

import (
	"context"
	"errors"
	"fmt"
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
)

const (
	DefaultTemporalBillingConnectionCheckTaskQueue = "alpha-billing-connection-check"
	DefaultBillingConnectionCheckWorkflowID        = "billing-connections/check/cron"
	DefaultBillingConnectionCheckCronSchedule      = "@every 15m"
	BillingConnectionCheckWorkflowName             = "alpha.billing_connections.check.workflow"
	BillingConnectionCheckRunActivityName          = "alpha.billing_connections.check.run_activity"
)

type BillingConnectionCheckWorkflowInput struct {
	Limit             int `json:"limit"`
	StaleAfterSeconds int `json:"stale_after_seconds"`
}

type Activities struct {
	service *service.BillingProviderConnectionService
}

func BillingConnectionCheckWorkflow(ctx workflow.Context, input BillingConnectionCheckWorkflowInput) error {
	if input.Limit <= 0 {
		return temporal.NewNonRetryableApplicationError("limit must be > 0", "validation_error", nil)
	}
	if input.StaleAfterSeconds <= 0 {
		return temporal.NewNonRetryableApplicationError("stale_after_seconds must be > 0", "validation_error", nil)
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
	return workflow.ExecuteActivity(ctx, BillingConnectionCheckRunActivityName, input).Get(ctx, nil)
}

func NewActivities(service *service.BillingProviderConnectionService) (*Activities, error) {
	if service == nil {
		return nil, fmt.Errorf("billing provider connection service is required")
	}
	return &Activities{service: service}, nil
}

func (a *Activities) RunBillingConnectionCheckBatch(ctx context.Context, input BillingConnectionCheckWorkflowInput) (service.BillingProviderConnectionRecheckBatchResult, error) {
	if a == nil || a.service == nil {
		return service.BillingProviderConnectionRecheckBatchResult{}, fmt.Errorf("billing connection check activities not configured")
	}
	return a.service.RecheckBillingProviderConnectionsBatch(ctx, service.BillingProviderConnectionRecheckBatchRequest{
		Limit:      input.Limit,
		StaleAfter: time.Duration(input.StaleAfterSeconds) * time.Second,
	})
}

func RegisterTemporalBillingConnectionCheckWorker(w worker.Worker, billingProviderService *service.BillingProviderConnectionService) error {
	activities, err := NewActivities(billingProviderService)
	if err != nil {
		return fmt.Errorf("create billing connection check activities: %w", err)
	}
	w.RegisterWorkflowWithOptions(BillingConnectionCheckWorkflow, workflow.RegisterOptions{Name: BillingConnectionCheckWorkflowName})
	w.RegisterActivityWithOptions(activities.RunBillingConnectionCheckBatch, activity.RegisterOptions{Name: BillingConnectionCheckRunActivityName})
	return nil
}

func EnsureBillingConnectionCheckCronWorkflow(
	ctx context.Context,
	temporalClient client.Client,
	taskQueue string,
	workflowID string,
	cronSchedule string,
	input BillingConnectionCheckWorkflowInput,
) error {
	if temporalClient == nil {
		return fmt.Errorf("temporal client is required")
	}
	if strings.TrimSpace(taskQueue) == "" {
		taskQueue = DefaultTemporalBillingConnectionCheckTaskQueue
	}
	if strings.TrimSpace(workflowID) == "" {
		workflowID = DefaultBillingConnectionCheckWorkflowID
	}
	if strings.TrimSpace(cronSchedule) == "" {
		cronSchedule = DefaultBillingConnectionCheckCronSchedule
	}
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.StaleAfterSeconds <= 0 {
		input.StaleAfterSeconds = 21600
	}

	_, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             taskQueue,
		CronSchedule:          cronSchedule,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, BillingConnectionCheckWorkflowName, input)
	if err == nil {
		return nil
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return fmt.Errorf("start billing connection check cron workflow: %w", err)
}
