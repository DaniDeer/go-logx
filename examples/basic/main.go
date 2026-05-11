package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	errx "github.com/DaniDeer/go-logx/errx"
	logx "github.com/DaniDeer/go-logx/logx"
)

func main() {

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	logger, cleanup, err := logx.New(logx.Config{
		Level:       slog.LevelDebug,
		AddSource:   false,
		Console:     true,
		ConsoleJSON: false,
		File:        "app.log",
		FileLevel:   slog.LevelInfo,
	})

	// A valid use case to bypass the logger is ofc when the logger itself fails to initialize.
	// In that case, we print the error to stderr and exit with a non-zero status code.
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Instead of defer cleanup(), we call it explicitly and log any errors it returns.
	// This ensures that we handle cleanup errors properly.
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to clean up logger: %v\n", err)
			os.Exit(1)
		}
	}()

	run(ctx, cancel, logger)
	//status := run(ctx, cancel, logger)
	//cancel()
	//os.Exit(status)
}

// Use dependency injection to pass the logger to the run function, which simulates handling a request and logging an error with structured attributes.
func run(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger) int {

	err := errx.Wrap(
		errors.New("database timeout"),
		"purchase failed",
		"purchase_id", "p-123",
		"user_id", "u-42",
	)

	logger.Error("request failed",
		slog.Any("error", err),
	)

	if err != nil {
		logger.Debug("handling error",
			slog.Any("error", err),
		)
		return 1
	}

	return 0
}
