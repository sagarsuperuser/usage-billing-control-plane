package main

import (
	"net/http"
	"testing"
)

func TestValidateAuthRuntimeConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     authRuntimeConfig
		wantErr bool
	}{
		{
			name: "prod requires secure session cookie",
			cfg: authRuntimeConfig{
				Environment:             "production",
				UISessionCookieSecure:   false,
				UISessionCookieSameSite: http.SameSiteLaxMode,
			},
			wantErr: true,
		},
		{
			name: "same-site none requires secure",
			cfg: authRuntimeConfig{
				Environment:               "local",
				UISessionCookieSecure:     false,
				UISessionCookieSameSite:   http.SameSiteNoneMode,
				UISessionCookieSameSiteID: "none",
			},
			wantErr: true,
		},
		{
			name: "staging secure config is valid",
			cfg: authRuntimeConfig{
				Environment:             "staging",
				UISessionCookieSecure:   true,
				UISessionCookieSameSite: http.SameSiteLaxMode,
			},
			wantErr: false,
		},
		{
			name: "local allows non-secure cookie",
			cfg: authRuntimeConfig{
				Environment:             "local",
				UISessionCookieSecure:   false,
				UISessionCookieSameSite: http.SameSiteLaxMode,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthRuntimeConfig(tc.cfg)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveRuntimeEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ENVIRONMENT", "prod")
	if got := resolveRuntimeEnvironment(); got != "staging" {
		t.Fatalf("expected APP_ENV to win, got %q", got)
	}

	t.Setenv("APP_ENV", "")
	t.Setenv("ENVIRONMENT", "prod")
	if got := resolveRuntimeEnvironment(); got != "prod" {
		t.Fatalf("expected ENVIRONMENT fallback, got %q", got)
	}

	t.Setenv("APP_ENV", "")
	t.Setenv("ENVIRONMENT", "")
	if got := resolveRuntimeEnvironment(); got != "local" {
		t.Fatalf("expected local default, got %q", got)
	}
}

func TestParseSameSiteMode(t *testing.T) {
	mode, label := parseSameSiteMode("none")
	if mode != http.SameSiteNoneMode || label != "none" {
		t.Fatalf("expected none mapping, got mode=%v label=%q", mode, label)
	}

	mode, label = parseSameSiteMode("invalid")
	if mode != http.SameSiteLaxMode || label != "lax" {
		t.Fatalf("expected lax fallback, got mode=%v label=%q", mode, label)
	}
}
