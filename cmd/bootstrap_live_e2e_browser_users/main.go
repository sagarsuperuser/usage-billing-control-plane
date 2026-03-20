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
	"usage-billing-control-plane/internal/domain"
	"usage-billing-control-plane/internal/logging"
	"usage-billing-control-plane/internal/service"
	"usage-billing-control-plane/internal/store"
)

type browserUser struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password,omitempty"`
	Scope       string `json:"scope"`
	Role        string `json:"role,omitempty"`
}

type result struct {
	TenantID      string      `json:"tenant_id"`
	TenantCreated bool        `json:"tenant_created"`
	GeneratedAt   time.Time   `json:"generated_at"`
	BaseURL       string      `json:"playwright_live_base_url,omitempty"`
	APIBaseURL    string      `json:"playwright_live_api_base_url,omitempty"`
	PlatformUser  browserUser `json:"platform_user"`
	TenantWriter  browserUser `json:"tenant_writer_user"`
	TenantReader  browserUser `json:"tenant_reader_user"`
}

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())

	var (
		tenantID            string
		tenantName          string
		baseURL             string
		apiBaseURL          string
		output              string
		platformEmail       string
		platformDisplayName string
		platformPassword    string
		writerEmail         string
		writerDisplayName   string
		writerPassword      string
		readerEmail         string
		readerDisplayName   string
		readerPassword      string
	)

	flag.StringVar(&tenantID, "tenant-id", "default", "target tenant id for writer and reader browser users")
	flag.StringVar(&tenantName, "tenant-name", "", "optional tenant display name when ensuring the tenant")
	flag.StringVar(&baseURL, "base-url", strings.TrimSpace(os.Getenv("PLAYWRIGHT_LIVE_BASE_URL")), "optional PLAYWRIGHT_LIVE_BASE_URL value to include in output")
	flag.StringVar(&apiBaseURL, "api-base-url", strings.TrimSpace(os.Getenv("PLAYWRIGHT_LIVE_API_BASE_URL")), "optional PLAYWRIGHT_LIVE_API_BASE_URL value to include in output")
	flag.StringVar(&output, "output", "json", "output format: json, text, or shell")
	flag.StringVar(&platformEmail, "platform-email", "playwright-live-platform-admin@alpha.test", "email for the live platform admin browser user")
	flag.StringVar(&platformDisplayName, "platform-display-name", "Playwright Live Platform Admin", "display name for the live platform admin browser user")
	flag.StringVar(&platformPassword, "platform-password", "playwright-live-platform-password", "password for the live platform admin browser user")
	flag.StringVar(&writerEmail, "writer-email", "playwright-live-writer@alpha.test", "email for the live tenant writer browser user")
	flag.StringVar(&writerDisplayName, "writer-display-name", "Playwright Live Tenant Writer", "display name for the live tenant writer browser user")
	flag.StringVar(&writerPassword, "writer-password", "playwright-live-writer-password", "password for the live tenant writer browser user")
	flag.StringVar(&readerEmail, "reader-email", "playwright-live-reader@alpha.test", "email for the live tenant reader browser user")
	flag.StringVar(&readerDisplayName, "reader-display-name", "Playwright Live Tenant Reader", "display name for the live tenant reader browser user")
	flag.StringVar(&readerPassword, "reader-password", "playwright-live-reader-password", "password for the live tenant reader browser user")
	flag.Parse()

	output = strings.ToLower(strings.TrimSpace(output))
	switch output {
	case "json", "text", "shell":
	default:
		fatal(logger, "unsupported output format", "output", output)
	}

	platformEmail = normalizeRequired(platformEmail, "platform-email", logger)
	platformDisplayName = normalizeRequired(platformDisplayName, "platform-display-name", logger)
	platformPassword = normalizeRequired(platformPassword, "platform-password", logger)
	writerEmail = normalizeRequired(writerEmail, "writer-email", logger)
	writerDisplayName = normalizeRequired(writerDisplayName, "writer-display-name", logger)
	writerPassword = normalizeRequired(writerPassword, "writer-password", logger)
	readerEmail = normalizeRequired(readerEmail, "reader-email", logger)
	readerDisplayName = normalizeRequired(readerDisplayName, "reader-display-name", logger)
	readerPassword = normalizeRequired(readerPassword, "reader-password", logger)

	dbCfg, err := appconfig.LoadDBConfigFromEnv()
	if err != nil {
		fatal(logger, err.Error())
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

	tenant, tenantCreated, err := tenantService.EnsureTenant(service.EnsureTenantRequest{
		ID:   tenantID,
		Name: tenantName,
	}, "")
	if err != nil {
		fatal(logger, "ensure tenant", "error", err)
	}

	platformUser, err := ensureBrowserUser(repo, ensureBrowserUserInput{
		email:       platformEmail,
		displayName: platformDisplayName,
		password:    platformPassword,
		platform:    true,
	})
	if err != nil {
		fatal(logger, "ensure platform browser user", "error", err)
	}

	writerUser, err := ensureBrowserUser(repo, ensureBrowserUserInput{
		email:       writerEmail,
		displayName: writerDisplayName,
		password:    writerPassword,
		tenantID:    tenant.ID,
		tenantRole:  "writer",
	})
	if err != nil {
		fatal(logger, "ensure tenant writer browser user", "error", err)
	}

	readerUser, err := ensureBrowserUser(repo, ensureBrowserUserInput{
		email:       readerEmail,
		displayName: readerDisplayName,
		password:    readerPassword,
		tenantID:    tenant.ID,
		tenantRole:  "reader",
	})
	if err != nil {
		fatal(logger, "ensure tenant reader browser user", "error", err)
	}

	res := result{
		TenantID:      tenant.ID,
		TenantCreated: tenantCreated,
		GeneratedAt:   time.Now().UTC(),
		BaseURL:       strings.TrimSpace(baseURL),
		APIBaseURL:    strings.TrimSpace(apiBaseURL),
		PlatformUser: browserUser{
			Email:       platformUser.Email,
			DisplayName: platformUser.DisplayName,
			Password:    platformPassword,
			Scope:       "platform",
			Role:        string(platformUser.PlatformRole),
		},
		TenantWriter: browserUser{
			Email:       writerUser.Email,
			DisplayName: writerUser.DisplayName,
			Password:    writerPassword,
			Scope:       "tenant",
			Role:        "writer",
		},
		TenantReader: browserUser{
			Email:       readerUser.Email,
			DisplayName: readerUser.DisplayName,
			Password:    readerPassword,
			Scope:       "tenant",
			Role:        "reader",
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

type ensureBrowserUserInput struct {
	email       string
	displayName string
	password    string
	platform    bool
	tenantID    string
	tenantRole  string
}

func ensureBrowserUser(repo *store.PostgresStore, input ensureBrowserUserInput) (domain.User, error) {
	user, err := repo.GetUserByEmail(input.email)
	if err != nil {
		if err != store.ErrNotFound {
			return domain.User{}, err
		}
		user, err = repo.CreateUser(domain.User{
			Email:       input.email,
			DisplayName: input.displayName,
			Status:      domain.UserStatusActive,
			PlatformRole: func() domain.UserPlatformRole {
				if input.platform {
					return domain.UserPlatformRoleAdmin
				}
				return ""
			}(),
		})
		if err != nil {
			return domain.User{}, err
		}
	} else {
		desiredRole := domain.UserPlatformRole("")
		if input.platform {
			desiredRole = domain.UserPlatformRoleAdmin
		}
		if user.DisplayName != input.displayName || user.Status != domain.UserStatusActive || user.PlatformRole != desiredRole {
			user.DisplayName = input.displayName
			user.Status = domain.UserStatusActive
			user.PlatformRole = desiredRole
			user, err = repo.UpdateUser(user)
			if err != nil {
				return domain.User{}, err
			}
		}
	}

	hash, err := service.HashPassword(input.password)
	if err != nil {
		return domain.User{}, err
	}
	if _, err := repo.UpsertUserPasswordCredential(domain.UserPasswordCredential{
		UserID:            user.ID,
		PasswordHash:      hash,
		PasswordUpdatedAt: time.Now().UTC(),
	}); err != nil {
		return domain.User{}, err
	}

	if strings.TrimSpace(input.tenantID) != "" && strings.TrimSpace(input.tenantRole) != "" {
		if _, err := repo.UpsertUserTenantMembership(domain.UserTenantMembership{
			UserID:   user.ID,
			TenantID: input.tenantID,
			Role:     input.tenantRole,
			Status:   domain.UserTenantMembershipStatusActive,
		}); err != nil {
			return domain.User{}, err
		}
	}

	return user, nil
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
	fmt.Printf("platform_email=%s\n", res.PlatformUser.Email)
	fmt.Printf("tenant_writer_email=%s\n", res.TenantWriter.Email)
	fmt.Printf("tenant_reader_email=%s\n", res.TenantReader.Email)
}

func printShell(res result) {
	if res.BaseURL != "" {
		fmt.Printf("export PLAYWRIGHT_LIVE_BASE_URL=%q\n", res.BaseURL)
	}
	if res.APIBaseURL != "" {
		fmt.Printf("export PLAYWRIGHT_LIVE_API_BASE_URL=%q\n", res.APIBaseURL)
	}
	fmt.Printf("export PLAYWRIGHT_LIVE_PLATFORM_EMAIL=%q\n", res.PlatformUser.Email)
	fmt.Printf("export PLAYWRIGHT_LIVE_PLATFORM_PASSWORD=%q\n", res.PlatformUser.Password)
	fmt.Printf("export PLAYWRIGHT_LIVE_WRITER_EMAIL=%q\n", res.TenantWriter.Email)
	fmt.Printf("export PLAYWRIGHT_LIVE_WRITER_PASSWORD=%q\n", res.TenantWriter.Password)
	fmt.Printf("export PLAYWRIGHT_LIVE_READER_EMAIL=%q\n", res.TenantReader.Email)
	fmt.Printf("export PLAYWRIGHT_LIVE_READER_PASSWORD=%q\n", res.TenantReader.Password)
	fmt.Printf("export PLAYWRIGHT_LIVE_TENANT_ID=%q\n", res.TenantID)
	// Preserve compatibility for API-only scripts that still use the key mint target.
}

func normalizeRequired(value, name string, logger *slog.Logger) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		fatal(logger, name+" is required")
	}
	return trimmed
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
