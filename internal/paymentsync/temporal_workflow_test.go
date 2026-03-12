package paymentsync

import (
	"context"
	"strings"
	"testing"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestPaymentReconcileWorkflowRejectsInvalidInput(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(PaymentReconcileWorkflow, workflow.RegisterOptions{Name: PaymentReconcileWorkflowName})

	env.ExecuteWorkflow(PaymentReconcileWorkflowName, PaymentReconcileWorkflowInput{
		Limit:             0,
		StaleAfterSeconds: 300,
	})

	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if env.GetWorkflowError() == nil {
		t.Fatalf("expected validation error")
	}
	if got := strings.ToLower(env.GetWorkflowError().Error()); !strings.Contains(got, "limit must be > 0") {
		t.Fatalf("expected missing limit error, got: %v", env.GetWorkflowError())
	}
}

func TestPaymentReconcileWorkflowExecutesActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(PaymentReconcileWorkflow, workflow.RegisterOptions{Name: PaymentReconcileWorkflowName})

	var called bool
	env.RegisterActivityWithOptions(func(_ context.Context, input PaymentReconcileWorkflowInput) error {
		called = true
		if input.Limit != 50 {
			t.Fatalf("expected limit 50, got %d", input.Limit)
		}
		if input.StaleAfterSeconds != 600 {
			t.Fatalf("expected stale_after_seconds 600, got %d", input.StaleAfterSeconds)
		}
		return nil
	}, activity.RegisterOptions{Name: PaymentReconcileRunActivityName})

	env.ExecuteWorkflow(PaymentReconcileWorkflowName, PaymentReconcileWorkflowInput{
		Limit:             50,
		StaleAfterSeconds: 600,
	})

	if !called {
		t.Fatalf("expected activity execution")
	}
	if !env.IsWorkflowCompleted() {
		t.Fatalf("expected workflow to complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("expected workflow success, got %v", err)
	}
}
