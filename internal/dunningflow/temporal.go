package dunningflow

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
	DefaultTemporalDunningTaskQueue      = "alpha-dunning"
	DefaultDunningWorkflowID             = "dunning/collect-payment-reminders"
	DefaultDunningCronSchedule           = "@every 2m"
	DunningCollectPaymentWorkflowName    = "alpha.dunning.collect_payment.workflow"
	DunningCollectPaymentRunActivityName = "alpha.dunning.collect_payment.run_activity"
)

type CollectPaymentReminderWorkflowInput struct {
	TenantID string `json:"tenant_id"`
	Limit    int    `json:"limit"`
}

func CollectPaymentReminderWorkflow(ctx workflow.Context, input CollectPaymentReminderWorkflowInput) error {
	if strings.TrimSpace(input.TenantID) == "" {
		return temporal.NewNonRetryableApplicationError("tenant_id is required", "validation_error", nil)
	}
	if input.Limit <= 0 {
		return temporal.NewNonRetryableApplicationError("limit must be > 0", "validation_error", nil)
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
	return workflow.ExecuteActivity(ctx, DunningCollectPaymentRunActivityName, input).Get(ctx, nil)
}

type DunningActivities struct {
	service *service.DunningService
}

func NewDunningActivities(repo store.Repository, customerAdapter service.CustomerBillingAdapter, notifications *service.NotificationService) (*DunningActivities, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if customerAdapter == nil {
		return nil, fmt.Errorf("customer billing adapter is required")
	}
	if notifications == nil {
		return nil, fmt.Errorf("notification service is required")
	}
	customerSvc := service.NewCustomerService(repo, customerAdapter).WithWorkspaceBillingBindingService(service.NewWorkspaceBillingBindingService(repo))
	requestSvc := service.NewCustomerPaymentSetupRequestService(repo, customerSvc, notifications)
	dunningSvc, err := service.NewDunningService(repo)
	if err != nil {
		return nil, err
	}
	return &DunningActivities{service: dunningSvc.WithPaymentSetupRequestSender(requestSvc)}, nil
}

func (a *DunningActivities) RunCollectPaymentReminderBatch(ctx context.Context, input CollectPaymentReminderWorkflowInput) (service.DunningCollectPaymentReminderBatchResult, error) {
	if a == nil || a.service == nil {
		return service.DunningCollectPaymentReminderBatchResult{}, fmt.Errorf("dunning activities not configured")
	}
	return a.service.ProcessCollectPaymentReminderBatch(input.TenantID, input.Limit)
}

func RegisterTemporalDunningWorker(w worker.Worker, repo store.Repository, customerAdapter service.CustomerBillingAdapter, notifications *service.NotificationService) error {
	activities, err := NewDunningActivities(repo, customerAdapter, notifications)
	if err != nil {
		return err
	}
	w.RegisterWorkflowWithOptions(CollectPaymentReminderWorkflow, workflow.RegisterOptions{Name: DunningCollectPaymentWorkflowName})
	w.RegisterActivityWithOptions(activities.RunCollectPaymentReminderBatch, activity.RegisterOptions{Name: DunningCollectPaymentRunActivityName})
	return nil
}

func EnsureCollectPaymentReminderCronWorkflow(
	ctx context.Context,
	temporalClient client.Client,
	taskQueue string,
	workflowID string,
	cronSchedule string,
	input CollectPaymentReminderWorkflowInput,
) error {
	if temporalClient == nil {
		return fmt.Errorf("temporal client is required")
	}
	if strings.TrimSpace(taskQueue) == "" {
		taskQueue = DefaultTemporalDunningTaskQueue
	}
	if strings.TrimSpace(workflowID) == "" {
		workflowID = DefaultDunningWorkflowID
	}
	if strings.TrimSpace(cronSchedule) == "" {
		cronSchedule = DefaultDunningCronSchedule
	}
	input.TenantID = strings.TrimSpace(input.TenantID)
	if input.TenantID == "" {
		input.TenantID = "default"
	}
	if input.Limit <= 0 {
		input.Limit = 20
	}

	_, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             taskQueue,
		CronSchedule:          cronSchedule,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, DunningCollectPaymentWorkflowName, input)
	if err == nil {
		return nil
	}

	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}
