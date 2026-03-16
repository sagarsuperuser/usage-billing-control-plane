package logging

import (
	"log"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Format string
	Level  slog.Level
}

func LoadConfigFromEnv() Config {
	return Config{
		Format: strings.TrimSpace(os.Getenv("LOG_FORMAT")),
		Level:  ParseLevel(strings.TrimSpace(os.Getenv("LOG_LEVEL"))),
	}
}

func ConfigureDefault(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Level}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(handler, slog.LevelInfo).Writer())

	return logger
}

func ParseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
