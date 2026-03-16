package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

type authRuntimeConfig struct {
	Environment               string
	UISessionCookieSecure     bool
	UISessionCookieSameSite   http.SameSite
	UISessionCookieSameSiteID string
}

func validateAuthRuntimeConfig(cfg authRuntimeConfig) error {
	if isProductionLikeEnvironment(cfg.Environment) && !cfg.UISessionCookieSecure {
		return fmt.Errorf("UI_SESSION_COOKIE_SECURE must be true in %s", cfg.Environment)
	}
	if cfg.UISessionCookieSameSite == http.SameSiteNoneMode && !cfg.UISessionCookieSecure {
		return fmt.Errorf("UI_SESSION_COOKIE_SAMESITE=%s requires UI_SESSION_COOKIE_SECURE=true", cfg.UISessionCookieSameSiteID)
	}
	return nil
}

func resolveRuntimeEnvironment() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("APP_ENV")),
		strings.TrimSpace(os.Getenv("ENVIRONMENT")),
	}
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" {
			return candidate
		}
	}
	return "local"
}

func isProductionLikeEnvironment(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

func parseSameSiteMode(raw string) (http.SameSite, string) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode, "strict"
	case "none":
		return http.SameSiteNoneMode, "none"
	case "lax", "":
		return http.SameSiteLaxMode, "lax"
	default:
		return http.SameSiteLaxMode, "lax"
	}
}
