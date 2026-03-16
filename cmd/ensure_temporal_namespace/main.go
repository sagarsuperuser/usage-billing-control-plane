package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	temporalclient "go.temporal.io/sdk/client"

	"usage-billing-control-plane/internal/logging"
	"usage-billing-control-plane/internal/temporalutil"
)

func main() {
	logger := logging.ConfigureDefault(logging.LoadConfigFromEnv())

	var address string
	var namespace string
	var timeout time.Duration
	var retention time.Duration

	flag.StringVar(&address, "address", "127.0.0.1:17233", "Temporal frontend host:port")
	flag.StringVar(&namespace, "namespace", "default", "Temporal namespace")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "overall timeout")
	flag.DurationVar(&retention, "retention", 24*time.Hour, "workflow retention period for created namespace")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, err := dialTemporalWithRetry(ctx, address, namespace)
	if err != nil {
		fatal(logger, err.Error())
	}
	defer client.Close()

	if err := temporalutil.EnsureNamespaceReady(ctx, client, namespace, retention); err != nil {
		fatal(logger, fmt.Sprintf("ensure namespace %q", namespace), "error", err)
	}

	fmt.Printf("temporal namespace %q is ready at %s\n", namespace, address)
}

func dialTemporalWithRetry(ctx context.Context, address, namespace string) (temporalclient.Client, error) {
	backoff := 200 * time.Millisecond
	var lastErr error

	for {
		if ctx.Err() != nil {
			if lastErr != nil {
				return nil, fmt.Errorf("dial temporal %s timed out: %w (last error: %v)", address, ctx.Err(), lastErr)
			}
			return nil, fmt.Errorf("dial temporal %s timed out: %w", address, ctx.Err())
		}

		client, err := temporalclient.Dial(temporalclient.Options{
			HostPort:  address,
			Namespace: namespace,
		})
		if err == nil {
			return client, nil
		}
		lastErr = err

		if err := sleepWithContext(ctx, backoff); err != nil {
			return nil, fmt.Errorf("dial temporal %s timed out: %w (last error: %v)", address, err, lastErr)
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func fatal(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
