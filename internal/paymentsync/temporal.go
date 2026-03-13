package paymentsync

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
	"usage-billing-control-plane/internal/store"
)

const (
	DefaultTemporalPaymentReconcileTaskQueue = "alpha-payment-reconcile"
	DefaultPaymentReconcileWorkflowID        = "payment-reconcile/cron"
	DefaultPaymentReconcileCronSchedule      = "@every 2m"
	PaymentReconcileWorkflowName             = "alpha.payment.reconcile.workflow"
	PaymentReconcileRunActivityName          = "alpha.payment.reconcile.run_activity"
)

type PaymentReconcileWorkflowInput struct {
	Limit             int `json:"limit"`
	StaleAfterSeconds int `json:"stale_after_seconds"`
}

func PaymentReconcileWorkflow(ctx workflow.Context, input PaymentReconcileWorkflowInput) error {
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
	return workflow.ExecuteActivity(ctx, PaymentReconcileRunActivityName, input).Get(ctx, nil)
}

type ReconcileActivities struct {
	service *service.PaymentReconcileService
}

func NewReconcileActivities(repo store.Repository, lagoClient *service.LagoClient) (*ReconcileActivities, error) {
	svc, err := service.NewPaymentReconcileService(repo, lagoClient)
	if err != nil {
		return nil, err
	}
	return &ReconcileActivities{service: svc}, nil
}

func (a *ReconcileActivities) RunPaymentReconcileBatch(ctx context.Context, input PaymentReconcileWorkflowInput) (service.PaymentReconcileBatchResult, error) {
	if a == nil || a.service == nil {
		return service.PaymentReconcileBatchResult{}, fmt.Errorf("payment reconcile activities not configured")
	}
	return a.service.ReconcileBatch(ctx, service.PaymentReconcileBatchRequest{
		Limit:      input.Limit,
		StaleAfter: time.Duration(input.StaleAfterSeconds) * time.Second,
	})
}

func RegisterTemporalPaymentReconcileWorker(w worker.Worker, repo store.Repository, lagoClient *service.LagoClient) error {
	activities, err := NewReconcileActivities(repo, lagoClient)
	if err != nil {
		return err
	}
	w.RegisterWorkflowWithOptions(PaymentReconcileWorkflow, workflow.RegisterOptions{Name: PaymentReconcileWorkflowName})
	w.RegisterActivityWithOptions(activities.RunPaymentReconcileBatch, activity.RegisterOptions{Name: PaymentReconcileRunActivityName})
	return nil
}

func EnsurePaymentReconcileCronWorkflow(
	ctx context.Context,
	temporalClient client.Client,
	taskQueue string,
	workflowID string,
	cronSchedule string,
	input PaymentReconcileWorkflowInput,
) error {
	if temporalClient == nil {
		return fmt.Errorf("temporal client is required")
	}
	if strings.TrimSpace(taskQueue) == "" {
		taskQueue = DefaultTemporalPaymentReconcileTaskQueue
	}
	if strings.TrimSpace(workflowID) == "" {
		workflowID = DefaultPaymentReconcileWorkflowID
	}
	if strings.TrimSpace(cronSchedule) == "" {
		cronSchedule = DefaultPaymentReconcileCronSchedule
	}
	if input.Limit <= 0 {
		input.Limit = 100
	}
	if input.StaleAfterSeconds <= 0 {
		input.StaleAfterSeconds = 300
	}

	_, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             taskQueue,
		CronSchedule:          cronSchedule,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, PaymentReconcileWorkflowName, input)
	if err == nil {
		return nil
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}
