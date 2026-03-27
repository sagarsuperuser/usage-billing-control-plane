package appconfig

import "testing"

func TestLoadDBConfigFromEnvPrefersDiscreteDBEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://stale:stale@stale-db:5432/stale?sslmode=disable")
	t.Setenv("DB_HOST", "fresh-db.example.com")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_NAME", "fresh")
	t.Setenv("DB_USER", "fresh_user")
	t.Setenv("DB_PASSWORD", "fresh_pass")
	t.Setenv("DB_SSLMODE", "verify-full")

	cfg, err := LoadDBConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadDBConfigFromEnv() error = %v", err)
	}

	want := "postgres://fresh_user:fresh_pass@fresh-db.example.com:6543/fresh?sslmode=verify-full"
	if cfg.URL != want {
		t.Fatalf("cfg.URL = %q, want %q", cfg.URL, want)
	}
}

func TestLoadDBConfigFromEnvFallsBackToDatabaseURL(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db.example.com:5432/app?sslmode=require")

	cfg, err := LoadDBConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadDBConfigFromEnv() error = %v", err)
	}

	if cfg.URL != "postgres://user:pass@db.example.com:5432/app?sslmode=require" {
		t.Fatalf("cfg.URL = %q", cfg.URL)
	}
}

func TestLoadDBConfigFromEnvRequiresCompleteDiscreteDBEnv(t *testing.T) {
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_NAME", "app")
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DATABASE_URL", "")

	_, err := LoadDBConfigFromEnv()
	if err == nil {
		t.Fatal("LoadDBConfigFromEnv() error = nil, want error")
	}
}
