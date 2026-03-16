package replay

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"usage-billing-control-plane/internal/store"
)

const (
	DefaultTemporalReplayTaskQueue = "alpha-replay-jobs"
	ReplayWorkflowName             = "alpha.replay.workflow"
	ReplayRunActivityName          = "alpha.replay.run_activity"
)

type ReplayJobWorkflowInput struct {
	TenantID string `json:"tenant_id"`
	JobID    string `json:"job_id"`
}

func ReplayWorkflowID(jobID string) string {
	return "replay-job/" + strings.TrimSpace(jobID)
}

func ReplayJobWorkflow(ctx workflow.Context, input ReplayJobWorkflowInput) error {
	jobID := strings.TrimSpace(input.JobID)
	if jobID == "" {
		return temporal.NewNonRetryableApplicationError("job_id is required", "validation_error", nil)
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    1,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	return workflow.ExecuteActivity(ctx, ReplayRunActivityName, input).Get(ctx, nil)
}

type ReplayActivities struct {
	processor *Processor
}

func NewReplayActivities(repo store.Repository) *ReplayActivities {
	return &ReplayActivities{processor: NewProcessor(repo)}
}

func (a *ReplayActivities) RunReplayJob(ctx context.Context, input ReplayJobWorkflowInput) error {
	input.JobID = strings.TrimSpace(input.JobID)
	input.TenantID = strings.TrimSpace(input.TenantID)
	if input.JobID == "" {
		return fmt.Errorf("job_id is required")
	}
	if input.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	_, _, _, err := a.processor.ProcessJobByID(input.TenantID, input.JobID)
	return err
}

func RegisterTemporalReplayWorker(w worker.Worker, repo store.Repository) {
	activities := NewReplayActivities(repo)
	w.RegisterWorkflowWithOptions(ReplayJobWorkflow, workflow.RegisterOptions{Name: ReplayWorkflowName})
	w.RegisterActivityWithOptions(activities.RunReplayJob, activity.RegisterOptions{Name: ReplayRunActivityName})
}

type TemporalDispatcherStats struct {
	ScannedTotal             int64 `json:"scanned_total"`
	DispatchStartedTotal     int64 `json:"dispatch_started_total"`
	AlreadyStartedTotal      int64 `json:"already_started_total"`
	DispatchErrorsTotal      int64 `json:"dispatch_errors_total"`
	DispatcherErrorsTotal    int64 `json:"dispatcher_errors_total"`
	DispatcherIdlePollsTotal int64 `json:"dispatcher_idle_polls_total"`
}

type TemporalDispatcher struct {
	store        store.Repository
	temporal     client.Client
	taskQueue    string
	pollInterval time.Duration
	batchSize    int
	logger       *slog.Logger

	scannedTotal             atomic.Int64
	dispatchStartedTotal     atomic.Int64
	alreadyStartedTotal      atomic.Int64
	dispatchErrorsTotal      atomic.Int64
	dispatcherErrorsTotal    atomic.Int64
	dispatcherIdlePollsTotal atomic.Int64
}

func NewTemporalDispatcher(repo store.Repository, temporalClient client.Client, taskQueue string, pollInterval time.Duration, batchSize int, loggers ...*slog.Logger) *TemporalDispatcher {
	if strings.TrimSpace(taskQueue) == "" {
		taskQueue = DefaultTemporalReplayTaskQueue
	}
	if pollInterval <= 0 {
		pollInterval = 750 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 25
	}
	if batchSize > 500 {
		batchSize = 500
	}
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &TemporalDispatcher{
		store:        repo,
		temporal:     temporalClient,
		taskQueue:    taskQueue,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		logger:       logger.With("component", "replay_temporal_dispatcher"),
	}
}

func (d *TemporalDispatcher) Run(ctx context.Context) {
	for {
		dispatched, err := d.dispatchOnce(ctx)
		if err != nil {
			d.dispatcherErrorsTotal.Add(1)
			d.logger.Error("dispatch loop error", "error", err)
		}
		if dispatched == 0 {
			d.dispatcherIdlePollsTotal.Add(1)
		}
		if !sleepWithContext(ctx, d.pollInterval) {
			return
		}
	}
}

func (d *TemporalDispatcher) Stats() TemporalDispatcherStats {
	return TemporalDispatcherStats{
		ScannedTotal:             d.scannedTotal.Load(),
		DispatchStartedTotal:     d.dispatchStartedTotal.Load(),
		AlreadyStartedTotal:      d.alreadyStartedTotal.Load(),
		DispatchErrorsTotal:      d.dispatchErrorsTotal.Load(),
		DispatcherErrorsTotal:    d.dispatcherErrorsTotal.Load(),
		DispatcherIdlePollsTotal: d.dispatcherIdlePollsTotal.Load(),
	}
}

func (d *TemporalDispatcher) dispatchOnce(ctx context.Context) (int, error) {
	jobs, err := d.store.ListQueuedReplayJobs(d.batchSize)
	if err != nil {
		return 0, err
	}
	if len(jobs) == 0 {
		return 0, nil
	}

	dispatched := 0
	for _, job := range jobs {
		d.scannedTotal.Add(1)
		if err := d.startWorkflow(ctx, job.TenantID, job.ID); err != nil {
			var alreadyStartedErr *serviceerror.WorkflowExecutionAlreadyStarted
			if errors.As(err, &alreadyStartedErr) {
				d.alreadyStartedTotal.Add(1)
				continue
			}
			d.dispatchErrorsTotal.Add(1)
			d.logger.Error("start workflow failed", "job_id", job.ID, "tenant_id", job.TenantID, "error", err)
			continue
		}
		dispatched++
		d.dispatchStartedTotal.Add(1)
	}

	return dispatched, nil
}

func (d *TemporalDispatcher) startWorkflow(ctx context.Context, tenantID, jobID string) error {
	tenantID = strings.TrimSpace(tenantID)
	jobID = strings.TrimSpace(jobID)
	if tenantID == "" || jobID == "" {
		return fmt.Errorf("tenant_id and job_id are required")
	}

	workflowID := ReplayWorkflowID(jobID)
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := d.temporal.ExecuteWorkflow(startCtx, client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             d.taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
	}, ReplayWorkflowName, ReplayJobWorkflowInput{
		TenantID: tenantID,
		JobID:    jobID,
	})
	return err
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
