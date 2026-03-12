package temporalutil

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/api/serviceerror"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	temporalclient "go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/durationpb"
)

// EnsureNamespaceReady verifies that a Temporal namespace exists and is queryable.
// If the namespace does not exist, it is created and then polled until stable.
func EnsureNamespaceReady(ctx context.Context, temporalClient temporalclient.Client, namespace string, retention time.Duration) error {
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	if retention <= 0 {
		retention = 24 * time.Hour
	}

	serviceClient := temporalClient.WorkflowService()
	stableDescribeSuccesses := 0

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return fmt.Errorf("timed out waiting for temporal namespace %q: %w", namespace, ctx.Err())
		}

		_, describeErr := serviceClient.DescribeNamespace(ctx, &workflowservice.DescribeNamespaceRequest{
			Namespace: namespace,
		})
		if describeErr == nil {
			stableDescribeSuccesses++
			if stableDescribeSuccesses >= 2 {
				return nil
			}
			if err := sleepWithContext(ctx, 200*time.Millisecond); err != nil {
				return fmt.Errorf("timed out waiting for temporal namespace %q: %w", namespace, err)
			}
			continue
		}

		stableDescribeSuccesses = 0
		var namespaceNotFound *serviceerror.NamespaceNotFound
		if errors.As(describeErr, &namespaceNotFound) {
			_, registerErr := serviceClient.RegisterNamespace(ctx, &workflowservice.RegisterNamespaceRequest{
				Namespace:                        namespace,
				Description:                      "lago-usage-billing-alpha integration namespace",
				OwnerEmail:                       "integration-tests@local",
				WorkflowExecutionRetentionPeriod: durationpb.New(retention),
			})
			if registerErr == nil {
				continue
			}
			var alreadyExists *serviceerror.NamespaceAlreadyExists
			if errors.As(registerErr, &alreadyExists) {
				continue
			}
			describeErr = registerErr
		}

		backoff := 200 * time.Millisecond
		if attempt > 0 {
			backoff = time.Duration(attempt) * 200 * time.Millisecond
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
		}
		if err := sleepWithContext(ctx, backoff); err != nil {
			return fmt.Errorf("timed out waiting for temporal namespace %q after error %v: %w", namespace, describeErr, err)
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
