package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"usage-billing-control-plane/internal/appconfig"
	"usage-billing-control-plane/internal/logging"
	"usage-billing-control-plane/migrations"
)

type command string

const (
	commandUp     command = "up"
	commandStatus command = "status"
	commandVerify command = "verify"
)

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())

	cmd, err := parseCommand(os.Args[1:])
	if err != nil {
		fatal(logger, err.Error())
	}

	dbCfg, err := appconfig.LoadDBConfigFromEnv()
	if err != nil {
		fatal(logger, err.Error())
	}

	db, err := appconfig.OpenPostgres(dbCfg)
	if err != nil {
		fatal(logger, "open database", "error", err)
	}
	defer db.Close()

	runner := migrations.NewRunner(db, migrations.WithTimeout(dbCfg.MigrationTimeout))

	switch cmd {
	case commandUp:
		runUp(logger, runner)
	case commandStatus:
		runStatus(logger, runner)
	case commandVerify:
		runVerify(logger, runner)
	default:
		fatal(logger, "unsupported command", "command", cmd)
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

func runUp(logger *slog.Logger, runner *migrations.Runner) {
	before, err := runner.Status(context.Background())
	if err != nil {
		fatal(logger, "load migration status before run", "error", err)
	}

	started := time.Now().UTC()
	if err := runner.Run(context.Background()); err != nil {
		fatal(logger, "migration run failed", "error", err)
	}

	after, err := runner.Status(context.Background())
	if err != nil {
		fatal(logger, "load migration status after run", "error", err)
	}

	appliedThisRun := before.PendingCount - after.PendingCount
	if appliedThisRun < 0 {
		appliedThisRun = 0
	}

	logger.Info(
		"migrations completed",
		"component", "migrate",
		"applied_this_run", appliedThisRun,
		"available", len(after.Available),
		"applied", after.AppliedCount,
		"pending", after.PendingCount,
		"latest", after.LatestVersion,
		"duration_ms", time.Since(started).Milliseconds(),
	)
}

func runStatus(logger *slog.Logger, runner *migrations.Runner) {
	status, err := runner.Status(context.Background())
	if err != nil {
		fatal(logger, "load migration status", "error", err)
	}

	fmt.Println(status.SummaryString())

	for _, pending := range status.Pending {
		fmt.Printf("PENDING version=%d name=%s\n", pending.Version, pending.Name)
	}
}

func runVerify(logger *slog.Logger, runner *migrations.Runner) {
	if err := runner.Verify(context.Background()); err != nil {
		fatal(logger, "migration verification failed", "error", err)
	}
	logger.Info("migrations verified", "component", "migrate")
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

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
