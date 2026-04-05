package service

import "time"

const (
	// billingOperationTimeout is the maximum time for Stripe/billing provider calls.
	billingOperationTimeout = 10 * time.Second

	// temporalStartTimeout is the maximum time to start a Temporal workflow.
	temporalStartTimeout = 5 * time.Second
)
