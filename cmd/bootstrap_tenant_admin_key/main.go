package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type result struct {
	TenantID           string     `json:"tenant_id"`
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
	var (
		tenantID                string
		name                    string
		expiresAtRaw            string
		output                  string
		allowExistingActiveKeys bool
	)

	flag.StringVar(&tenantID, "tenant-id", strings.TrimSpace(os.Getenv("TENANT_ID")), "tenant id to bootstrap")
	flag.StringVar(&name, "name", strings.TrimSpace(os.Getenv("KEY_NAME")), "display name for the new admin key")
	flag.StringVar(&expiresAtRaw, "expires-at", strings.TrimSpace(os.Getenv("EXPIRES_AT")), "optional RFC3339 expiry timestamp")
	flag.StringVar(&output, "output", strings.TrimSpace(os.Getenv("OUTPUT")), "output format: json or text")
	flag.BoolVar(&allowExistingActiveKeys, "allow-existing-active-keys", envBool("ALLOW_EXISTING_ACTIVE_KEYS"), "allow bootstrap even when the tenant already has active keys")
	flag.Parse()

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		log.Fatal("TENANT_ID or -tenant-id is required")
	}
	if name == "" {
		name = "bootstrap-admin-" + tenantID
	}
	if output == "" {
		output = "json"
	}
	output = strings.ToLower(strings.TrimSpace(output))
	if output != "json" && output != "text" {
		log.Fatalf("unsupported output format %q", output)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	var expiresAt *time.Time
	if expiresAtRaw != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAtRaw)
		if err != nil {
			log.Fatalf("invalid EXPIRES_AT/-expires-at value %q: %v", expiresAtRaw, err)
		}
		parsed = parsed.UTC()
		expiresAt = &parsed
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo := store.NewPostgresStore(
		db,
		store.WithQueryTimeout(5*time.Second),
		store.WithMigrationTimeout(60*time.Second),
	)
	keyService := service.NewAPIKeyService(repo)

	activeKeys, err := keyService.ListAPIKeys(tenantID, service.ListAPIKeysRequest{
		State: "active",
		Limit: 1,
	})
	if err != nil {
		log.Fatalf("failed to inspect existing active keys for tenant %q: %v", tenantID, err)
	}
	if activeKeys.Total > 0 && !allowExistingActiveKeys {
		log.Fatalf("tenant %q already has %d active key(s); refusing to create another bootstrap admin key without -allow-existing-active-keys", tenantID, activeKeys.Total)
	}

	created, err := keyService.CreateAPIKey(tenantID, "", service.CreateAPIKeyRequest{
		Name:      name,
		Role:      "admin",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		log.Fatalf("failed to create tenant admin key for tenant %q: %v", tenantID, err)
	}

	res := result{
		TenantID:           created.APIKey.TenantID,
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
			log.Fatalf("failed to encode result: %v", err)
		}
	}
}

func printText(res result) {
	fmt.Printf("tenant_id=%s\n", res.TenantID)
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

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
