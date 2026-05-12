// Package main demonstrates attr.Group for nested attribute grouping.
//
// attr.Group creates a named group of slog.Attr values that appear nested
// under a common key in the log output — for example "db.table", "db.query_ms",
// "request.method", "request.path". This keeps related fields organized and
// avoids key collisions across different subsystems.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DaniDeer/go-logx/attr"
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

	run(ctx, logger)
}

func run(ctx context.Context, logger *slog.Logger) {
	// attr.Group bundles related fields under a shared key prefix.
	// Here all request-scoped fields appear as request.* in the log output.
	requestGroup := attr.Group("request",
		slog.String("id", "req-abc123"),
		slog.String("method", "POST"),
		slog.String("path", "/orders"),
	)

	// Attach the request group to the logger so every subsequent log line
	// from this call carries request.id, request.method, request.path.
	logger = logger.With(requestGroup)

	logger.Info("handling request")

	order, err := processOrder(ctx, logger, "order-42")
	if err != nil {
		logger.Error("order processing failed", slog.Any("error", err))
		return
	}

	logger.Info("order processed", slog.String("order_id", order))
}

// processOrder simulates a DB-backed operation.
// It uses a separate attr.Group for DB-specific fields so they never collide
// with request-level or order-level attributes even if key names overlap.
func processOrder(ctx context.Context, logger *slog.Logger, orderID string) (string, error) {
	_ = ctx

	start := time.Now()

	// Simulate a DB insert — in real code this would be an actual query.
	time.Sleep(20 * time.Millisecond)

	elapsed := time.Since(start)

	dbErr := insertOrder(orderID)
	if dbErr != nil {
		return "", errx.Wrap(dbErr, "insert failed", "order_id", orderID)
	}

	// attr.Group for DB diagnostics: fields are nested under "db.*"
	// regardless of what other groups (e.g. "request.*") are on the logger.
	dbGroup := attr.Group("db",
		slog.String("table", "orders"),
		slog.String("op", "INSERT"),
		slog.Int64("query_ms", elapsed.Milliseconds()),
	)

	logger.Debug("db query completed", dbGroup)

	return orderID, nil
}

func insertOrder(_ string) error {
	// Simulate a successful insert.
	return nil
}
