package store

import (
	"testing"

	"usage-billing-control-plane/internal/domain"
)

func TestNormalizeDunningEventTypePreservesRetryEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   domain.DunningEventType
		want domain.DunningEventType
	}{
		{name: "retry attempted", in: domain.DunningEventTypeRetryAttempted, want: domain.DunningEventTypeRetryAttempted},
		{name: "retry succeeded", in: domain.DunningEventTypeRetrySucceeded, want: domain.DunningEventTypeRetrySucceeded},
		{name: "retry failed", in: domain.DunningEventTypeRetryFailed, want: domain.DunningEventTypeRetryFailed},
		{name: "retry scheduled", in: domain.DunningEventTypeRetryScheduled, want: domain.DunningEventTypeRetryScheduled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeDunningEventType(tc.in); got != tc.want {
				t.Fatalf("normalizeDunningEventType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
