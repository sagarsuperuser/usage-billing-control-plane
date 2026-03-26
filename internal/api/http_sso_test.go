package api

import "testing"

func TestInvitationTokenFromNextPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nextPath string
		want     string
	}{
		{name: "invite route", nextPath: "/invite/token-123", want: "token-123"},
		{name: "invite route trims spaces", nextPath: " /invite/token-123 ", want: "token-123"},
		{name: "invite route rejects nested action", nextPath: "/invite/token-123/accept", want: ""},
		{name: "ui invitation route", nextPath: "/v1/ui/invitations/token-456", want: "token-456"},
		{name: "ui invitation accept route", nextPath: "/v1/ui/invitations/token-456/accept", want: "token-456"},
		{name: "ui invitation invalid action", nextPath: "/v1/ui/invitations/token-456/nope", want: ""},
		{name: "other path", nextPath: "/customers", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := invitationTokenFromNextPath(tc.nextPath); got != tc.want {
				t.Fatalf("invitationTokenFromNextPath(%q) = %q, want %q", tc.nextPath, got, tc.want)
			}
		})
	}
}
