// Package main demonstrates logx.WithLogger and logx.FromContext.
//
// Instead of threading a *slog.Logger through every function signature,
// store the enriched logger in the context once and retrieve it with
// logx.FromContext wherever logging is needed.
//
// Call chain: main → processRequest → queryDB
//   main          — initializes the logger, enriches it with service-level fields
//   processRequest — enriches further with request-scoped fields, stores in context
//   queryDB        — retrieves the logger from context, logs with all prior fields
//
// Each layer adds fields without knowing about the layers above or below it.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger, cleanup, err := logx.New(logx.Config{
		Level:   slog.LevelDebug,
		Console: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up logger: %v\n", err)
		}
	}()

	// Attach service-level fields to the logger before storing it in context.
	// Every log line produced anywhere in this process will carry these fields.
	logger = logger.With(
		slog.String("service", "order-api"),
		slog.String("version", "1.2.0"),
	)

	// Store the enriched logger in the root context.
	ctx = logx.WithLogger(ctx, logger)

	processRequest(ctx, "req-xyz", "POST", "/orders")
}

// processRequest simulates an HTTP handler.
// It enriches the logger with request-scoped fields and stores the result
// back into the context so that downstream functions inherit them automatically.
func processRequest(ctx context.Context, requestID, method, path string) {
	// logx.FromContext retrieves the logger stored by the caller.
	// If no logger was stored, it falls back to slog.Default().
	logger := logx.FromContext(ctx)

	// Enrich with request-scoped fields and push back into context.
	logger = logger.With(
		slog.String("request_id", requestID),
		slog.String("method", method),
		slog.String("path", path),
	)
	ctx = logx.WithLogger(ctx, logger)

	logger.Debug("handling request")

	if err := queryDB(ctx, "SELECT * FROM orders LIMIT 10"); err != nil {
		logger.Error("request failed", slog.Any("error", err))
		return
	}

	logger.Info("request completed")
}

// queryDB simulates a database call.
// It never receives a *slog.Logger argument — it retrieves one from the context,
// which already carries service-level and request-level fields added upstream.
func queryDB(ctx context.Context, query string) error {
	logger := logx.FromContext(ctx)

	logger.Debug("executing query", slog.String("query", query))

	// Simulate query execution.
	time.Sleep(30 * time.Millisecond)

	// Simulate a transient DB failure.
	return errx.New("deadlock detected",
		"query", query,
		"table", "orders",
	)
}
