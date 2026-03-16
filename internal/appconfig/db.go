package appconfig

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func LoadDBConfigFromEnv() (DBConfig, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return DBConfig{}, fmt.Errorf("DATABASE_URL is required")
	}

	return DBConfig{
		URL:                 databaseURL,
		MaxOpenConns:        getIntEnv("DB_MAX_OPEN_CONNS", 20),
		MaxIdleConns:        getIntEnv("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime:     time.Duration(getIntEnv("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute,
		PingTimeout:         time.Duration(getIntEnv("DB_PING_TIMEOUT_SEC", 5)) * time.Second,
		QueryTimeout:        time.Duration(getIntEnv("DB_QUERY_TIMEOUT_MS", 5000)) * time.Millisecond,
		MigrationTimeout:    time.Duration(getIntEnv("DB_MIGRATION_TIMEOUT_SEC", 60)) * time.Second,
		RunMigrationsOnBoot: getBoolEnv("RUN_MIGRATIONS_ON_BOOT", false),
	}, nil
}

func OpenPostgres(cfg DBConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), cfg.PingTimeout)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
