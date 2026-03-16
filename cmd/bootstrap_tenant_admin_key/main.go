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

type result struct {
	TenantID           string     `json:"tenant_id"`
	TenantName         string     `json:"tenant_name"`
	TenantStatus       string     `json:"tenant_status"`
	TenantCreated      bool       `json:"tenant_created"`
	Name               string     `json:"name"`
	Role               string     `json:"role"`
	Secret             string     `json:"secret"`
	APIKeyID           string     `json:"api_key_id"`
	KeyPrefix          string     `json:"key_prefix"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	ExistingActiveKeys int        `json:"existing_active_keys"`
}

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())
	envCfg := appconfig.LoadBootstrapTenantAdminConfigFromEnv()

	var (
		tenantID                string
		tenantName              string
		name                    string
		expiresAtRaw            string
		output                  string
		allowExistingActiveKeys bool
	)

	flag.StringVar(&tenantID, "tenant-id", envCfg.TenantID, "tenant id to bootstrap")
	flag.StringVar(&tenantName, "tenant-name", envCfg.TenantName, "display name for the tenant")
	flag.StringVar(&name, "name", envCfg.KeyName, "display name for the new admin key")
	flag.StringVar(&expiresAtRaw, "expires-at", envCfg.ExpiresAt, "optional RFC3339 expiry timestamp")
	flag.StringVar(&output, "output", envCfg.Output, "output format: json or text")
	flag.BoolVar(&allowExistingActiveKeys, "allow-existing-active-keys", envCfg.AllowExistingActiveKeys, "allow bootstrap even when the tenant already has active keys")
	flag.Parse()

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		fatal(logger, "TENANT_ID or -tenant-id is required")
	}
	if name == "" {
		name = "bootstrap-admin-" + tenantID
	}
	if output == "" {
		output = "json"
	}
	output = strings.ToLower(strings.TrimSpace(output))
	if output != "json" && output != "text" {
		fatal(logger, "unsupported output format", "output", output)
	}

	dbCfg, err := appconfig.LoadDBConfigFromEnv()
	if err != nil {
		fatal(logger, err.Error())
	}

	var expiresAt *time.Time
	if expiresAtRaw != "" {
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
	keyService := service.NewAPIKeyService(repo)

	tenant, tenantCreated, err := tenantService.EnsureTenant(service.EnsureTenantRequest{
		ID:   tenantID,
		Name: tenantName,
	})
	if err != nil {
		fatal(logger, "ensure tenant", "tenant_id", tenantID, "error", err)
	}

	activeKeys, err := keyService.ListAPIKeys(tenantID, service.ListAPIKeysRequest{
		State: "active",
		Limit: 1,
	})
	if err != nil {
		fatal(logger, "inspect existing active keys", "tenant_id", tenantID, "error", err)
	}
	if activeKeys.Total > 0 && !allowExistingActiveKeys {
		fatal(logger, "tenant already has active keys; refusing bootstrap", "tenant_id", tenantID, "active_keys", activeKeys.Total)
	}

	created, err := keyService.CreateAPIKey(tenantID, "", service.CreateAPIKeyRequest{
		Name:      name,
		Role:      "admin",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		fatal(logger, "create tenant admin key", "tenant_id", tenantID, "error", err)
	}

	res := result{
		TenantID:           created.APIKey.TenantID,
		TenantName:         tenant.Name,
		TenantStatus:       string(tenant.Status),
		TenantCreated:      tenantCreated,
		Name:               created.APIKey.Name,
		Role:               created.APIKey.Role,
		Secret:             created.Secret,
		APIKeyID:           created.APIKey.ID,
		KeyPrefix:          created.APIKey.KeyPrefix,
		CreatedAt:          created.APIKey.CreatedAt,
		ExpiresAt:          created.APIKey.ExpiresAt,
		ExistingActiveKeys: activeKeys.Total,
	}

	switch output {
	case "text":
		printText(res)
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
	fmt.Printf("tenant_name=%s\n", res.TenantName)
	fmt.Printf("tenant_status=%s\n", res.TenantStatus)
	fmt.Printf("tenant_created=%t\n", res.TenantCreated)
	fmt.Printf("name=%s\n", res.Name)
	fmt.Printf("role=%s\n", res.Role)
	fmt.Printf("api_key_id=%s\n", res.APIKeyID)
	fmt.Printf("key_prefix=%s\n", res.KeyPrefix)
	fmt.Printf("created_at=%s\n", res.CreatedAt.Format(time.RFC3339))
	if res.ExpiresAt != nil {
		fmt.Printf("expires_at=%s\n", res.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Printf("existing_active_keys=%d\n", res.ExistingActiveKeys)
	fmt.Printf("secret=%s\n", res.Secret)
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
