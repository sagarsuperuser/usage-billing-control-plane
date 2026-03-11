package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type MigrationDescriptor struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

type AppliedMigration struct {
	Version   string    `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

type StatusReport struct {
	Available      []MigrationDescriptor `json:"available"`
	Applied        []AppliedMigration    `json:"applied"`
	Pending        []MigrationDescriptor `json:"pending"`
	UnknownApplied []AppliedMigration    `json:"unknown_applied"`
}

func GetStatus(ctx context.Context, db *sql.DB) (StatusReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return StatusReport{}, err
	}

	entries, err := loadMigrationEntries()
	if err != nil {
		return StatusReport{}, err
	}

	available := make([]MigrationDescriptor, 0, len(entries))
	availableByVersion := make(map[string]MigrationDescriptor, len(entries))
	for _, entry := range entries {
		desc := MigrationDescriptor{Version: entry.Version, Name: entry.Name}
		available = append(available, desc)
		availableByVersion[entry.Version] = desc
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		return StatusReport{}, err
	}

	appliedVersionSet := make(map[string]struct{}, len(applied))
	for _, item := range applied {
		appliedVersionSet[item.Version] = struct{}{}
	}

	pending := make([]MigrationDescriptor, 0)
	for _, item := range available {
		if _, ok := appliedVersionSet[item.Version]; !ok {
			pending = append(pending, item)
		}
	}

	unknown := make([]AppliedMigration, 0)
	for _, item := range applied {
		if _, ok := availableByVersion[item.Version]; !ok {
			unknown = append(unknown, item)
		}
	}

	sort.Slice(unknown, func(i, j int) bool {
		if unknown[i].Version == unknown[j].Version {
			return unknown[i].AppliedAt.Before(unknown[j].AppliedAt)
		}
		return unknown[i].Version < unknown[j].Version
	})

	return StatusReport{
		Available:      available,
		Applied:        applied,
		Pending:        pending,
		UnknownApplied: unknown,
	}, nil
}

func Verify(ctx context.Context, db *sql.DB) error {
	status, err := GetStatus(ctx, db)
	if err != nil {
		return err
	}

	if len(status.Pending) == 0 && len(status.UnknownApplied) == 0 {
		return nil
	}

	parts := make([]string, 0, 2)
	if len(status.Pending) > 0 {
		parts = append(parts, fmt.Sprintf("pending=%d", len(status.Pending)))
	}
	if len(status.UnknownApplied) > 0 {
		parts = append(parts, fmt.Sprintf("unknown_applied=%d", len(status.UnknownApplied)))
	}

	return fmt.Errorf("migration verification failed: %s", strings.Join(parts, " "))
}

func loadAppliedMigrations(ctx context.Context, db queryContexter) ([]AppliedMigration, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, name, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	out := make([]AppliedMigration, 0)
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Name, &item.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		item.AppliedAt = item.AppliedAt.UTC()
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return out, nil
}
