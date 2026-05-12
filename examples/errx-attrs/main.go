package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/DaniDeer/go-logx/attr"
	"github.com/DaniDeer/go-logx/errx"
	"github.com/DaniDeer/go-logx/logx"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger, cleanup, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		AddSource:   false,
		Console:     true,
		ConsoleJSON: false,
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
	// Simulate a layered error chain: repository → service → handler.
	// Each layer attaches structured attributes via errx.
	err := processOrder(ctx, "order-789")
	if err != nil {
		// Standard approach: log the full error including nested attrs and stack trace.
		// slog.Any("error", err) works because *errx.Error implements slog.LogValuer.
		logger.Error("order processing failed", slog.Any("error", err))

		// errx.Attrs() approach: manually extract all slog.Attr from the error chain.
		// Use this when you need the structured context outside of slog — for example
		// to build an alert payload, enrich a response, or forward to a monitoring system.
		reportError(logger, "order-processor", err)
	}
}

// processOrder simulates an order pipeline with a multi-layer error chain.
// Each layer wraps or annotates the error with domain-specific attributes.
func processOrder(_ context.Context, orderID string) error {
	// Repository layer: a low-level failure with infra context.
	dbErr := errx.New("payment gateway timeout",
		"gateway", "stripe",
		"timeout_ms", 3000,
	)

	// Service layer: wraps the repo error with business context.
	svcErr := errx.Wrap(dbErr, "charge failed",
		"order_id", orderID,
		"amount_cents", 4999,
		"currency", "EUR",
	)

	// Handler layer: attaches user context to an existing stdlib error chain.
	return errx.With(
		fmt.Errorf("order rejected: %w", svcErr),
		"user_id", "u-55",
		"region", "eu-west-1",
	)
}

// reportError demonstrates errx.Attrs() and the attr helpers.
// It extracts all structured attributes from the error chain and combines them
// with additional context to build a hypothetical alert payload.
//
// This pattern is useful when you need the attrs programmatically — for
// monitoring integrations, audit logs, or custom error-reporting pipelines —
// rather than only through the slog.LogValuer output.
func reportError(logger *slog.Logger, component string, err error) {
	if err == nil {
		return
	}

	// errx.Attrs walks the full error chain and merges all []slog.Attr.
	// Outermost error's attrs take precedence (first key wins on conflict).
	errAttrs := errx.Attrs(err)

	// attr.Args converts loose key/value pairs into []slog.Attr — the same
	// conversion used internally by errx.New, errx.Wrap, and errx.With.
	reportAttrs := attr.Args(
		"component", component,
		"alert_type", "error",
		"error_message", errors.Unwrap(err).Error(),
	)

	// attr.Merge combines multiple attr slices; first occurrence of a key wins.
	// Here report-level attrs take precedence over error-chain attrs.
	payload := attr.Merge(reportAttrs, errAttrs)

	// Log the reconstructed payload — in a real system you would send this
	// to an alerting endpoint, write to an audit log, or build a JSON body.
	logger.Warn("alert dispatched", attrsToArgs(payload)...)
}

// attrsToArgs converts []slog.Attr to []any for use as variadic logger args.
func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}
