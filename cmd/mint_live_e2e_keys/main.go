package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"usage-billing-control-plane/internal/appconfig"
	"usage-billing-control-plane/internal/logging"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type mintedKey struct {
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	Secret    string     `json:"secret"`
	APIKeyID  string     `json:"api_key_id"`
	KeyPrefix string     `json:"key_prefix"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type result struct {
	TenantID           string    `json:"tenant_id"`
	TenantCreated      bool      `json:"tenant_created"`
	GeneratedAt        time.Time `json:"generated_at"`
	BaseURL            string    `json:"playwright_live_base_url,omitempty"`
	APIBaseURL         string    `json:"playwright_live_api_base_url,omitempty"`
	PlatformAPIKey     mintedKey `json:"platform_api_key"`
	TenantWriterAPIKey mintedKey `json:"tenant_writer_api_key"`
	TenantReaderAPIKey mintedKey `json:"tenant_reader_api_key"`
}

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())

	var (
		tenantID        string
		tenantName      string
		baseURL         string
		apiBaseURL      string
		output          string
		expiresAtRaw    string
		platformKeyName string
		writerKeyName   string
		readerKeyName   string
	)

	flag.StringVar(&tenantID, "tenant-id", "default", "target tenant id for reader/writer keys")
	flag.StringVar(&tenantName, "tenant-name", "", "optional tenant display name when ensuring the tenant")
	flag.StringVar(&baseURL, "base-url", strings.TrimSpace(os.Getenv("PLAYWRIGHT_LIVE_BASE_URL")), "optional PLAYWRIGHT_LIVE_BASE_URL value to include in output")
	flag.StringVar(&apiBaseURL, "api-base-url", strings.TrimSpace(os.Getenv("PLAYWRIGHT_LIVE_API_BASE_URL")), "optional PLAYWRIGHT_LIVE_API_BASE_URL value to include in output")
	flag.StringVar(&output, "output", "json", "output format: json, text, or shell")
	flag.StringVar(&expiresAtRaw, "expires-at", "", "optional RFC3339 expiry applied to all minted keys")
	flag.StringVar(&platformKeyName, "platform-key-name", "playwright-live-platform-admin", "display name for the minted platform admin key")
	flag.StringVar(&writerKeyName, "writer-key-name", "playwright-live-writer", "display name for the minted tenant writer key")
	flag.StringVar(&readerKeyName, "reader-key-name", "playwright-live-reader", "display name for the minted tenant reader key")
	flag.Parse()

	output = strings.ToLower(strings.TrimSpace(output))
	switch output {
	case "json", "text", "shell":
	default:
		fatal(logger, "unsupported output format", "output", output)
	}

	dbCfg, err := appconfig.LoadDBConfigFromEnv()
	if err != nil {
		fatal(logger, err.Error())
	}

	var expiresAt *time.Time
	if strings.TrimSpace(expiresAtRaw) != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAtRaw)
		if err != nil {
			fatal(logger, "invalid expires-at value", "value", expiresAtRaw, "error", err)
		}
		parsed = parsed.UTC()
		expiresAt = &parsed
	}

	db, err := appconfig.OpenPostgres(dbCfg)
	if err != nil {
		fatal(logger, "open database", "error", err)
	}
	defer db.Close()

	repo := store.NewPostgresStore(
		db,
		store.WithQueryTimeout(dbCfg.QueryTimeout),
		store.WithMigrationTimeout(dbCfg.MigrationTimeout),
	)
	tenantService := service.NewTenantService(repo)
	apiKeyService := service.NewAPIKeyService(repo)
	platformKeyService := service.NewPlatformAPIKeyService(repo)

	tenant, tenantCreated, err := tenantService.EnsureTenant(service.EnsureTenantRequest{
		ID:   tenantID,
		Name: tenantName,
	}, "")
	if err != nil {
		fatal(logger, "ensure tenant", "error", err)
	}

	platformCreated, err := platformKeyService.CreatePlatformAPIKey(service.CreatePlatformAPIKeyRequest{
		Name:      strings.TrimSpace(platformKeyName),
		Role:      "platform_admin",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		fatal(logger, "create platform api key", "error", err)
	}

	writerCreated, err := apiKeyService.CreateAPIKey(tenant.ID, "", service.CreateAPIKeyRequest{
		Name:      strings.TrimSpace(writerKeyName),
		Role:      "writer",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		fatal(logger, "create tenant writer api key", "error", err)
	}

	readerCreated, err := apiKeyService.CreateAPIKey(tenant.ID, "", service.CreateAPIKeyRequest{
		Name:      strings.TrimSpace(readerKeyName),
		Role:      "reader",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		fatal(logger, "create tenant reader api key", "error", err)
	}

	res := result{
		TenantID:      tenant.ID,
		TenantCreated: tenantCreated,
		GeneratedAt:   time.Now().UTC(),
		BaseURL:       strings.TrimSpace(baseURL),
		APIBaseURL:    strings.TrimSpace(apiBaseURL),
		PlatformAPIKey: mintedKey{
			Name:      platformCreated.APIKey.Name,
			Role:      platformCreated.APIKey.Role,
			Secret:    platformCreated.Secret,
			APIKeyID:  platformCreated.APIKey.ID,
			KeyPrefix: platformCreated.APIKey.KeyPrefix,
			ExpiresAt: platformCreated.APIKey.ExpiresAt,
		},
		TenantWriterAPIKey: mintedKey{
			Name:      writerCreated.APIKey.Name,
			Role:      writerCreated.APIKey.Role,
			Secret:    writerCreated.Secret,
			APIKeyID:  writerCreated.APIKey.ID,
			KeyPrefix: writerCreated.APIKey.KeyPrefix,
			ExpiresAt: writerCreated.APIKey.ExpiresAt,
		},
		TenantReaderAPIKey: mintedKey{
			Name:      readerCreated.APIKey.Name,
			Role:      readerCreated.APIKey.Role,
			Secret:    readerCreated.Secret,
			APIKeyID:  readerCreated.APIKey.ID,
			KeyPrefix: readerCreated.APIKey.KeyPrefix,
			ExpiresAt: readerCreated.APIKey.ExpiresAt,
		},
	}

	switch output {
	case "text":
		printText(res)
	case "shell":
		printShell(res)
	default:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(res); err != nil {
			fatal(logger, "encode result", "error", err)
		}
	}
}

func printText(res result) {
	fmt.Printf("tenant_id=%s\n", res.TenantID)
	fmt.Printf("tenant_created=%t\n", res.TenantCreated)
	fmt.Printf("generated_at=%s\n", res.GeneratedAt.Format(time.RFC3339))
	if res.BaseURL != "" {
		fmt.Printf("playwright_live_base_url=%s\n", res.BaseURL)
	}
	if res.APIBaseURL != "" {
		fmt.Printf("playwright_live_api_base_url=%s\n", res.APIBaseURL)
	}
	fmt.Printf("platform_api_key_secret=%s\n", res.PlatformAPIKey.Secret)
	fmt.Printf("tenant_writer_api_key_secret=%s\n", res.TenantWriterAPIKey.Secret)
	fmt.Printf("tenant_reader_api_key_secret=%s\n", res.TenantReaderAPIKey.Secret)
}

func printShell(res result) {
	if res.BaseURL != "" {
		fmt.Printf("export PLAYWRIGHT_LIVE_BASE_URL=%q\n", res.BaseURL)
	}
	if res.APIBaseURL != "" {
		fmt.Printf("export PLAYWRIGHT_LIVE_API_BASE_URL=%q\n", res.APIBaseURL)
	}
	fmt.Printf("export PLAYWRIGHT_LIVE_PLATFORM_API_KEY=%q\n", res.PlatformAPIKey.Secret)
	fmt.Printf("export PLAYWRIGHT_LIVE_WRITER_API_KEY=%q\n", res.TenantWriterAPIKey.Secret)
	fmt.Printf("export PLAYWRIGHT_LIVE_READER_API_KEY=%q\n", res.TenantReaderAPIKey.Secret)
	fmt.Printf("export PLAYWRIGHT_LIVE_TENANT_ID=%q\n", res.TenantID)
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
