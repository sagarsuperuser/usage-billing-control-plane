package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"usage-billing-control-plane/migrations"
)

type command string

const (
	commandUp     command = "up"
	commandStatus command = "status"
	commandVerify command = "verify"
)

func main() {
	cmd, err := parseCommand(os.Args[1:])
	if err != nil {
		log.Fatalf("%v", err)
	}

	db, err := openDBFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}
	defer db.Close()

	migrationTimeout := time.Duration(getIntEnv("DB_MIGRATION_TIMEOUT_SEC", 60)) * time.Second
	runner := migrations.NewRunner(db, migrations.WithTimeout(migrationTimeout))

	switch cmd {
	case commandUp:
		runUp(runner)
	case commandStatus:
		runStatus(runner)
	case commandVerify:
		runVerify(runner)
	default:
		log.Fatalf("unsupported command: %s", cmd)
	}
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return commandUp, nil
	}

	raw := args[0]
	switch raw {
	case string(commandUp):
		return commandUp, nil
	case string(commandStatus):
		return commandStatus, nil
	case string(commandVerify):
		return commandVerify, nil
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	default:
		return "", fmt.Errorf("unknown command %q\n\n%s", raw, usageText())
	}
	return "", nil
}

func openDBFromEnv() (*sql.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(getIntEnv("DB_MAX_OPEN_CONNS", 20))
	db.SetMaxIdleConns(getIntEnv("DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(time.Duration(getIntEnv("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), time.Duration(getIntEnv("DB_PING_TIMEOUT_SEC", 5))*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func runUp(runner *migrations.Runner) {
	before, err := runner.Status(context.Background())
	if err != nil {
		log.Fatalf("failed to load migration status before run: %v", err)
	}

	started := time.Now().UTC()
	if err := runner.Run(context.Background()); err != nil {
		log.Fatalf("migration run failed: %v", err)
	}

	after, err := runner.Status(context.Background())
	if err != nil {
		log.Fatalf("failed to load migration status after run: %v", err)
	}

	appliedThisRun := before.PendingCount - after.PendingCount
	if appliedThisRun < 0 {
		appliedThisRun = 0
	}

	durationMs := time.Since(started).Milliseconds()
	log.Printf(
		"level=info component=migrate event=completed applied_this_run=%d %s duration_ms=%d",
		appliedThisRun,
		after.SummaryString(),
		durationMs,
	)
}

func runStatus(runner *migrations.Runner) {
	status, err := runner.Status(context.Background())
	if err != nil {
		log.Fatalf("failed to load migration status: %v", err)
	}

	fmt.Println(status.SummaryString())

	for _, pending := range status.Pending {
		fmt.Printf("PENDING version=%d name=%s\n", pending.Version, pending.Name)
	}
}

func runVerify(runner *migrations.Runner) {
	if err := runner.Verify(context.Background()); err != nil {
		log.Fatalf("verification failed: %v", err)
	}
	log.Printf("level=info component=migrate event=verified")
}

func usageText() string {
	return `Usage:
  go run ./cmd/migrate [command]

Commands:
  up       Apply pending migrations (default)
  status   Print migration status (available/applied/pending/current)
  verify   Exit non-zero if dirty, unknown-current, or pending migrations exist
  help     Show this help
`
}

func printUsage() {
	fmt.Print(usageText())
}

func getIntEnv(key string, defaultVal int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return parsed
}
