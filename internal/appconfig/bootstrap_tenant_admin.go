package appconfig

import (
	"os"
	"strings"
)

type BootstrapTenantAdminConfig struct {
	TenantID                string
	KeyName                 string
	ExpiresAt               string
	Output                  string
	AllowExistingActiveKeys bool
}

func LoadBootstrapTenantAdminConfigFromEnv() BootstrapTenantAdminConfig {
	return BootstrapTenantAdminConfig{
		TenantID:                strings.TrimSpace(os.Getenv("TENANT_ID")),
		KeyName:                 strings.TrimSpace(os.Getenv("KEY_NAME")),
		ExpiresAt:               strings.TrimSpace(os.Getenv("EXPIRES_AT")),
		Output:                  strings.TrimSpace(os.Getenv("OUTPUT")),
		AllowExistingActiveKeys: getBoolEnv("ALLOW_EXISTING_ACTIVE_KEYS", false),
	}
}
