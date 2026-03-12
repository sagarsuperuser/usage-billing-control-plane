package replay

import (
	"context"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestReplayJobWorkflowRejectsMissingJobID(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(ReplayJobWorkflow, workflow.RegisterOptions{Name: ReplayWorkflowName})

	env.ExecuteWorkflow(ReplayWorkflowName, ReplayJobWorkflowInput{
		TenantID: "tenant_alpha",
		JobID:    "",
	})

	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatalf("expected workflow validation error")
	}
	if got := strings.ToLower(env.GetWorkflowError().Error()); !strings.Contains(got, "job_id is required") {
		t.Fatalf("expected missing job_id error, got: %v", env.GetWorkflowError())
	}
}

func TestReplayJobWorkflowExecutesActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(ReplayJobWorkflow, workflow.RegisterOptions{Name: ReplayWorkflowName})

	var called bool
	env.RegisterActivityWithOptions(func(_ context.Context, input ReplayJobWorkflowInput) error {
		called = true
		if input.TenantID != "tenant_alpha" {
			t.Fatalf("expected tenant_alpha, got %q", input.TenantID)
		}
		if input.JobID != "rpl_123" {
			t.Fatalf("expected rpl_123, got %q", input.JobID)
		}
		return nil
	}, activity.RegisterOptions{Name: ReplayRunActivityName})

	env.ExecuteWorkflow(ReplayWorkflowName, ReplayJobWorkflowInput{
		TenantID: "tenant_alpha",
		JobID:    "rpl_123",
	})

	if !called {
		t.Fatalf("expected replay activity to be called")
	}
	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("expected workflow success, got: %v", err)
	}
}
