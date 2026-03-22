package dunningflow

import (
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"usage-billing-control-plane/internal/service"
)

func TestCollectPaymentReminderWorkflowValidatesInput(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(CollectPaymentReminderWorkflow, workflow.RegisterOptions{Name: DunningCollectPaymentWorkflowName})

	env.ExecuteWorkflow(CollectPaymentReminderWorkflow, CollectPaymentReminderWorkflowInput{})
	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatalf("expected workflow validation error")
	}
}

func TestCollectPaymentReminderWorkflowExecutesActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(CollectPaymentReminderWorkflow, workflow.RegisterOptions{Name: DunningCollectPaymentWorkflowName})

	called := false
	env.RegisterActivityWithOptions(func(input CollectPaymentReminderWorkflowInput) (service.DunningCollectPaymentReminderBatchResult, error) {
		called = true
		if input.TenantID != "tenant_a" {
			t.Fatalf("expected tenant_a, got %q", input.TenantID)
		}
		if input.Limit != 5 {
			t.Fatalf("expected limit=5, got %d", input.Limit)
		}
		return service.DunningCollectPaymentReminderBatchResult{TenantID: input.TenantID, Limit: input.Limit, Processed: 2, Dispatched: 2}, nil
	}, activity.RegisterOptions{Name: DunningCollectPaymentRunActivityName})

	env.ExecuteWorkflow(CollectPaymentReminderWorkflow, CollectPaymentReminderWorkflowInput{TenantID: "tenant_a", Limit: 5})
	if !called {
		t.Fatalf("expected dunning activity to be called")
	}
	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("expected workflow success, got: %v", err)
	}
}
