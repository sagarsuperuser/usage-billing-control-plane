package appconfig

import (
	"os"
	"strings"
)

type BootstrapPlatformAdminConfig struct {
	KeyName                 string
	ExpiresAt               string
	Output                  string
	AllowExistingActiveKeys bool
}

func LoadBootstrapPlatformAdminConfigFromEnv() BootstrapPlatformAdminConfig {
	keyName := strings.TrimSpace(os.Getenv("PLATFORM_KEY_NAME"))
	if keyName == "" {
		keyName = strings.TrimSpace(os.Getenv("KEY_NAME"))
	}

	return BootstrapPlatformAdminConfig{
		KeyName:                 keyName,
		ExpiresAt:               strings.TrimSpace(os.Getenv("EXPIRES_AT")),
		Output:                  strings.TrimSpace(os.Getenv("OUTPUT")),
		AllowExistingActiveKeys: getBoolEnv("ALLOW_EXISTING_ACTIVE_KEYS", false),
	}
}
